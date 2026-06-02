package logic

import (
	"fmt"
	"math/rand"
	"slices"
	"sync"
	"time"

	"gitee.com/we7coreteam/w7-registry-cache/common/helper"
	"github.com/patrickmn/go-cache"
)

type RegistryReqInfo struct {
	Total   int
	Success int
}

type RegistrySelector struct {
	registries []string   // 所有镜像
	mu         sync.Mutex // 互斥锁保证并发安全
	cache      *cache.Cache
}

func NewRegistrySelector(registries []string) *RegistrySelector {
	return &RegistrySelector{
		registries: registries,
		cache:      cache.New(cache.NoExpiration, time.Minute*30),
	}
}

func (s *RegistrySelector) GetRegistryNum() int {
	return len(s.registries)
}

// Select 根据策略选择镜像源
func (s *RegistrySelector) Select(repositoryName, reference string, fixedServerUrl string) string {
	if len(s.registries) == 0 {
		return ""
	}

	if fixedServerUrl != "" {
		return s.handleFixed(fixedServerUrl)
	}
	return s.handleWeight(repositoryName, reference)
}

// 处理固定策略
func (s *RegistrySelector) handleFixed(fixedServerUrl string) string {
	// 如果指定地址存在则返回，否则返回第一个
	for _, m := range s.registries {
		if m == fixedServerUrl {
			return fixedServerUrl
		}
	}
	return ""
}

// 处理权重策略
func (s *RegistrySelector) handleWeight(repositoryName, reference string) string {
	notExistsRegistryServer := s.getRepositoryReferenceNotExistsRegistries(repositoryName, reference)
	existsRegistryServerUrls, _ := helper.GetDifference(s.registries, notExistsRegistryServer)
	if existsRegistryServerUrls == nil || len(existsRegistryServerUrls) == 0 {
		rand.Seed(time.Now().UnixNano())
		// 生成随机索引
		index := rand.Intn(len(s.registries))
		return s.registries[index]
	}

	totalWeight := 0.0
	for _, m := range existsRegistryServerUrls {
		totalWeight += s.getRepositoryReferenceRegistrySuccessRate(repositoryName, reference, m)
	}

	// 随机选择（基于权重）
	randWeight := rand.Float64() * totalWeight
	currentWeight := 0.0

	for _, backend := range existsRegistryServerUrls {
		weight := s.getRepositoryReferenceRegistrySuccessRate(repositoryName, reference, backend)

		currentWeight += weight
		if randWeight < currentWeight {
			return backend
		}
	}

	return ""
}

func (s *RegistrySelector) getRepositoryReferenceNotExistsRegistries(repositoryName, reference string) []string {
	var notExistsRegistryServer []string
	val, exists := s.cache.Get(fmt.Sprintf("not:exists:%s:%s", repositoryName, reference))
	if exists {
		notExistsRegistryServer = val.([]string)
	} else {
		notExistsRegistryServer = make([]string, 0)
	}

	return notExistsRegistryServer
}

func (s *RegistrySelector) RecordRepositoryReferenceNotExistsAtRegistry(repositoryName, reference, registryServerUrl string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	notExistsRegistryServer := s.getRepositoryReferenceNotExistsRegistries(repositoryName, reference)
	notExistsRegistryServer = append(notExistsRegistryServer, registryServerUrl)

	s.cache.Set(fmt.Sprintf("not:exists:%s:%s", repositoryName, reference), notExistsRegistryServer, cache.NoExpiration)
}

func (s *RegistrySelector) RemoveRepositoryReferenceNotExistsAtRegistry(repositoryName, reference, registryServerUrl string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	notExistsRegistryServer := s.getRepositoryReferenceNotExistsRegistries(repositoryName, reference)
	index := slices.Index(notExistsRegistryServer, registryServerUrl)
	if index >= 0 {
		notExistsRegistryServer = append(notExistsRegistryServer[:index], notExistsRegistryServer[index+1:]...)
	}

	s.cache.Set(fmt.Sprintf("not:exists:%s:%s", repositoryName, reference), notExistsRegistryServer, cache.NoExpiration)
}

func (s *RegistrySelector) getRepositoryReferenceRegistrySuccessRate(repositoryName, reference, registryServerUrl string) float64 {
	cacheKey := fmt.Sprintf("weigth:%s:%s:%s", repositoryName, reference, registryServerUrl)
	val, exists := s.cache.Get(cacheKey)
	if exists {
		info := val.(*RegistryReqInfo)

		return min(float64(info.Success)/float64(info.Total), 1.0)
	}
	return 1.0
}

func (s *RegistrySelector) RecordRepositoryReferenceRegistryWeight(repositoryName, reference, registryServerUrl string, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	incr := 1
	if !success {
		incr = -1
	}

	info := &RegistryReqInfo{
		Total:   0,
		Success: 0,
	}
	cacheKey := fmt.Sprintf("weigth:%s:%s:%s", repositoryName, reference, registryServerUrl)
	val, exists := s.cache.Get(cacheKey)
	if exists {
		info = val.(*RegistryReqInfo)
	}
	info.Total++

	if info.Total > 1000 {
		info.Total -= 1000
		info.Success -= 1000
		if info.Success < 0 {
			info.Success = min(5, info.Total)
		}
	}
	info.Success += incr

	s.cache.Set(cacheKey, info, cache.NoExpiration)
}
