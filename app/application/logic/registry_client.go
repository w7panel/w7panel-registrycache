package logic

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	registry2 "gitee.com/we7coreteam/w7-registry-cache/common/service/registry"
	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/client"
	commonhttp "gitee.com/we7coreteam/w7-registry-cache/common/service/registry/http"
	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/types"
	"github.com/docker/distribution/registry/client/auth/challenge"
)

var defaultRegistryClientCredentialMap = sync.Map{}

type RegistryClient struct {
	logic
}

func (l RegistryClient) SetRegistryClientCredential(groupHost string, registryUrl string, credential types.Registry) {
	defaultRegistryClientCredentialMap.Store(groupHost+registryUrl, credential)
}

func (l RegistryClient) ResetRegistryClient(groupHost string, registries []RegistrySource) error {
	for _, registry := range registries {
		proxyUrl := ""
		if registry.Proxy.ServerUrl != "" {
			proxyUrl = registry.Proxy.ServerUrl
			if !strings.HasPrefix(proxyUrl, "http") && !strings.HasPrefix(proxyUrl, "https") {
				proxyUrl = "http://" + registry.Proxy.ServerUrl
			}
			if registry.Proxy.Port != 0 {
				proxyUrl = fmt.Sprintf("%s:%d", proxyUrl, registry.Proxy.Port)
			}
		}
		l.SetRegistryClientCredential(groupHost, registry.ServerUrl, types.Registry{
			Credential: &types.Credential{
				AccessKey:    registry.Username,
				AccessSecret: registry.Password,
			},
			ProxyUrl: proxyUrl,
		})
	}

	return nil
}

func (l RegistryClient) GetRegistryClient(groupHost string, registryUrl string, headers http.Header) client.Client {
	cred := types.Credential{
		AccessKey:    "",
		AccessSecret: "",
		Region:       "",
	}
	registryConfig := types.Registry{
		Credential: &cred,
		ProxyUrl:   "",
	}
	registryConfigVal, exists := defaultRegistryClientCredentialMap.Load(groupHost + registryUrl)
	if exists {
		registryConfig = registryConfigVal.(types.Registry)
		registryConfig.Credential = &types.Credential{
			AccessKey:    registryConfig.Credential.AccessKey,
			AccessSecret: registryConfig.Credential.AccessSecret,
			Type:         registryConfig.Credential.Type,
			Region:       registryConfig.Credential.Region,
		}
	}

	if headers != nil && headers.Get("Authorization") != "" {
		registryConfig.Credential.AccessKey = ""
		registryConfig.Credential.AccessSecret = ""
		header := strings.Split(headers.Get("Authorization"), "Bearer ")
		if len(header) == 2 {
			registryConfig.Credential.Type = types.CredentialTypeOAuth
			registryConfig.Credential.Token = header[1]
			setting := Setting{}.GetStorageCacheSetting(groupHost)
			if setting != nil && setting.CacheRegistry.CacheNamespacePrefix != "" {
				sourceRepository, actions, err := l.parseRepositoryScopeFromBearerToken(header[1])
				if err != nil {
					slog.Warn("parse repository from bearer token failed", "registryUrl", registryUrl, "err", err)
				} else if sourceRepository != "" && (!strings.HasPrefix(sourceRepository, setting.CacheRegistry.CacheNamespacePrefix+"/")) {
					targetRepository := l.RebuildImageName(sourceRepository, setting.CacheRegistry.CacheNamespacePrefix)
					scopedToken, err := l.exchangeRegistryToken(registryUrl, registryConfig.ProxyUrl, headers.Get("Authorization"), targetRepository, actions)
					if err != nil {
						slog.Warn("exchange registry token failed", "registryUrl", registryUrl, "targetRepository", targetRepository, "err", err)
					} else if scopedToken != "" {
						registryConfig.Credential.Token = scopedToken
					}
				}
			}
		}
	}

	tclient, err := registry2.NewRegistryClient(registryUrl, registryConfig.Credential, registryConfig.ProxyUrl)
	slog.Info("new registry client", registryConfig, "url", registryUrl)
	if err != nil {
		slog.Error("registry client init err", "groupHost", groupHost, "registryUrl", registryUrl, "err", err)
		return nil
	}

	return tclient
}

