package controller

import (
	"encoding/json"
	"net/url"
	"strings"

	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Setting struct {
	controller.Abstract
}

type commonRegistrySource struct {
	ServerURL string `json:"server_url"`
}

type commonPageSetting struct {
	Markdown      string `json:"markdown"`
	ICPNumber     string `json:"icp_number"`
	ICPLink       string `json:"icp_link"`
	PoliceNumber  string `json:"police_number"`
	PoliceLink    string `json:"police_link"`
	Copyright     string `json:"copyright"`
	CopyrightLink string `json:"copyright_link"`
}

type commonExtra struct {
	PageSetting *commonPageSetting `json:"page_setting,omitempty"`
}

type commonRegistryCacheSetting struct {
	RegistrySources []commonRegistrySource `json:"registry_sources,omitempty"`
	Extra           *commonExtra           `json:"extra,omitempty"`
}

func mergeRegistryCacheList() (map[string]*logic.RegistryCacheSetting, error) {
	list, err := logic.Setting{}.StorageCacheList()
	if err != nil {
		return nil, err
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
		mergedSetting := *val
		mergedSetting.Host = tmpKey
		mergeList[tmpKey] = &mergedSetting
	}
	return mergeList, nil
}

func (c Setting) List(ctx *gin.Context) {
	mergeList, err := mergeRegistryCacheList()
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		return
	}

	c.JsonResponseWithoutError(ctx, mergeList)
}

func (c Setting) CommonList(ctx *gin.Context) {
	list, err := mergeRegistryCacheList()
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		return
	}

	c.JsonResponseWithoutError(ctx, buildCommonRegistryCacheList(list))
}

func buildCommonRegistryCacheList(list map[string]*logic.RegistryCacheSetting) map[string]commonRegistryCacheSetting {
	// 公开接口只返回首页所需字段，避免缓存仓库凭据、源仓库凭据、代理配置、
	// 缓存规则以及后续新增的内部字段被意外暴露。
	commonList := make(map[string]commonRegistryCacheSetting)
	for group, setting := range list {
		if group == "global" {
			commonList[group] = commonRegistryCacheSetting{
				Extra: &commonExtra{
					PageSetting: commonPageSettingFromExtra(setting.Extra),
				},
			}
			continue
		}

		commonSetting := commonRegistryCacheSetting{}
		for _, source := range setting.RegistrySources {
			if serverURL := sanitizeCommonURL(source.ServerUrl); serverURL != "" {
				commonSetting.RegistrySources = append(commonSetting.RegistrySources, commonRegistrySource{
					ServerURL: serverURL,
				})
			}
		}
		commonList[group] = commonSetting
	}
	return commonList
}

func commonPageSettingFromExtra(extra map[string]interface{}) *commonPageSetting {
	value, exists := extra["page_setting"]
	if !exists {
		return nil
	}

	content, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	pageSetting := commonPageSetting{}
	if err = json.Unmarshal(content, &pageSetting); err != nil {
		return nil
	}
	return &pageSetting
}

func sanitizeCommonURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func (c Setting) Set(ctx *gin.Context) {
	type ParamsValidate struct {
		Host                 string                      `json:"group" binding:"required"`
		CacheStorageRegistry logic.CacheStorageRegistry  `json:"cache_storage_registry"  binding:"required"`
		RepositoryCacheRules []logic.RepositoryCacheRule `json:"repository_cache_rules"  binding:"required"`
		RegistrySources      []logic.RegistrySource      `json:"registry_sources"`
		OriginRegistry       logic.RegistrySource        `json:"origin_registry"`
		Extra                map[string]interface{}      `json:"extra"`
	}
	params := ParamsValidate{}
	if !c.Validate(ctx, &params) {
		return
	}

	host := strings.Split(params.Host, ",")
	parent := ""
	for i, item := range host {
		err := logic.Setting{}.SetStorageCacheSetting(item, logic.RegistryCacheSetting{
			Host:                 item,
			CacheRegistry:        params.CacheStorageRegistry,
			RepositoryCacheRules: params.RepositoryCacheRules,
			RegistrySources:      params.RegistrySources,
			OriginRegistry:       params.OriginRegistry,
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

	if setting != nil && host[0] != "global" && setting.CacheRegistry.CacheNamespacePrefix != "" {
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
