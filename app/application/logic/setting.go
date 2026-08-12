package logic

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/types"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

var settingFileSuffix = "-setting.json"

const globalSettingGroup = "global"

type CacheStorageRegistry struct {
	ServerUrl            string `json:"server_url"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	CacheNamespacePrefix string `json:"cache_namespace_prefix"`
}

type RegistrySource struct {
	ServerUrl string `json:"server_url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Weight    int    `json:"weight"`
	Proxy     struct {
		ServerUrl string `json:"server_url"`
		Port      int    `json:"port"`
	} `json:"proxy"`
}

type RegistryCacheSetting struct {
	Host                 string                 `json:"group"`
	CacheRegistry        CacheStorageRegistry   `json:"cache_registry"`
	RepositoryCacheRules []RepositoryCacheRule  `json:"cache_rules"`
	RegistrySources      []RegistrySource       `json:"registry_sources"`
	OriginRegistry       RegistrySource         `json:"origin_registry"`
	Extra                map[string]interface{} `json:"extra"`
	Parent               string                 `json:"parent"`
}

var defaultStorageSettingMap = sync.Map{}

type Setting struct {
	logic
}

func (l Setting) SetStorageCacheSetting(host string, cacheSetting RegistryCacheSetting) error {
	if cacheSetting.RepositoryCacheRules != nil {
		sort.Slice(cacheSetting.RepositoryCacheRules, func(i, j int) bool {
			return cacheSetting.RepositoryCacheRules[i].Weight < cacheSetting.RepositoryCacheRules[j].Weight
		})
	}
	if err := l.NormalizeCacheRegistry(&cacheSetting.CacheRegistry); err != nil {
		return err
	}
	if host != globalSettingGroup && cacheRepositoryMode(cacheSetting.Extra) == "global" {
		// 全局模式只继承仓库连接信息，保留站点自己的存储目录。
		cacheSetting.CacheRegistry.ServerUrl = ""
		cacheSetting.CacheRegistry.Username = ""
		cacheSetting.CacheRegistry.Password = ""
	}

	settingContent, err := json.Marshal(cacheSetting)
	if err != nil {
		return err
	}

	settingSaveDir := filepath.Dir(facade.GetConfig().GetString("database.default.db_name"))
	err = os.MkdirAll(settingSaveDir, 0755)
	settingSavePath := filepath.Join(settingSaveDir, host+settingFileSuffix)
	err = os.WriteFile(settingSavePath, settingContent, 0644)
	if err != nil {
		return err
	}

	registryClients := registryClientSources(cacheSetting)
	if len(registryClients) > 0 {
		err = RegistryClient{}.ResetRegistryClient(cacheSetting.Host, registryClients)
		if err != nil {
			return err
		}
	}

	if cacheSetting.CacheRegistry.Username != "" {
		RegistryClient{}.SetRegistryClientCredential(cacheSetting.Host, cacheSetting.CacheRegistry.ServerUrl, types.Registry{
			Credential: &types.Credential{
				AccessKey:    cacheSetting.CacheRegistry.Username,
				AccessSecret: cacheSetting.CacheRegistry.Password,
			}},
		)
	}

	defaultStorageSettingMap.Delete(host)
	RegistryServer{}.ResetRegistryServerSelector(host, registrySelectorSources(cacheSetting))

	return nil
}

func (l Setting) GetStorageCacheSetting(host string) *RegistryCacheSetting {
	cacheSetting := &RegistryCacheSetting{}
	val, exists := defaultStorageSettingMap.Load(host)
	if !exists {
		settingSaveDir := filepath.Dir(facade.GetConfig().GetString("database.default.db_name"))
		settingSavePath := filepath.Join(settingSaveDir, host+settingFileSuffix)
		if _, err := os.Stat(settingSavePath); os.IsNotExist(err) {
			return nil
		}

		content, err := os.ReadFile(settingSavePath)
		if err != nil {
			slog.Error("GetStorageCacheSetting: os.ReadFile(settingSavePath) error(%v)", err)
			return cacheSetting
		}
		err = json.Unmarshal(content, &cacheSetting)
		if err != nil {
			slog.Error("GetStorageCacheSetting: json.Unmarshal() error(%v)", err)
			return nil
		}

		registryClients := registryClientSources(*cacheSetting)
		if len(registryClients) > 0 {
			err = RegistryClient{}.ResetRegistryClient(cacheSetting.Host, registryClients)
			if err != nil {
				slog.Error("GetStorageCacheSetting: ResetRegistryClient() error(%v)", err)
			}

			RegistryServer{}.ResetRegistryServerSelector(host, registrySelectorSources(*cacheSetting))
		}

		if cacheSetting.CacheRegistry.Username != "" {
			RegistryClient{}.SetRegistryClientCredential(cacheSetting.Host, cacheSetting.CacheRegistry.ServerUrl, types.Registry{
				Credential: &types.Credential{
					AccessKey:    cacheSetting.CacheRegistry.Username,
					AccessSecret: cacheSetting.CacheRegistry.Password,
				}},
			)
		}

		defaultStorageSettingMap.Store(host, cacheSetting)
	} else {
		cacheSetting = val.(*RegistryCacheSetting)
	}

	if host == globalSettingGroup || cacheRepositoryMode(cacheSetting.Extra) != "global" {
		return cacheSetting
	}

	globalSetting := l.GetStorageCacheSetting(globalSettingGroup)
	if globalSetting == nil || globalSetting.CacheRegistry.ServerUrl == "" {
		return cacheSetting
	}

	resolved := *cacheSetting
	resolved.CacheRegistry = resolveGlobalCacheRegistry(globalSetting.CacheRegistry, *cacheSetting)
	if resolved.CacheRegistry.ServerUrl != "" {
		RegistryClient{}.SetRegistryClientCredential(resolved.Host, resolved.CacheRegistry.ServerUrl, types.Registry{
			Credential: &types.Credential{
				AccessKey:    resolved.CacheRegistry.Username,
				AccessSecret: resolved.CacheRegistry.Password,
			},
		})
	}
	return &resolved
}

