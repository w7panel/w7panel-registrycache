package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

type Cors struct {
	middleware.Abstract
}

func (self Cors) Process(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Headers", self.getAllowHeader())
	ctx.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	ctx.Header("Access-Control-Expose-Headers", self.getAllowHeader())
	ctx.Header("Access-Control-Allow-Credentials", "false")
	if ctx.Request.Method == "OPTIONS" {
		ctx.AbortWithStatus(http.StatusNoContent)
	}
	ctx.Next()
}

func (self Cors) getAllowHeader() string {
	allowHeader := []string{
		"Content-Length",
		"Content-Type",
		"X-Auth-Token",
		"Origin",
		"Authorization",
		"X-Requested-With",
		"x-requested-with",
		"x-xsrf-token",
		"x-csrf-token",
		"x-w7-from",
		"access-token",
		"Api-Version",
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Headers",
		"Access-Control-Allow-Methods",
		"authority",
		"uid",
		"uuid",
		"X-W7Panel-Token",
	}
	return strings.Join(allowHeader, ",")
}
