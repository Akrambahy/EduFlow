package httpx

import (
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

func Data(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
