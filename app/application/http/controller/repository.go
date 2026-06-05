package controller

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
	"github.com/docker/distribution"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Repository struct {
	controller.Abstract
}

func (c Repository) Handler(ctx *gin.Context) {
	host := ctx.Request.Host
	setting := logic.Setting{}.GetStorageCacheSetting(host)
	if setting == nil || setting.CacheRegistry.ServerUrl == "" {
		c.JsonResponseWithServerError(ctx, errors.New("缓存仓库配置错误"))
		return
	}
	if setting.RegistrySources == nil {
		c.JsonResponseWithServerError(ctx, errors.New("源仓库配置错误"))
		return
	}

	slog.Info("req path", "fullpath", ctx.Request.RequestURI)

	var params *ParamsValidate
	resourcesType := int8(0)
	if ctx.Request.Method == http.MethodGet || ctx.Request.Method == http.MethodHead {
		if matches := manifestRegex.FindStringSubmatch(ctx.Request.RequestURI); matches != nil {
			params = extractParams(manifestRegex, matches)
			resourcesType = logic.TransferTypeManifest
		} else if matches = blobRegex.FindStringSubmatch(ctx.Request.RequestURI); matches != nil {
			params = extractParams(blobRegex, matches)
			resourcesType = logic.TransferTypeBlob
		}
	}

	if resourcesType == 0 {
		//代理
		requestURI := ctx.Request.RequestURI
		if setting.CacheRegistry.CacheNamespacePrefix != "" {
			rewrittenURI := rewritePushRequestURI(ctx.Request.Method, ctx.Request.RequestURI, setting.CacheRegistry.CacheNamespacePrefix)
			if rewrittenURI != ctx.Request.RequestURI {
				requestURI = rewrittenURI
				slog.Info("proxy push route rewrite",
					"fullpath", ctx.Request.RequestURI,
					"proxyURI", requestURI,
				)
			}
		}

		proxyUrl := fmt.Sprintf("%s/%s", setting.CacheRegistry.ServerUrl, strings.TrimLeft(requestURI, "/"))
		remote, err := url.Parse(proxyUrl) // 替换为你的目标URL
		if err != nil {
			slog.Error("proxy url parse error", "url", proxyUrl, "err", err)
			c.JsonResponseWithServerError(ctx, err)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(remote)
		proxy.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   50,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			DisableCompression:    true,                                  // Docker镜像已压缩
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // 生产环境应使用有效证书
		}
		proxy.ModifyResponse = func(resp *http.Response) error {
			if location := resp.Header.Get("Location"); location != "" {
				resp.Header.Set("Location", rewritePushLocation(ctx, ctx.Request.RequestURI, requestURI, location))
			}
			slog.Info("proxy push response",
				"method", ctx.Request.Method,
				"fullpath", ctx.Request.RequestURI,
				"proxyURI", requestURI,
				"status", resp.StatusCode,
				"www_authenticate", resp.Header.Get("Www-Authenticate"),
				"location", resp.Header.Get("Location"),
				"docker_upload_uuid", resp.Header.Get("Docker-Upload-Uuid"),
			)

			if setting.CacheRegistry.CacheNamespacePrefix == "" || resp.StatusCode != http.StatusUnauthorized {
				return nil
			}
			authenticate := resp.Header.Get("Www-Authenticate")
			if authenticate == "" {
				return nil
			}
			token := rewriteAuthenticateRealm(ctx, authenticate)
			resp.Header.Set("Www-Authenticate", token)
			slog.Info("proxy reset toker", "fullpath", ctx.Request.RequestURI, "origin", authenticate, "token", token)
			return nil
		}
		proxy.Director = func(req *http.Request) {
			req.Header = ctx.Request.Header.Clone()
			req.Host = remote.Host
			// 设置正确的Host头
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = remote.Path
			req.URL.RawQuery = remote.RawQuery
		}
		proxy.ServeHTTP(ctx.Writer, ctx.Request)
		return
	}
	if params == nil {
		slog.Error("params not exists")
		c.JsonResponseWithServerError(ctx, errors.New("req url error"))
		return
	}

	if !c.Validate(ctx, params) {
		return
	}

	if resourcesType == logic.TransferTypeManifest {
		c.handlerManifest(ctx, *setting, *params)
		return
	} else if resourcesType == logic.TransferTypeBlob {
		c.handlerBlob(ctx, *setting, *params)
		return
	}

	ctx.Status(http.StatusOK)
}

