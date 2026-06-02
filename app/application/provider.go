package app

import (
	"net/http"

	"gitee.com/we7coreteam/w7-registry-cache/app/application/http/controller"
	"gitee.com/we7coreteam/w7-registry-cache/app/application/http/middleware"
	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
	"github.com/gin-gonic/gin"
	httpServer "github.com/we7coreteam/w7-rangine-go/v2/src/http/server"
)

type Provider struct {
}

func (p Provider) Register(httpServer *httpServer.Server) {
	p.RegisterHttpRoutes(httpServer)

	logic.Transfer{}.Loop()
}

func (p Provider) RegisterHttpRoutes(server *httpServer.Server) {
	server.RegisterRouters(func(engine *gin.Engine) {
		engine.Any("/health", func(context *gin.Context) {
			context.Status(http.StatusOK)
		})

		engine.Any("/api/setting/set", middleware.Cors{}.Process, middleware.Auth{}.Process, controller.Setting{}.Set)
		engine.Any("/api/setting/get", middleware.Cors{}.Process, middleware.Auth{}.Process, controller.Setting{}.Get)
		engine.Any("/api/setting/list", middleware.Cors{}.Process, middleware.Auth{}.Process, controller.Setting{}.List)
		engine.Any("/api/setting/del", middleware.Cors{}.Process, middleware.Auth{}.Process, controller.Setting{}.Del)
		engine.Any("/api/k8s/proxy/*path", middleware.Cors{}.Process, middleware.Auth{}.Process, controller.K8s{}.Proxy)

		engine.Any("/oauth/auth-proxy", controller.AuthProxy{}.Handler)
		engine.Any("/v2/*path", controller.Repository{}.Handler)
	})
}
