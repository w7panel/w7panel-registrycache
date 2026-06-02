package logic

import (
	"context"
	"sync"
)

var defaultRegistryServerSelectorMap = sync.Map{}

type RegistryServer struct {
	logic
}

func (l RegistryServer) ResetRegistryServerSelector(host string, registries []RegistrySource) {
	registryServerUrls := make([]string, len(registries))
	for i, registry := range registries {
		registryServerUrls[i] = registry.ServerUrl
	}
	defaultRegistryServerSelectorMap.Store(host, NewRegistrySelector(registryServerUrls))
}

func (l RegistryServer) GetRegistrySourceFromRule(ctx context.Context, host, repositoryName, reference string, cacheRule *RepositoryCacheRule, checkAvailable func(registry string) bool) string {
	val, ok := defaultRegistryServerSelectorMap.Load(host)
	if !ok {
		return ""
	}
	selector := val.(*RegistrySelector)
	allRegistryNum := selector.GetRegistryNum()
	curNum := 0

	for {
		select {
		case <-ctx.Done():
			return ""
		default:
		}

		if curNum > allRegistryNum {
			return ""
		}

		serverUrl := selector.Select(repositoryName, reference, cacheRule.AssignRegistry)
		if serverUrl == "" {
			return ""
		}
		if checkAvailable(serverUrl) {
			selector.RemoveRepositoryReferenceNotExistsAtRegistry(repositoryName, reference, serverUrl)
			return serverUrl
		} else {
			selector.RecordRepositoryReferenceNotExistsAtRegistry(repositoryName, reference, serverUrl)
		}

		curNum++
	}
}

func (l RegistryServer) RecordRepositoryReferenceRegistryWeight(host, repositoryName, reference, registryServerUrl string, success bool) {
	val, ok := defaultRegistryServerSelectorMap.Load(host)
	if !ok {
		return
	}
	selector := val.(*RegistrySelector)

	selector.RecordRepositoryReferenceRegistryWeight(repositoryName, reference, registryServerUrl, success)
}
