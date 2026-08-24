package controller

import (
	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type RegistrySyncStatus struct {
	controller.Abstract
	Lifecycle *logic.RegistrySyncStatusLifecycle
}

// List returns active and failed cache synchronization tasks for a host.
func (c RegistrySyncStatus) List(ctx *gin.Context) {
	host := ctx.Query("host")
	if host == "" {
		host = ctx.PostForm("host")
	}
	statuses := []logic.RegistrySyncStatus{}
	if c.Lifecycle != nil {
		statuses = c.Lifecycle.ListStatuses(host)
	}
	c.JsonResponseWithoutError(ctx, map[string]interface{}{"list": statuses})
}