func (c Repository) handlerV2(ctx *gin.Context) {
	ctx.Header("Docker-Distribution-API-Version", "registry/2.0")
	ctx.Status(http.StatusOK)
}

func (c Repository) handlerManifest(ctx *gin.Context, setting logic.RegistryCacheSetting, params ParamsValidate) {
	registryClient := logic.RegistryClient{}.GetRegistryClient(setting.Host, setting.CacheRegistry.ServerUrl, ctx.Request.Header)
	if registryClient == nil {
		slog.Error("handle manifest begin", "params", params, "err", "缓存仓库配置错误")
		c.JsonResponseWithServerError(ctx, errors.New("缓存仓库配置错误"))
		return
	}

	cacheRuleLogic := logic.CacheRule{}
	enableCache := false
	existsCache := false
	cacheTtl := int64(-1)
	var manifest *distribution.Descriptor
	var downloadFunc func(repository, reference string, accepttedMediaTypes ...string) (manifest distribution.Manifest, digest string, err error)
	sourceRegistryServerUrl := ""
	reqImageName := params.ImageName
	pullImageName := reqImageName

	repositoryCacheRule, err := cacheRuleLogic.MatchRepositoryCacheRule(reqImageName, setting.RepositoryCacheRules)
	slog.Info("manifest: match repositoryCacheRule", "params", params, "match_rule", repositoryCacheRule, "err", err)
	if repositoryCacheRule != nil && repositoryCacheRule.Enable {
		enableCache = repositoryCacheRule.Enable
		cacheTtl = repositoryCacheRule.CacheTtl
	}

	forceRecache := logic.Storage{}.RepositoryIsForceReCache(setting.Host, reqImageName)
	if enableCache && !forceRecache {
		params.ImageName = logic.Storage{}.RebuildImageName(pullImageName, setting.CacheRegistry.CacheNamespacePrefix)
		_, manifest, err = registryClient.ManifestExist(params.ImageName, params.ImageReference)
		slog.Info("manifest: StatObject with registry cache", "params", params, "err", err)
		if err != nil && strings.Contains(err.Error(), "UNAUTHORIZED") {
			ctx.JSON(http.StatusUnauthorized, gin.H{
				"errors": []gin.H{
					{
						"code":    "UNAUTHORIZED",
						"message": "authentication required",
					},
				},
			})
			return
		}
		if manifest != nil {
			existsCache = true
			downloadFunc = registryClient.PullManifest

			if cacheTtl > 0 {
				//检测过期时间
				modifyTime, _ := logic.Storage{}.GetRepositoryModifyTime(setting.Host, reqImageName, params.ImageReference)
				if modifyTime.IsZero() || time.Since(modifyTime).Minutes() > float64(cacheTtl) {
					existsCache = false
				}
			}

			if existsCache {
				pullImageName = params.ImageName
			}
		}
	}

	if !existsCache {
		//按照规则选择一个仓库地址
		sourceRegistryServerUrl = logic.RegistryServer{}.GetRegistrySourceFromRule(ctx, setting.Host, pullImageName, params.ImageReference, repositoryCacheRule, func(registryServerUrl string) bool {
			sourceRegistryClient := logic.RegistryClient{}.GetRegistryClient(setting.Host, registryServerUrl, nil)
			if sourceRegistryClient == nil {
				slog.Info("manifest: GetRegistryClient with registry source ", "params", params, "source", registryServerUrl, "err", errors.New("sourceRegistryClient init fail"))
				return false
			} else {
				_, smanifest, err1 := sourceRegistryClient.ManifestExist(pullImageName, params.ImageReference)
				slog.Info("manifest: StatObject with registry source", "url", registryServerUrl, "params", params, "exists", smanifest, "err", err1)
				if smanifest != nil {
					manifest = smanifest
					downloadFunc = sourceRegistryClient.PullManifest
					return true
				}
				return false
			}
		})
		if sourceRegistryServerUrl == "" {
			slog.Info("manifest: GetRegistrySourceFromRule with registry source", "params", params, "cacheRule", repositoryCacheRule, "err", errors.New("not found"))
		}
	}

	if manifest == nil || downloadFunc == nil {
		ctx.JSON(http.StatusNotFound,
			gin.H{
				"errors": []gin.H{
					{
						"code":    "MANIFEST_UNKNOWN",
						"message": "manifest unknown",
						"detail": map[string]string{
							"name":      reqImageName,
							"reference": params.ImageReference,
						},
					},
				},
			},
		)
		return
	}

	if ctx.Request.Method == http.MethodHead || forceRecache {
		//开启缓存，缓存不存在
		if enableCache && !existsCache && sourceRegistryServerUrl != "" {
			go logic.Transfer{}.Push(logic.TransferInfo{
				CacheSetting:            setting,
				SourceRegistryServerUrl: sourceRegistryServerUrl,
				RepositoryName:          reqImageName,
				Reference:               params.ImageReference,
			})
		}
		if forceRecache {
			logic.Storage{}.ResetRepositoryIsForceReCache(setting.Host, reqImageName)
		}
	}

	if etagMatch(ctx.Request, manifest.Digest.String()) {
		ctx.Status(http.StatusNotModified)
		return
	}

	ctx.Header("Docker-Content-Digest", manifest.Digest.String())
	ctx.Header("Content-Type", manifest.MediaType)
	ctx.Header("Content-length", fmt.Sprintf("%d", manifest.Size))
	ctx.Header("Etag", manifest.Digest.String())
	ctx.Header("Docker-Distribution-API-Version", "registry/2.0")

	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}

	manifestObj, err := logic.Storage{}.DownloadManifest(ctx, downloadFunc, pullImageName, params.ImageReference)
	if !existsCache && sourceRegistryServerUrl != "" {
		go logic.RegistryServer{}.RecordRepositoryReferenceRegistryWeight(setting.Host, reqImageName, params.ImageReference, sourceRegistryServerUrl, err == nil)
	}
	if err != nil {
		slog.Error("handle download manifest complete", "params", params, "err", err)
		if errors.Is(err, io.ErrUnexpectedEOF) {
			logic.Storage{}.RepositoryForceReCache(setting.Host, reqImageName)

			ctx.JSON(http.StatusServiceUnavailable,
				gin.H{
					"errors": []gin.H{
						{
							"code":    "UNAVAILABLE",
							"message": "service temporarily unavailable, please retry",
							"detail": map[string]interface{}{
								"retry_after_seconds": 5,
							},
						},
					},
				},
			)
			return
		}

		c.JsonResponseWithServerError(ctx, err)
		return
	}

	mediaType, payload, err := manifestObj.Payload()
	if err != nil {
		slog.Error("handle manifest complete", "params", params, "err", err)
		c.JsonResponseWithServerError(ctx, err)
		return
	}

	ctx.Data(http.StatusOK, mediaType, payload)
}

