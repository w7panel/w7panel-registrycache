package logic

import (
	"context"
	"log/slog"
	"sort"
	"sync"

	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/client"
	"github.com/docker/distribution"
)

var defaultRegistryServerMap = sync.Map{}

type RegistryServer struct {
	logic
}

type registrySourceResult struct {
	registry       string
	registryClient client.Client
	manifest       *distribution.Descriptor
	blobSize       int64
	blobDigest     string
}

func (l RegistryServer) ResetRegistryServerSelector(host string, registries []RegistrySource) {
	weightedRegistries := make([]RegistrySource, 0, len(registries))
	registryIndexes := make(map[string]int, len(registries))
	for _, registry := range registries {
		if registry.ServerUrl == "" {
			continue
		}
		index, exists := registryIndexes[registry.ServerUrl]
		if !exists {
			registryIndexes[registry.ServerUrl] = len(weightedRegistries)
			weightedRegistries = append(weightedRegistries, registry)
			continue
		}
		if registry.Weight < weightedRegistries[index].Weight {
			weightedRegistries[index] = registry
		}
	}
	sort.SliceStable(weightedRegistries, func(i, j int) bool {
		return weightedRegistries[i].Weight < weightedRegistries[j].Weight
	})
	registryURLs := make([]string, 0, len(weightedRegistries))
	for _, registry := range weightedRegistries {
		registryURLs = append(registryURLs, registry.ServerUrl)
	}
	defaultRegistryServerMap.Store(host, registryURLs)
}

func (l RegistryServer) walkRegistrySourcesFromRule(ctx context.Context, host string, cacheRule *RepositoryCacheRule, check func(registry string) registrySourceResult, handle func(result registrySourceResult) bool) {
	sources := l.registrySourcesFromRule(ctx, host, cacheRule)
	if len(sources) == 0 {
		return
	}

	resultChan := make(chan registrySourceResult, len(sources))
	for _, serverURL := range sources {
		go func(serverURL string) {
			if check == nil {
				resultChan <- registrySourceResult{registry: serverURL}
				return
			}
			resultChan <- check(serverURL)
		}(serverURL)
	}

	for i := 0; i < len(sources); i++ {
		select {
		case <-ctx.Done():
			return
		case result := <-resultChan:
			if result.registry != "" {
				if handle == nil || handle(result) {
					return
				}
			}
		}
	}
}

func (l RegistryServer) registrySourcesFromRule(ctx context.Context, host string, cacheRule *RepositoryCacheRule) []string {
	if l.contextDone(ctx) {
		return nil
	}
	if cacheRule != nil && cacheRule.AssignRegistry != "" {
		return []string{cacheRule.AssignRegistry}
	}

	val, ok := defaultRegistryServerMap.Load(host)
	if !ok {
		return nil
	}
	return val.([]string)
}

func (l RegistryServer) WalkManifestRegistrySourcesFromRule(ctx context.Context, host, repositoryName, reference string, cacheRule *RepositoryCacheRule, handleAvailable func(registry string, registryClient client.Client, manifest *distribution.Descriptor) bool) {
	l.walkRegistrySourcesFromRule(ctx, host, cacheRule, func(registryServerUrl string) registrySourceResult {
		registryClient := RegistryClient{}.GetRegistryClient(host, registryServerUrl, nil)
		if registryClient == nil {
			slog.Info("manifest: GetRegistryClient with registry source", "source", registryServerUrl, "err", "sourceRegistryClient init fail")
			return registrySourceResult{}
		}

		_, manifest, err := registryClient.ManifestExist(repositoryName, reference)
		slog.Info("manifest: StatObject with registry source", "url", registryServerUrl, "repoName", repositoryName, "reference", reference, "exists", manifest, "err", err)
		if err != nil || manifest == nil {
			return registrySourceResult{}
		}

		return registrySourceResult{
			registry:       registryServerUrl,
			registryClient: registryClient,
			manifest:       manifest,
		}
	}, func(result registrySourceResult) bool {
		if handleAvailable == nil {
			return true
		}
		return handleAvailable(result.registry, result.registryClient, result.manifest)
	})
}

func (l RegistryServer) WalkBlobRegistrySourcesFromRule(ctx context.Context, host, repositoryName, digest string, cacheRule *RepositoryCacheRule, handleAvailable func(registry string, registryClient client.Client, blobSize int64, blobDigest string) bool) {
	l.walkRegistrySourcesFromRule(ctx, host, cacheRule, func(registryServerUrl string) registrySourceResult {
		registryClient := RegistryClient{}.GetRegistryClient(host, registryServerUrl, nil)
		if registryClient == nil {
			slog.Info("blob: GetRegistryClient with registry source", "source", registryServerUrl, "err", "sourceRegistryClient init fail")
			return registrySourceResult{}
		}

		exists, blobSize, blobDigest, err := registryClient.BlobExist(repositoryName, digest)
		slog.Info("blob: StatObject with registry source", "url", registryServerUrl, "repoName", repositoryName, "digest", digest, "exists", exists, "err", err)
		if err != nil || !exists {
			return registrySourceResult{}
		}

		return registrySourceResult{
			registry:       registryServerUrl,
			registryClient: registryClient,
			blobSize:       blobSize,
			blobDigest:     blobDigest,
		}
	}, func(result registrySourceResult) bool {
		if handleAvailable == nil {
			return true
		}
		return handleAvailable(result.registry, result.registryClient, result.blobSize, result.blobDigest)
	})
}

func (l RegistryServer) contextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
