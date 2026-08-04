package upload

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"smallgo/server/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MaxUploadSize is the largest accepted upload (bytes). Adjust as needed.
const MaxUploadSize = 20 << 20 // 20 MiB
const maxUploadRequestSize = MaxUploadSize + (1 << 20)

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

func HandleUpload(uploadDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadRequestSize)
		file, err := c.FormFile("file")
		if err != nil {
			response.ErrorBadRequest(c, "请上传文件")
			return
		}

		if file.Size > MaxUploadSize {
			response.ErrorBadRequest(c, fmt.Sprintf("文件过大（最大 %d MB）", MaxUploadSize>>20))
			return
		}
		if file.Size <= 0 {
			response.ErrorBadRequest(c, "文件不能为空")
			return
		}

		src, err := file.Open()
		if err != nil {
			response.ErrorBadRequest(c, "读取文件失败")
			return
		}
		header := make([]byte, 512)
		n, readErr := src.Read(header)
		src.Close()
		if readErr != nil && n == 0 {
			response.ErrorBadRequest(c, "读取文件失败")
			return
		}
		contentType := http.DetectContentType(header[:n])
		ext, allowed := allowedImageTypes[contentType]
		if !allowed {
			response.ErrorBadRequest(c, "仅支持 JPEG、PNG、GIF 或 WebP 图片")
			return
		}

		now := time.Now()
		datePath := filepath.Join(
			fmt.Sprintf("%04d", now.Year()),
			fmt.Sprintf("%02d", now.Month()),
			fmt.Sprintf("%02d", now.Day()),
		)

		dir := filepath.Join(uploadDir, datePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			response.ErrorInternal(c, "创建目录失败")
			return
		}

		// The extension is derived from detected content, never from user input.
		filename := uuid.New().String() + ext
		savePath := filepath.Join(dir, filename)

		if err := c.SaveUploadedFile(file, savePath); err != nil {
			response.ErrorInternal(c, "保存文件失败")
			return
		}

		relativePath := "/uploads/" + strings.ReplaceAll(filepath.Join(datePath, filename), "\\", "/")
		response.Success(c, gin.H{"path": relativePath})
	}
}

func ServeUpload(uploadDir string) gin.HandlerFunc {
	root, err := filepath.Abs(uploadDir)
	if err != nil {
		root = uploadDir
	}
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
		// Clean the request path against "/" first so any ".." segments are
		// collapsed before joining — this prevents path traversal out of root.
		rel := filepath.Clean("/" + c.Param("filepath"))
		fullPath := filepath.Join(root, rel)

		// Defense in depth: ensure the resolved path stays inside the root.
		if fullPath != root && !strings.HasPrefix(fullPath, root+string(os.PathSeparator)) {
			c.Status(http.StatusNotFound)
			return
		}

		c.File(fullPath)
	}
}
