package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// 首屏快慢几乎由「要传多少字节」决定：前端入口包约 160KB、样式 66KB，弱网
// 下（手机端经飞牛网关远程访问尤其明显）未压缩传输就是几秒的等待。这里对
// 文本类响应统一 gzip，体积降到约五分之一。图片、字体、SQLite 上传文件等
// 已压缩内容按类型跳过，不做无用功。
//
// 只压缩「已知是文本」的响应：Content-Type 由处理函数设置，缺失时按内容嗅探。
var compressibleTypes = []string{
	"application/json",
	"application/javascript",
	"application/manifest+json",
	"application/xml",
	"image/svg+xml",
	"text/",
}

// 小于该长度的响应压缩后可能反而更大，直接原样发出。
const compressMinSize = 512

var gzipPool = sync.Pool{
	New: func() any {
		// 默认压缩级别：静态产物已被强缓存，同一份文件每次进入只压一次，省下
		// 的字节对弱网更有价值；低端 NAS 上这点 CPU 也远低于一次网络往返。
		writer, err := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		if err != nil {
			return nil
		}
		return writer
	},
}

func isCompressibleType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" {
		return false
	}
	if mediaType := strings.SplitN(contentType, ";", 2)[0]; mediaType != "" {
		contentType = mediaType
	}
	for _, prefix := range compressibleTypes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

// Compress 对文本响应做 gzip。判定延迟到真正写入响应头时进行，因此对
// http.ServeContent（静态文件）与 gin 的 JSON 渲染同样有效；Range 请求与
// HEAD 请求绝不压缩（前者压缩后 Content-Range 会与实际字节数不符，后者只有
// 响应头，压缩会破坏 Content-Length 语义）。
func Compress() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead || c.Request.Header.Get("Range") != "" || !acceptsGzip(c.Request) {
			c.Next()
			return
		}
		writer := &gzipResponseWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()
		writer.close()
	}
}

func acceptsGzip(r *http.Request) bool {
	for _, token := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, _, _ := strings.Cut(strings.TrimSpace(token), ";")
		if strings.EqualFold(name, "gzip") {
			return true
		}
	}
	return false
}

// gzipResponseWriter 在写入前决定是否压缩：一旦决定，就改写响应头并把后续
// 写入交给 gzip.Writer，由它转发到原始 writer。
type gzipResponseWriter struct {
	gin.ResponseWriter

	gz       *gzip.Writer
	decided  bool
	compress bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.decide(nil)
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	w.decide(data)
	if !w.compress {
		return w.ResponseWriter.Write(data)
	}
	return w.gz.Write(data)
}

func (w *gzipResponseWriter) WriteString(s string) (int, error) {
	w.decide([]byte(s))
	if !w.compress {
		return w.ResponseWriter.WriteString(s)
	}
	return w.gz.Write([]byte(s))
}

// Flush 让流式响应在压缩管线下依然能及时送达（压缩缓冲需要显式冲刷）。
func (w *gzipResponseWriter) Flush() {
	if w.compress && w.gz != nil {
		_ = w.gz.Flush()
	}
	w.ResponseWriter.Flush()
}

// decide 在写入前决定是否压缩。gin 的渲染是先写状态码、再写 Content-Type
// （c.Status → render.Render），所以「状态码已写但类型未知」不能当作最终结论，
// 否则所有 JSON 响应都会被判成不可压缩；这种情况把判定推迟到响应体写入时，
// 类型仍缺失则按内容嗅探。
func (w *gzipResponseWriter) decide(data []byte) {
	if w.decided {
		return
	}

	header := w.ResponseWriter.Header()
	if header.Get("Content-Encoding") != "" {
		w.decided = true
		return
	}

	contentType := header.Get("Content-Type")
	if contentType == "" {
		if len(data) == 0 {
			return
		}
		contentType = http.DetectContentType(data)
	}
	w.decided = true

	if !isCompressibleType(contentType) {
		return
	}

	size := len(data)
	if length := header.Get("Content-Length"); length != "" {
		if parsed, err := strconv.Atoi(length); err == nil {
			size = parsed
		}
	}
	if size < compressMinSize {
		return
	}

	writer, ok := gzipPool.Get().(*gzip.Writer)
	if !ok || writer == nil {
		return
	}
	writer.Reset(w.ResponseWriter)
	w.gz = writer
	w.compress = true
	header.Set("Content-Encoding", "gzip")
	header.Del("Content-Length")
	// 缓存层必须按编码区分，否则会把 gzip 内容发给不支持压缩的客户端。
	appendVary(header, "Accept-Encoding")
}

func (w *gzipResponseWriter) close() {
	if !w.compress || w.gz == nil {
		return
	}
	_ = w.gz.Close()
	w.gz.Reset(io.Discard)
	gzipPool.Put(w.gz)
	w.gz = nil
}

func appendVary(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for _, item := range strings.Split(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(item), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

// StaticCacheHeaders 为带内容哈希的构建产物与图标类静态文件加上长缓存。
// 前端资源名里有哈希，内容变化即换名，可以放心长期强缓存；没有这层缓存时
// 手机端每次进入都要把同一份 JS/CSS 重新下载一遍，是「打开很卡」的主因之一。
func StaticCacheHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case strings.Contains(path, "/assets/"):
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		case path == "/manifest.webmanifest" || hasStaticAssetExt(path):
			c.Header("Cache-Control", "public, max-age=604800")
		}
		c.Next()
	}
}

func hasStaticAssetExt(path string) bool {
	switch {
	case strings.HasSuffix(path, ".png"), strings.HasSuffix(path, ".ico"),
		strings.HasSuffix(path, ".svg"), strings.HasSuffix(path, ".webp"),
		strings.HasSuffix(path, ".woff2"):
		return true
	}
	return false
}
