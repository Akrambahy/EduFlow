package httpx

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("url_or_empty", urlOrEmpty)
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			if name == "" {
				return f.Name
			}
			return name
		})
	}
}

func BindJSON(c *gin.Context, dst any) bool {
	err := c.ShouldBindJSON(dst)
	if err == nil {
		return true
	}
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		details := make(map[string]string, len(verrs))
		for _, fe := range verrs {
			details[fe.Field()] = validationMessage(fe)
		}
		Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", details)
		return false
	}
	if errors.Is(err, io.EOF) {
		Error(c, http.StatusBadRequest, "invalid_body", "Request body is required", nil)
		return false
	}
	Error(c, http.StatusBadRequest, "invalid_body", "Request body must be valid JSON", nil)
	return false
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "min":
		return "must be at least " + fe.Param()
	case "max":
		return "must be at most " + fe.Param()
	case "oneof":
		return "must be one of: " + fe.Param()
	case "uuid", "uuid4":
		return "must be a valid id"
	case "url", "http_url", "url_or_empty":
		return "must be a valid URL"
	default:
		return "is invalid"
	}
}

func urlOrEmpty(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
