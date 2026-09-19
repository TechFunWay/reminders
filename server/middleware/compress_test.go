package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newCompressRouter(handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Compress())
	r.GET("/probe", handler)
	return r
}

func probe(r *gin.Engine, acceptEncoding string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeGzip(t *testing.T, body []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	return decoded
}

// 首屏的入口包与样式都是纯文本，弱网（手机端经飞牛网关）下体积就是等待时间：
// 这个测试锁住「文本响应确实被压缩、且解压后内容完全一致」。
func TestCompressGzipsJSONBody(t *testing.T) {
	payload := map[string]string{"site_title": strings.Repeat("提醒事项", 200)}
	r := newCompressRouter(func(c *gin.Context) { c.JSON(http.StatusOK, payload) })

	w := probe(r, "gzip, deflate, br", nil)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", w.Header().Get("Content-Encoding"))
	}
	if w.Header().Get("Content-Length") != "" {
		t.Fatalf("compressed response must not keep the original Content-Length")
	}
	if !strings.Contains(w.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Accept-Encoding so caches keep both variants", w.Header().Get("Vary"))
	}
	var decoded map[string]string
	if err := json.Unmarshal(decodeGzip(t, w.Body.Bytes()), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["site_title"] != payload["site_title"] {
		t.Fatalf("body mismatch after gzip round trip")
	}
}

func TestCompressSkipsWhenClientDoesNotAcceptGzip(t *testing.T) {
	r := newCompressRouter(func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"a": strings.Repeat("x", 600)}) })
	w := probe(r, "", nil)
	if w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("unexpected Content-Encoding %q", w.Header().Get("Content-Encoding"))
	}
	if !strings.Contains(w.Body.String(), "xxxx") {
		t.Fatalf("body should be plain JSON, got %q", w.Body.String())
	}
}

// 图片、字体等已压缩内容再压缩毫无收益，只会白烧 NAS 的 CPU。
func TestCompressLeavesBinaryUntouched(t *testing.T) {
	raw := bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47}, 200)
	r := newCompressRouter(func(c *gin.Context) {
		c.Data(http.StatusOK, "image/png", raw)
	})
	w := probe(r, "gzip", nil)
	if w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("png must not be compressed, got %q", w.Header().Get("Content-Encoding"))
	}
	if !bytes.Equal(w.Body.Bytes(), raw) {
		t.Fatalf("png body changed")
	}
}

func TestCompressSkipsTinyBodies(t *testing.T) {
	r := newCompressRouter(func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"code": 0}) })
	w := probe(r, "gzip", nil)
	if w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("tiny body should stay uncompressed")
	}
}

// Range 请求压缩后 Content-Range 会与实际字节数不符；HEAD 只有响应头，
// 压缩会破坏 Content-Length 语义。两者都必须原样返回。
func TestCompressSkipsRangeAndHead(t *testing.T) {
	body := []byte(strings.Repeat("0123456789", 400))
	r := newCompressRouter(func(c *gin.Context) { c.Data(http.StatusOK, "text/plain", body) })

	ranged := probe(r, "gzip", map[string]string{"Range": "bytes=0-99"})
	if ranged.Header().Get("Content-Encoding") != "" {
		t.Fatalf("range request must not be compressed")
	}

	gin.SetMode(gin.TestMode)
	head := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/probe", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	r.ServeHTTP(head, req)
	if head.Header().Get("Content-Encoding") != "" {
		t.Fatalf("HEAD response must not be compressed")
	}
}

// Vite 产物名带内容哈希，可以长期强缓存；SPA 入口则必须不缓存（升级后立刻
// 换新版前端），两者由同一个中间件按路径区分。
func TestStaticCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(StaticCacheHeaders())
	r.GET("/assets/index-abc123.js", func(c *gin.Context) { c.String(http.StatusOK, "js") })
	r.GET("/favicon-32.png", func(c *gin.Context) { c.String(http.StatusOK, "png") })
	r.GET("/api/version", func(c *gin.Context) { c.String(http.StatusOK, "{}") })

	w := probePath(r, "/assets/index-abc123.js")
	if !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("hashed asset Cache-Control = %q, want immutable", w.Header().Get("Cache-Control"))
	}
	w = probePath(r, "/favicon-32.png")
	if !strings.Contains(w.Header().Get("Cache-Control"), "max-age") {
		t.Fatalf("icon Cache-Control = %q, want max-age", w.Header().Get("Cache-Control"))
	}
	w = probePath(r, "/api/version")
	if w.Header().Get("Cache-Control") != "" {
		t.Fatalf("api response should carry no cache header, got %q", w.Header().Get("Cache-Control"))
	}
}

func probePath(r *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
