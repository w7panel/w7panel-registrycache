package controller

import (
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type AuthProxy struct {
	controller.Abstract
}

func (c AuthProxy) Handler(ctx *gin.Context) {
	host := ctx.Request.Host
	setting := logic.Setting{}.GetStorageCacheSetting(host)
	if setting == nil || setting.CacheRegistry.ServerUrl == "" {
		c.JsonResponseWithServerError(ctx, errors.New("缓存仓库配置错误"))
		return
	}

	upstreamRealm := ctx.Query("upstream")
	if upstreamRealm == "" {
		c.JsonResponseWithServerError(ctx, errors.New("upstream realm not found"))
		return
	}

	upstreamURL, err := url.Parse(upstreamRealm)
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		return
	}

	query := upstreamURL.Query()
	for key, values := range ctx.Request.URL.Query() {
		if key == "upstream" {
			continue
		}
		query.Del(key)
		for _, value := range values {
			if key == "scope" && setting.CacheRegistry.CacheNamespacePrefix != "" {
				value = rewriteRepositoryScope(value, setting.CacheRegistry.CacheNamespacePrefix)
			}
			query.Add(key, value)
		}
	}
	upstreamURL.RawQuery = query.Encode()

	slog.Info("auth proxy", "url", upstreamURL.String())

	req, err := http.NewRequestWithContext(ctx.Request.Context(), ctx.Request.Method, upstreamURL.String(), ctx.Request.Body)
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		return
	}
	req.Header = ctx.Request.Header.Clone()
	req.Host = upstreamURL.Host

	client := &http.Client{
		Transport: &http.Transport{
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
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("auth proxy request error", "url", upstreamURL.String(), "err", err)
		c.JsonResponseWithServerError(ctx, err)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			ctx.Writer.Header().Add(key, value)
		}
	}
	ctx.Status(resp.StatusCode)
	_, _ = io.Copy(ctx.Writer, resp.Body)
}
