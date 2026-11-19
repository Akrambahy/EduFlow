package httpx

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}

func Error(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Message: message,
		Code:    code,
		Details: details,
	})
}

func Internal(c *gin.Context, err error) {
	log.Printf("level=error request_id=%s method=%s route=%s error=%q", c.GetString("requestId"), c.Request.Method, c.FullPath(), err.Error())
	Error(c, http.StatusInternalServerError, "internal_error", "Something went wrong", nil)
}

func Data(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
