package controller

import (
	"net/http/httputil"
	"net/url"

	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"k8s.io/client-go/rest"
)

type K8s struct {
	controller.Abstract
}

func (c K8s) Proxy(ctx *gin.Context) {
	type ParamsValidate struct {
		Path string `uri:"path" binding:"required"`
	}
	params := ParamsValidate{}
	if !c.Validate(ctx, &params) {
		return
	}

	k8sConfig := ""
	if facade.GetConfig().GetString("app.env") == "debug" {
		k8sConfig = "apiVersion: v1\nclusters:\n- cluster:\n    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRJMU5qUTRPVE13SGhjTk1qVXdOekUxTURjek5EVXpXaGNOTXpVd056RXpNRGN6TkRVegpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRJMU5qUTRPVE13V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUYTd6Z2EzU1dJbXU2OXZtUWdhQmpjNDdManhPL3d0NUc3dE9wSThSYlYKWnYySmRpTXlrRSs5RVRZSGVnc0RMS3YxZWY5QS83UlNnMmRPSmlwTWFSdGhvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXpFUGNOOFdGNlBEZ3l4L3hnaVc1Cmo3c25GVzh3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnZUlYYXliQkM1UGk0L0JIcTdFTFpUZXZZN0tudFVIQWEKZXVvK2RtOCsvOFFDSVFEcFU1K2psUllqbjdZL2lxZW4vUGNXMEdpTThwQUJQMGRRbzZvUVRtVzB2QT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K\n    server: https://218.23.2.55:6443\n  name: default\ncontexts:\n- context:\n    cluster: default\n    user: default\n  name: default\ncurrent-context: default\nkind: Config\npreferences: {}\nusers:\n- name: default\n  user:\n    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJrVENDQVRlZ0F3SUJBZ0lJZG1PMlZtUnJUN3N3Q2dZSUtvWkl6ajBFQXdJd0l6RWhNQjhHQTFVRUF3d1kKYXpOekxXTnNhV1Z1ZEMxallVQXhOelV5TlRZME9Ea3pNQjRYRFRJMU1EY3hOVEEzTXpRMU0xb1hEVEkyTURjeApOVEEzTXpRMU0xb3dNREVYTUJVR0ExVUVDaE1PYzNsemRHVnRPbTFoYzNSbGNuTXhGVEFUQmdOVkJBTVRESE41CmMzUmxiVHBoWkcxcGJqQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJHM1loS2tKTS9waGV0UkEKbE4wTXlnRmg4b0dPT3ZhR2ZTVUU2ZVhTOHh0Sjk1dCtvUHc5SmRoLzhLMlJqSDYwcTZ1Wm0wblZDcHZiVXlibAo4UXcvSGtDalNEQkdNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUtCZ2dyQmdFRkJRY0RBakFmCkJnTlZIU01FR0RBV2dCUm9NUXMxTmJRcGErS0xsVW1OZkxlcHRQWXNsREFLQmdncWhrak9QUVFEQWdOSUFEQkYKQWlFQW81Smc1ais3S3Q1dXBuVGdISUR5RU45NG1XWStKazNyTUR3MkhhVXppeUFDSUE0bTA2VVJneGxYbEFJLwpTWmFPcFJIdHpFSU9pam15UVRHYWszOXc3bWhsCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJlRENDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdFkyeHAKWlc1MExXTmhRREUzTlRJMU5qUTRPVE13SGhjTk1qVXdOekUxTURjek5EVXpXaGNOTXpVd056RXpNRGN6TkRVegpXakFqTVNFd0h3WURWUVFEREJock0zTXRZMnhwWlc1MExXTmhRREUzTlRJMU5qUTRPVE13V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFSV2F4YVJ6a3FISHErRFRhbVlvVEZwYkI0Sm5NMjh1MFhGUFhQelQ4N3UKL2hjaVBCRkN3WGpCYmRBRExMa3JSR0tqSFpyQmQ0cCt1YVh3V0N2T0RuSHRvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWFERUxOVFcwS1d2aWk1VkpqWHkzCnFiVDJMSlF3Q2dZSUtvWkl6ajBFQXdJRFNRQXdSZ0loQUxNTDQ2UmpDRS9xdGJYUm1pazUvR0JETHd5eUQxS2cKekVpeng1cVNWZUhEQWlFQXVFZk45QStoaStsaWJoR2cxcjZ2UjFDQjUzdjdyOW1Qb210SUlVd2doUWM9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K\n    client-key-data: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUJwZVBRdk1URGsyY3NjMm1DSzhoQjA1UDF4Rlp2NXZ3NHVrWWVET1d1ekFvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFYmRpRXFRa3orbUY2MUVDVTNRektBV0h5Z1k0NjlvWjlKUVRwNWRMekcwbjNtMzZnL0QwbAoySC93clpHTWZyU3JxNW1iU2RVS205dFRKdVh4REQ4ZVFBPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo="
	}
	restConfig, err := logic.K8s{}.MakeK8sConfig(k8sConfig)
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		return
	}
	result, err := url.Parse(restConfig.Host)
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		return
	}

	// 3. 设置HTTP代理
	ctx.Request.URL.Path = params.Path
	ctx.Request.Header.Set("Authorization", "Bearer "+restConfig.BearerToken)
	proxy := httputil.NewSingleHostReverseProxy(result)
	tr, err := rest.TransportFor(restConfig)
	if err != nil {
		c.JsonResponseWithServerError(ctx, err)
		return
	}
	proxy.Transport = tr
	proxy.ServeHTTP(ctx.Writer, ctx.Request)

	return
}
