package main

import (
	"bytes"
	"embed"
	_ "embed"
	"io/fs"
	nethttp "net/http"

	app2 "gitee.com/we7coreteam/w7-registry-cache/app/application"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	app "github.com/we7coreteam/w7-rangine-go/v2/src"
	"github.com/we7coreteam/w7-rangine-go/v2/src/core/helper"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

//go:embed config.yaml
var ConfigFileContent []byte

//go:embed all:public
var Asset embed.FS

func main() {
	newApp := app.NewApp(app.Option{
		DefaultConfigLoader: func(config *viper.Viper) {
			config.SetConfigType("yaml")
			err := config.MergeConfig(bytes.NewReader(helper.ParseConfigContentEnv(ConfigFileContent)))
			if err != nil {
				panic(err)
			}
		},
	})

	httpServer := new(http.Provider).Register(newApp.GetConfig(), newApp.GetConsole(), newApp.GetServerManager()).Export()
	httpServer.Use(middleware.GetPanicHandlerMiddleware())

	httpServer.RegisterRouters(
		func(engine *gin.Engine) {
			staticFp, _ := fs.Sub(Asset, "public")
			engine.NoRoute(gin.WrapH(nethttp.FileServer(nethttp.FS(staticFp))))
		},
	)

	new(app2.Provider).Register(httpServer)

	newApp.RunConsole()
}
