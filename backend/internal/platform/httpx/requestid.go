package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

const RequestIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if len(id) == 0 || len(id) > 64 {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		c.Set("requestId", id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		id, _ := p.Keys["requestId"].(string)
		return fmt.Sprintf("time=%s request_id=%s method=%s path=%q status=%d latency_ms=%d ip=%s\n",
			p.TimeStamp.UTC().Format(time.RFC3339), id, p.Method, p.Path, p.StatusCode, p.Latency.Milliseconds(), p.ClientIP)
	})
}
