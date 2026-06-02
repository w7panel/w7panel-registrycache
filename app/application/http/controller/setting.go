package controller

import (
	"strings"

	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Setting struct {
	controller.Abstract
}

func (c Setting) List(ctx *gin.Context) {
	list, err := logic.Setting{}.StorageCacheList()
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		return
	}

	mergeList := make(map[string]*logic.RegistryCacheSetting)
	for key, val := range list {
		if val.Parent != "" {
			continue
		}
		tmpKey := key
		for key1, val1 := range list {
			if val1.Parent != "" && val1.Parent == key {
				tmpKey += "," + key1
			}
		}
		val.Host = tmpKey
		mergeList[tmpKey] = val
	}

	c.JsonResponseWithoutError(ctx, mergeList)
}

func (c Setting) Set(ctx *gin.Context) {
	type ParamsValidate struct {
		Host                 string                      `json:"group" binding:"required"`
		CacheStorageRegistry logic.CacheStorageRegistry  `json:"cache_storage_registry"  binding:"required"`
		RepositoryCacheRules []logic.RepositoryCacheRule `json:"repository_cache_rules"  binding:"required"`
		RegistrySources      []logic.RegistrySource      `json:"registry_sources"  binding:"required"`
		Extra                map[string]interface{}      `json:"extra"`
	}
	params := ParamsValidate{}
	if !c.Validate(ctx, &params) {
		return
	}
	params.CacheStorageRegistry.CacheNamespacePrefix = ""

	host := strings.Split(params.Host, ",")
	parent := ""
	for i, item := range host {
		err := logic.Setting{}.SetStorageCacheSetting(item, logic.RegistryCacheSetting{
			Host:                 item,
			CacheRegistry:        params.CacheStorageRegistry,
			RepositoryCacheRules: params.RepositoryCacheRules,
			RegistrySources:      params.RegistrySources,
			Extra:                params.Extra,
			Parent:               parent,
		})
		if err != nil {
			c.JsonResponseWithServerError(ctx, err)
			return
		}
		if i == 0 {
			parent = item
		}
	}

	c.JsonSuccessResponse(ctx)
}

func (c Setting) Get(ctx *gin.Context) {
	type ParamsValidate struct {
		Host string `json:"group" form:"group" binding:"required"`
	}
	params := ParamsValidate{}
	if !c.Validate(ctx, &params) {
		return
	}

	host := strings.Split(params.Host, ",")

	setting := logic.Setting{}.GetStorageCacheSetting(host[0])

	if setting != nil && setting.CacheRegistry.CacheNamespacePrefix != "" {
		tmpSetting := *setting
		tmpSetting.CacheRegistry.ServerUrl = tmpSetting.CacheRegistry.ServerUrl + "/" + tmpSetting.CacheRegistry.CacheNamespacePrefix
		tmpSetting.Host = params.Host

		c.JsonResponseWithoutError(ctx, tmpSetting)
		return
	}

	c.JsonResponseWithoutError(ctx, setting)
}

func (c Setting) Del(ctx *gin.Context) {
	type ParamsValidate struct {
		Host string `json:"group" form:"group" binding:"required"`
	}
	params := ParamsValidate{}
	if !c.Validate(ctx, &params) {
		return
	}

	host := strings.Split(params.Host, ",")
	for _, item := range host {
		logic.Setting{}.DelStorageCacheSetting(item)
	}

	c.JsonSuccessResponse(ctx)
}
