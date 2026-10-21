package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const MaxRequestBodyBytes int64 = 1 << 20

func BodyLimit() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, MaxRequestBodyBytes)
		ctx.Next()
	}
}