func (c Repository) handlerBlob(ctx *gin.Context, setting logic.RegistryCacheSetting, params ParamsValidate) {
	registryClient := logic.RegistryClient{}.GetRegistryClient(setting.Host, setting.CacheRegistry.ServerUrl, ctx.Request.Header)
	if registryClient == nil {
		slog.Error("handle blob begin", "params", params, "err", "缓存仓库配置错误")
		c.JsonResponseWithServerError(ctx, errors.New("缓存仓库配置错误"))
		return
	}

	cacheRuleLogic := logic.CacheRule{}
	enableCache := false
	existsCache := false
	blobSize := int64(0)
	blobDigest := ""
	var downloadFunc func(repository, digest string, blobSize, start, end int64) (size int64, blob io.ReadCloser, err error)
	sourceRegistryServerUrl := ""
	reqImageName := params.ImageName
	pullImageName := reqImageName

	repositoryCacheRule, err := cacheRuleLogic.MatchRepositoryCacheRule(reqImageName, setting.RepositoryCacheRules)
	slog.Info("blob: match repositoryCacheRule", "match_rule", repositoryCacheRule, "params", params, "err", err)
	if repositoryCacheRule != nil && repositoryCacheRule.Enable {
		enableCache = repositoryCacheRule.Enable
	}

	exists := false
	forceRecache := logic.Storage{}.RepositoryIsForceReCache(setting.Host, reqImageName)
	if enableCache && !forceRecache {
		params.ImageName = logic.Storage{}.RebuildImageName(pullImageName, setting.CacheRegistry.CacheNamespacePrefix)
		exists, blobSize, blobDigest, err = registryClient.BlobExist(params.ImageName, params.ImageReference)
		slog.Info("blob: StatObject with registry cache", "params", params, "err", err, "blobDigest", blobDigest, "exists", exists, "blobSize", blobSize)
		if err != nil && strings.Contains(err.Error(), "UNAUTHORIZED") {
			ctx.JSON(http.StatusUnauthorized, gin.H{
				"errors": []gin.H{
					{
						"code":    "UNAUTHORIZED",
						"message": "authentication required",
					},
				},
			})
			return
		}
		if exists {
			existsCache = true
			downloadFunc = registryClient.PullBlobChunk
			pullImageName = params.ImageName
		}
	}

	if !existsCache {
		sourceRegistryServerUrl = logic.RegistryServer{}.GetRegistrySourceFromRule(ctx, setting.Host, pullImageName, params.ImageReference, repositoryCacheRule, func(registryServerUrl string) bool {
			sourceRegistryClient := logic.RegistryClient{}.GetRegistryClient(setting.Host, registryServerUrl, nil)
			if sourceRegistryClient == nil {
				slog.Info("blob: GetRegistryClient with registry source", "params", params, "source", registryServerUrl, "err", errors.New("sourceRegistryClient init fail"))
				return false
			} else {
				sexists, sblobSize, sblobDigest, err1 := sourceRegistryClient.BlobExist(pullImageName, params.ImageReference)
				slog.Info("manifest: StatObject with registry source", "url", registryServerUrl, "params", params, "exists", sexists, "err", err1)

				if sexists {
					blobSize = sblobSize
					blobDigest = sblobDigest
					downloadFunc = sourceRegistryClient.PullBlobChunk
					return true
				}
				return false
			}
		})
		if sourceRegistryServerUrl == "" {
			slog.Info("blob: GetRegistrySourceFromRule with registry source", "params", params, "cacheRule", repositoryCacheRule, "err", errors.New("not found"))
		}
	}

	if downloadFunc == nil {
		ctx.JSON(http.StatusNotFound,
			gin.H{
				"errors": []gin.H{
					{
						"code":    "BLOB_UNKNOWN",
						"message": "blob unknown to registry",
					},
				},
			},
		)
		return
	}

	if etagMatch(ctx.Request, blobDigest) {
		ctx.Status(http.StatusNotModified)
		return
	}

	ctx.Header("Docker-Content-Digest", blobDigest)
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Length", fmt.Sprintf("%d", blobSize))
	ctx.Header("Etag", blobDigest)
	ctx.Header("Docker-Distribution-API-Version", "registry/2.0")

	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}

	err = logic.Storage{}.DownloadBlob(ctx, downloadFunc, pullImageName, params.ImageReference, blobSize, ctx.Writer, true)
	if !existsCache && sourceRegistryServerUrl != "" {
		go logic.RegistryServer{}.RecordRepositoryReferenceRegistryWeight(setting.Host, reqImageName, params.ImageReference, sourceRegistryServerUrl, err == nil)
	}
	if err != nil {
		slog.Error("handle blob complete", "params", params, "err", err)
		if errors.Is(err, io.ErrUnexpectedEOF) {
			logic.Storage{}.RepositoryForceReCache(setting.Host, reqImageName)

			ctx.JSON(http.StatusServiceUnavailable,
				gin.H{
					"errors": []gin.H{
						{
							"code":    "UNAVAILABLE",
							"message": "service temporarily unavailable, please retry",
							"detail": map[string]interface{}{
								"retry_after_seconds": 5,
							},
						},
					},
				},
			)
			return
		}

		c.JsonResponseWithServerError(ctx, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (c Repository) rebuildImageName(imageName string, namespaceSuffix string) string {
	list := strings.Split(strings.Trim(imageName, "/"), "/")
	if len(list) == 1 {
		list = append([]string{"default"}, list...)
	}
	list[0] = list[0] + "_" + namespaceSuffix
	return strings.Join(list, "/")
}