func (l RegistryClient) exchangeRegistryToken(registryUrl string, proxyUrl string, authorization string, targetRepository string, actions []string) (string, error) {
	httpClient := &http.Client{
		Transport: commonhttp.GetHTTPTransport(commonhttp.WithInsecure(true)),
	}
	if proxyUrl != "" {
		proxyURL, err := url.Parse(proxyUrl)
		if err != nil {
			return "", err
		}
		if transport, ok := httpClient.Transport.(*http.Transport); ok {
			transport.Proxy = http.ProxyURL(proxyURL)
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
	}

	pingReq, err := http.NewRequest(http.MethodGet, strings.TrimRight(registryUrl, "/")+"/v2/", nil)
	if err != nil {
		return "", err
	}
	pingResp, err := httpClient.Do(pingReq)
	if err != nil {
		return "", err
	}
	defer pingResp.Body.Close()

	var bearerChallenge challenge.Challenge
	for _, c := range challenge.ResponseChallenges(pingResp) {
		if strings.EqualFold(c.Scheme, "bearer") {
			bearerChallenge = c
			break
		}
	}
	if bearerChallenge.Scheme == "" {
		return "", nil
	}

	realm := bearerChallenge.Parameters["realm"]
	if realm == "" {
		return "", nil
	}

	tokenURL, err := url.Parse(realm)
	if err != nil {
		return "", err
	}
	query := tokenURL.Query()
	if service := bearerChallenge.Parameters["service"]; service != "" {
		query.Set("service", service)
	}
	actionStr := "pull"
	if len(actions) > 0 {
		actionStr = strings.Join(actions, ",")
	}
	query.Add("scope", fmt.Sprintf("repository:%s:%s", strings.Trim(targetRepository, "/"), actionStr))
	tokenURL.RawQuery = query.Encode()

	tokenReq, err := http.NewRequest(http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", err
	}
	tokenReq.Header.Set("Authorization", authorization)

	tokenResp, err := httpClient.Do(tokenReq)
	if err != nil {
		return "", err
	}
	defer tokenResp.Body.Close()

	body, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		return "", err
	}
	if tokenResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token service status=%d body=%s", tokenResp.StatusCode, string(body))
	}

	var tokenResult struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err = json.Unmarshal(body, &tokenResult); err != nil {
		return "", err
	}
	if tokenResult.Token != "" {
		return tokenResult.Token, nil
	}
	return tokenResult.AccessToken, nil
}

func (l RegistryClient) parseRepositoryScopeFromBearerToken(token string) (string, []string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", nil, fmt.Errorf("invalid jwt token")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, err
	}

	var claims struct {
		Access []struct {
			Type    string   `json:"type"`
			Name    string   `json:"name"`
			Actions []string `json:"actions"`
		} `json:"access"`
		Scope interface{} `json:"scope"`
	}
	if err = json.Unmarshal(payload, &claims); err != nil {
		return "", nil, err
	}

	for _, access := range claims.Access {
		if access.Type == "repository" && access.Name != "" {
			return access.Name, access.Actions, nil
		}
	}

	switch scope := claims.Scope.(type) {
	case string:
		return parseRepositoryScope(scope)
	case []interface{}:
		for _, item := range scope {
			if scopeStr, ok := item.(string); ok {
				repository, actions, err := parseRepositoryScope(scopeStr)
				if err == nil && repository != "" {
					return repository, actions, nil
				}
			}
		}
	}

	return "", nil, fmt.Errorf("repository scope not found in token")
}

func parseRepositoryScope(scope string) (string, []string, error) {
	scope = strings.TrimSpace(scope)
	if !strings.HasPrefix(scope, "repository:") {
		return "", nil, fmt.Errorf("not repository scope")
	}

	parts := strings.SplitN(scope, ":", 3)
	if len(parts) != 3 {
		return "", nil, fmt.Errorf("invalid repository scope")
	}

	repository := strings.Trim(parts[1], "/")
	actions := make([]string, 0)
	for _, action := range strings.Split(parts[2], ",") {
		action = strings.TrimSpace(action)
		if action != "" {
			actions = append(actions, action)
		}
	}

	return repository, actions, nil
}
