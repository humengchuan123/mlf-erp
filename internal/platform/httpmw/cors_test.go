package httpmw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowsIdempotencyHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS([]string{"http://example.com"}))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodOptions, "/test", nil)
	request.Header.Set("Origin", "http://example.com")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Idempotency-Key") {
		t.Fatalf("allow headers = %q, want Idempotency-Key", got)
	}
	if got := response.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, "Idempotency-Replayed") {
		t.Fatalf("expose headers = %q, want Idempotency-Replayed", got)
	}
}
