package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
)

const (
	CodeOK                 = "OK"
	CodeBadRequest         = "BAD_REQUEST"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeNotFound           = "NOT_FOUND"
	CodeInternal           = "INTERNAL"
	CodeCapabilityRequired = "CAPABILITY_REQUIRED"
	CodeLimitExceeded      = "LIMIT_EXCEEDED"
	CodeLicenseExpired     = "LICENSE_EXPIRED"
)

type Response struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, newResponse(c, CodeOK, "ok", data))
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, newResponse(c, CodeOK, "created", data))
}

func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, newResponse(c, code, message, nil))
}

func newResponse(c *gin.Context, code, message string, data any) Response {
	return Response{
		Code:      code,
		Message:   message,
		Data:      data,
		RequestID: c.GetString(httpmw.RequestIDKey),
	}
}
