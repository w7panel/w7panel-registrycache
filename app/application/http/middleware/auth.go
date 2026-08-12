package middleware

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

type Auth struct {
	middleware.Abstract
}

func (c Auth) checkToken(ctx *gin.Context) error {
	remoteToken := strings.ToLower(ctx.Request.Header.Get("Authorization"))
	localToken := strings.ToLower(facade.GetConfig().GetString("setting.oauth_token"))

	slog.Info("token info", "remoteToken", remoteToken, "localToken", localToken)
	if remoteToken == localToken {
		return nil
	}

	header := strings.Split(remoteToken, "bearer ")
	if len(header) <= 1 {
		return errors.New("请先登录")
	}
	token := header[1]

	if token == "" || token != localToken {
		return errors.New("非法请求")
	}

	return nil
}

func (c Auth) Process(ctx *gin.Context) {
	err := c.checkToken(ctx)
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		ctx.Abort()
		return
	}

	ctx.Next()
}
