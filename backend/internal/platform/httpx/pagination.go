package httpx

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

type Page struct {
	Number int
	Limit  int
	Offset int
}

func ParsePage(c *gin.Context) Page {
	number := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 20)
	if number < 1 {
		number = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return Page{Number: number, Limit: limit, Offset: (number - 1) * limit}
}

func SetTotal(c *gin.Context, total int) {
	c.Header("X-Total-Count", strconv.Itoa(total))
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return value
}
