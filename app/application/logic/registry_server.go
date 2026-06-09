package logic

import (
	"context"
	"log/slog"
	"math/rand"
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
	registryServerUrls := make([]string, 0, len(registries))
	seen := make(map[string]bool, len(registries))
	for _, registry := range registries {
		if registry.ServerUrl == "" || seen[registry.ServerUrl] {
			continue
		}
		seen[registry.ServerUrl] = true
		registryServerUrls = append(registryServerUrls, registry.ServerUrl)
	}
	defaultRegistryServerMap.Store(host, registryServerUrls)
}

func (l RegistryServer) walkRegistrySourcesFromRule(ctx context.Context, host string, cacheRule *RepositoryCacheRule, check func(registry string) registrySourceResult, handle func(result registrySourceResult) bool) {
	sources := l.registrySourcesFromRule(ctx, host, cacheRule)
	if len(sources) == 0 {
		return
	}

	resultChan := make(chan registrySourceResult, len(sources))
	for _, serverUrl := range sources {
		go func(serverUrl string) {
			if check == nil {
				resultChan <- registrySourceResult{registry: serverUrl}
				return
			}
			resultChan <- check(serverUrl)
		}(serverUrl)
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
	assignRegistry := ""
	if cacheRule != nil {
		assignRegistry = cacheRule.AssignRegistry
	}
	if assignRegistry != "" {
		if l.contextDone(ctx) {
			return nil
		}
		return []string{assignRegistry}
	}

	val, ok := defaultRegistryServerMap.Load(host)
	if !ok {
		return nil
	}
	registryServerUrls := val.([]string)
	if len(registryServerUrls) == 0 {
		return nil
	}

	sources := make([]string, 0, len(registryServerUrls))
	for _, index := range rand.Perm(len(registryServerUrls)) {
		if l.contextDone(ctx) {
			return sources
		}

		sources = append(sources, registryServerUrls[index])
	}

	return sources
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
