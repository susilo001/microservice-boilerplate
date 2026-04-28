package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apiresp "github.com/boilerplate/microservice-boilerplate/api/http/v1/response"
)

// ErrorHandler catches errors attached via c.Error() and renders a standard APIResponse.
// statusFor maps a domain error to the appropriate HTTP status code; it should return
// http.StatusInternalServerError for any error it does not recognise.
func ErrorHandler(statusFor func(error) int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err
		correlationID := c.GetString("correlationid")
		tin := apiresp.ContextTime(c, "tin")
		tout := time.Now()

		status := http.StatusInternalServerError
		if statusFor != nil {
			status = statusFor(err)
		}
		c.JSON(status, apiresp.NewError(correlationID, tin, tout, err.Error()))
	}
}
