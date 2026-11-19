package httpx

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return false
			}
		}
	}
	return true
}

func PathUUID(c *gin.Context, name string) (string, bool) {
	v := c.Param(name)
	if !IsUUID(v) {
		NotFound(c)
		return "", false
	}
	return v, true
}

func QueryUUID(c *gin.Context, name string) (string, bool) {
	v := c.Query(name)
	if v != "" && !IsUUID(v) {
		Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{name: "must be a valid id"})
		return "", false
	}
	return v, true
}

func QueryBool(c *gin.Context, name string) (*bool, bool) {
	v := c.Query(name)
	if v == "" {
		return nil, true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{name: "must be true or false"})
		return nil, false
	}
	return &b, true
}

func NotFound(c *gin.Context) {
	Error(c, http.StatusNotFound, "not_found", "Resource not found", nil)
}