func resolveGlobalCacheRegistry(globalRegistry CacheStorageRegistry, siteSetting RegistryCacheSetting) CacheStorageRegistry {
	globalRegistry.CacheNamespacePrefix = siteSetting.CacheRegistry.CacheNamespacePrefix
	return globalRegistry
}

func registryClientSources(setting RegistryCacheSetting) []RegistrySource {
	sources := append([]RegistrySource(nil), setting.RegistrySources...)
	if setting.OriginRegistry.ServerUrl != "" {
		sources = append(sources, setting.OriginRegistry)
	}
	return sources
}

func registrySelectorSources(setting RegistryCacheSetting) []RegistrySource {
	sources := append([]RegistrySource(nil), setting.RegistrySources...)
	if setting.OriginRegistry.ServerUrl != "" {
		origin := setting.OriginRegistry
		origin.Weight = math.MaxInt
		sources = append(sources, origin)
	}
	return sources
}

func cacheRepositoryMode(extra map[string]interface{}) string {
	repository, ok := extra["cache_repository"].(map[string]interface{})
	if !ok {
		return ""
	}
	mode, _ := repository["mode"].(string)
	return mode
}

func (l Setting) DelStorageCacheSetting(host string) {
	settingSaveDir := filepath.Dir(facade.GetConfig().GetString("database.default.db_name"))
	settingSavePath := filepath.Join(settingSaveDir, host+settingFileSuffix)
	os.Remove(settingSavePath)

	defaultStorageSettingMap.Delete(host)
}

func (l Setting) StorageCacheList() (map[string]*RegistryCacheSetting, error) {
	settingSaveDir := filepath.Dir(facade.GetConfig().GetString("database.default.db_name"))

	entries, err := os.ReadDir(settingSaveDir)
	if err != nil {
		return nil, err
	}

	list := make(map[string]*RegistryCacheSetting)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), settingFileSuffix) {
			continue
		}

		host := strings.TrimSuffix(entry.Name(), settingFileSuffix)
		if host == "" {
			continue
		}
		isetting := l.GetStorageCacheSetting(host)
		if isetting != nil {
			list[host] = isetting
		}
	}
	return list, nil
}

func (l Setting) NormalizeCacheRegistry(cacheRegistry *CacheStorageRegistry) error {
	if cacheRegistry.ServerUrl == "" {
		return nil
	}

	rawURL := strings.TrimRight(cacheRegistry.ServerUrl, "/")
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid cache registry server url: %s", cacheRegistry.ServerUrl)
	}

	cacheRegistry.ServerUrl = rawURL
	path := strings.Trim(parsedURL.Path, "/")
	if path == "" {
		return nil
	}
	if err = l.checkRegistryEndpoint(rawURL, cacheRegistry.Username, cacheRegistry.Password); err == nil {
		return nil
	}

	baseURL := *parsedURL
	currentPath := path
	for currentPath != "" {
		parts := strings.Split(currentPath, "/")
		currentPath = strings.Join(parts[:len(parts)-1], "/")
		baseURL.Path = buildCacheRegistryPath(currentPath)
		candidate := strings.TrimRight(baseURL.String(), "/")

		if probeErr := l.checkRegistryEndpoint(candidate, cacheRegistry.Username, cacheRegistry.Password); probeErr == nil {
			prefix := strings.Trim(strings.TrimPrefix(path, currentPath), "/")
			cacheRegistry.ServerUrl = candidate
			cacheRegistry.CacheNamespacePrefix = mergeCacheNamespacePrefix(cacheRegistry.CacheNamespacePrefix, prefix)
			return nil
		}
	}

	return err
}

func (l Setting) checkRegistryEndpoint(serverURL, username, password string) error {
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(serverURL, "/")+"/v2/", nil)
	if err != nil {
		return err
	}
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		return nil
	}

	return fmt.Errorf("registry endpoint check failed: %s status=%d", serverURL, resp.StatusCode)
}

func buildCacheRegistryPath(path string) string {
	if path == "" {
		return ""
	}
	return "/" + strings.Trim(path, "/")
}

func mergeCacheNamespacePrefix(currentPrefix, detectedPrefix string) string {
	detectedPrefix = strings.Trim(detectedPrefix, "/")
	currentPrefix = strings.Trim(currentPrefix, "/")
	if currentPrefix == "" {
		return detectedPrefix
	}

	return detectedPrefix + "/" + currentPrefix
}
