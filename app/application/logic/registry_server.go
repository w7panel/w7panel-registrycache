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

func (l RegistryServer) ResetRegistryServerSelector(host string, registries []RegistrySource) {
	registryServerUrls := make([]string, 0, len(registries))
	for _, registry := range registries {
		if registry.ServerUrl == "" {
			continue
		}
		registryServerUrls = append(registryServerUrls, registry.ServerUrl)
	}
	defaultRegistryServerMap.Store(host, registryServerUrls)
}

func (l RegistryServer) WalkRegistrySourcesFromRule(ctx context.Context, host string, cacheRule *RepositoryCacheRule, checkAvailable func(registry string) bool) {
	assignRegistry := ""
	if cacheRule != nil {
		assignRegistry = cacheRule.AssignRegistry
	}
	if assignRegistry != "" {
		if l.contextDone(ctx) {
			return
		}
		if checkAvailable(assignRegistry) {
			return
		}
		return
	}

	val, ok := defaultRegistryServerMap.Load(host)
	if !ok {
		return
	}
	registryServerUrls := val.([]string)
	if len(registryServerUrls) == 0 {
		return
	}

	for _, index := range rand.Perm(len(registryServerUrls)) {
		if l.contextDone(ctx) {
			return
		}

		serverUrl := registryServerUrls[index]
		if checkAvailable(serverUrl) {
			return
		}
	}
}

func (l RegistryServer) WalkManifestRegistrySourcesFromRule(ctx context.Context, host, repositoryName, reference string, cacheRule *RepositoryCacheRule, handleAvailable func(registry string, registryClient client.Client, manifest *distribution.Descriptor) bool) {
	l.WalkRegistrySourcesFromRule(ctx, host, cacheRule, func(registryServerUrl string) bool {
		registryClient := RegistryClient{}.GetRegistryClient(host, registryServerUrl, nil)
		if registryClient == nil {
			slog.Info("manifest: GetRegistryClient with registry source", "source", registryServerUrl, "err", "sourceRegistryClient init fail")
			return false
		}

		_, manifest, err := registryClient.ManifestExist(repositoryName, reference)
		slog.Info("manifest: StatObject with registry source", "url", registryServerUrl, "repoName", repositoryName, "reference", reference, "exists", manifest, "err", err)
		if err != nil || manifest == nil {
			return false
		}
		if handleAvailable == nil {
			return true
		}
		return handleAvailable(registryServerUrl, registryClient, manifest)
	})
}

func (l RegistryServer) WalkBlobRegistrySourcesFromRule(ctx context.Context, host, repositoryName, digest string, cacheRule *RepositoryCacheRule, handleAvailable func(registry string, registryClient client.Client, blobSize int64, blobDigest string) bool) {
	l.WalkRegistrySourcesFromRule(ctx, host, cacheRule, func(registryServerUrl string) bool {
		registryClient := RegistryClient{}.GetRegistryClient(host, registryServerUrl, nil)
		if registryClient == nil {
			slog.Info("blob: GetRegistryClient with registry source", "source", registryServerUrl, "err", "sourceRegistryClient init fail")
			return false
		}

		exists, blobSize, blobDigest, err := registryClient.BlobExist(repositoryName, digest)
		slog.Info("blob: StatObject with registry source", "url", registryServerUrl, "repoName", repositoryName, "digest", digest, "exists", exists, "err", err)
		if err != nil || !exists {
			return false
		}
		if handleAvailable == nil {
			return true
		}
		return handleAvailable(registryServerUrl, registryClient, blobSize, blobDigest)
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
