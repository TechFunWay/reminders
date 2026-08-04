package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

const (
	CodeSuccess            = 0
	CodeUnauthorized       = 401
	CodeForbidden          = 403
	CodeBadRequest         = 400
	CodeNotFound           = 404
	CodeInternalError      = 500
	CodeUserExists         = 1001
	CodeInvalidCredentials = 1002
	CodeRegisterDisabled   = 1003
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// SuccessPage wraps a paginated list in the standard envelope as
// { items, total, page, pageSize }. Use it for every list endpoint so clients
// can share one pagination helper.
func SuccessPage(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	Success(c, gin.H{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func Error(c *gin.Context, httpCode int, code int, message string) {
	c.JSON(httpCode, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

func ErrorUnauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

func ErrorForbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, CodeForbidden, message)
}

func ErrorBadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeBadRequest, message)
}

func ErrorInternal(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, CodeInternalError, message)
}
