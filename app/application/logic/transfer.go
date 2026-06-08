package logic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/client"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema2"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/panjf2000/ants/v2"
)

var transferMap sync.Map
var transferChan chan TransferInfo
var transferPool *ants.PoolWithFunc

func init() {
	transferChan = make(chan TransferInfo, 1000)

	pool, err := ants.NewPoolWithFunc(100, func(transfer interface{}) {
		Transfer{}.transfer(transfer.(TransferInfo))
	})
	if err != nil {
		panic(err)
	}
	transferPool = pool
}

const TransferTypeBlob = int8(1)
const TransferTypeManifest = int8(2)
const TransferSourceCheckTimeout = 10 * time.Second

type TransferInfo struct {
	CacheSetting            RegistryCacheSetting
	SourceRegistryServerUrl string
	RepositoryName          string
	Reference               string
	Type                    int8
	CurReTryNum             int8
}

type TransferSourceClient struct {
	serverUrl string
	client    client.Client
}

type SyncWriter struct {
	buffer           [][]byte
	startOffset      int64
	endOffset        int64
	RegistryClient   client.Client
	RepositoryName   string
	RepositoryDigest string
	location         string
}

func (w *SyncWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	w.buffer = append(w.buffer, p)

	if len(w.buffer) != 2 {
		if w.startOffset == 0 && w.endOffset == 0 {
			w.endOffset = int64(len(p)) - 1
			w.startOffset = 0
		}
	} else {
		location, _, err := w.RegistryClient.PushBlobChunk(w.RepositoryName, w.RepositoryDigest, w.endOffset+2, bytes.NewBuffer(w.buffer[0]), w.startOffset, w.endOffset, w.location)
		if err != nil {
			return 0, err
		}
		w.location = location
		w.startOffset = w.endOffset + 1
		w.endOffset = w.startOffset + int64(len(w.buffer[1])) - 1
		w.buffer = w.buffer[1:]
	}

	return len(p), nil
}

func (w *SyncWriter) End() error {
	if len(w.buffer) == 1 {
		_, _, err := w.RegistryClient.PushBlobChunk(w.RepositoryName, w.RepositoryDigest, w.endOffset+1, bytes.NewBuffer(w.buffer[0]), w.startOffset, w.endOffset, w.location)
		if err != nil {
			return err
		}
	}

	return nil
}

type Transfer struct {
	logic
}

func (l Transfer) deduplicateBlobs(blobs []distribution.Descriptor) []distribution.Descriptor {
	seen := make(map[string]bool)
	result := []distribution.Descriptor{}

	for _, blob := range blobs {
		digestStr := blob.Digest.String()
		if !seen[digestStr] {
			seen[digestStr] = true
			result = append(result, blob)
		}
	}

	return result
}

func (l Transfer) copyableBlobDescriptors(descriptors []distribution.Descriptor) []distribution.Descriptor {
	blobs := make([]distribution.Descriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if l.isForeignLayer(descriptor.MediaType) {
			slog.Warn("skip foreign layer", "digest", descriptor.Digest.String(), "mediaType", descriptor.MediaType)
			continue
		}
		if l.isManifestDescriptor(descriptor.MediaType) {
			continue
		}
		blobs = append(blobs, descriptor)
	}

	return blobs
}

func (l Transfer) isManifestDescriptor(mediaType string) bool {
	switch mediaType {
	case v1.MediaTypeImageIndex,
		manifestlist.MediaTypeManifestList,
		v1.MediaTypeImageManifest,
		schema2.MediaTypeManifest:
		return true
	default:
		return false
	}
}

func (l Transfer) isForeignLayer(mediaType string) bool {
	switch mediaType {
	case schema2.MediaTypeForeignLayer,
		v1.MediaTypeImageLayerNonDistributable,
		v1.MediaTypeImageLayerNonDistributableGzip,
		v1.MediaTypeImageLayerNonDistributableZstd:
		return true
	default:
		return false
	}
}

func (l Transfer) Push(transferInfo TransferInfo) {
	_, exists := transferMap.LoadOrStore(l.transferKey(transferInfo), transferInfo)
	if exists {
		return
	}

	transferChan <- transferInfo
}

func (l Transfer) transfer(transferInfo TransferInfo) {
	slog.Info("transfer begin", "transfer_info", transferInfo)

	defer transferMap.Delete(l.transferKey(transferInfo))

	err := l.transferImage(transferInfo)

	if err == nil {
		err = Storage{}.UpdateRepositoryFile(transferInfo.CacheSetting.Host, transferInfo.RepositoryName, transferInfo.Reference)
	} else if transferInfo.CurReTryNum < 10 {
		go func() {
			time.Sleep(6 * time.Second)
			transferInfo.CurReTryNum++
			slog.Info("transfer failed retry", "transfer_info", transferInfo)
			l.Push(transferInfo)
		}()
	}

	slog.Info("transfer CompleteMultipartUpload", "transfer_info", transferInfo, "err", err)
}

func (l Transfer) transferKey(transferInfo TransferInfo) string {
	return transferInfo.CacheSetting.Host + ":" + transferInfo.RepositoryName + ":" + transferInfo.Reference
}

func (l Transfer) transferImage(transferInfo TransferInfo) error {
	sourceRegistryClient := RegistryClient{}.GetRegistryClient(transferInfo.CacheSetting.Host, transferInfo.SourceRegistryServerUrl, nil)
	if sourceRegistryClient == nil {
		return errors.New("source registry client is nil")
	}

	cacheRegistryClient := RegistryClient{}.GetRegistryClient(transferInfo.CacheSetting.Host, transferInfo.CacheSetting.CacheRegistry.ServerUrl, nil)
	if cacheRegistryClient == nil {
		return errors.New("cache registry client is nil")
	}

	sourceManifest, err := Storage{}.DownloadManifest(context.Background(), sourceRegistryClient.PullManifest, transferInfo.RepositoryName, transferInfo.Reference)
	if err != nil {
		return err
	}

	return l.copyImage(sourceRegistryClient, cacheRegistryClient, sourceManifest, transferInfo)
}

func (l Transfer) copyImage(sourceRepo, targetRepo client.Client, sourceManifest distribution.Manifest, transferInfo TransferInfo) error {
	repoName := transferInfo.RepositoryName
	tag := transferInfo.Reference
	childManifest := make(map[string]distribution.Manifest)
	blobs := make([]distribution.Descriptor, 0)
	_, ok := sourceManifest.(*manifestlist.DeserializedManifestList)
	if ok {
		for _, item := range sourceManifest.(*manifestlist.DeserializedManifestList).Manifests {
			cmanifest, err := Storage{}.DownloadManifest(context.Background(), sourceRepo.PullManifest, repoName, item.Digest.String())
			if err != nil {
				slog.Error("transfer pull platform manifest error", "repoName", repoName, "tag", tag, "err", err)
				return err
			}

			childManifest[item.Digest.String()] = cmanifest
			blobs = append(blobs, l.copyableBlobDescriptors(cmanifest.References())...)
		}
	} else {
		blobs = append(blobs, l.copyableBlobDescriptors(sourceManifest.References())...)
	}

	// 同步所有blobs
	dedupedBlobs := l.deduplicateBlobs(blobs)
	targetRepoName := repoName
	targetRepoName = l.RebuildImageName(targetRepoName, transferInfo.CacheSetting.CacheRegistry.CacheNamespacePrefix)

	slog.Info("transfer blobs", "repoName", repoName, "targetRepoName", targetRepoName, "tag", tag, "blobs", dedupedBlobs)
	sourceRepos := l.transferSourceClients(sourceRepo, transferInfo)
	if err := l.copyBlobs(sourceRepos, targetRepo, repoName, targetRepoName, dedupedBlobs, transferInfo); err != nil {
		return fmt.Errorf("同步blobs失败: %v", err)
	}

	for digest, item := range childManifest {
		mediaType, payload, err := item.Payload()
		if err != nil {
			return err
		}
		_, err = targetRepo.PushManifest(targetRepoName, digest, mediaType, payload)
		slog.Info("transfer push platform manifest", "repoName", targetRepoName, "tag", tag, "digest", digest, "err", err)
		if err != nil {
			return err
		}
	}

	mediaType, payload, err := sourceManifest.Payload()
	if err != nil {
		return err
	}
	_, err = targetRepo.PushManifest(targetRepoName, tag, mediaType, payload)
	slog.Info("transfer push platform manifest", "repoName", targetRepoName, "tag", tag, "err", err)
	return err
}

func (l Transfer) copyBlobs(sourceRepos []TransferSourceClient, targetRepo client.Client, sourceRepoName string, targetRepoName string, blobs []distribution.Descriptor, transferInfo TransferInfo) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(blobs))
	sem := make(chan struct{}, min(10, len(blobs))) // 限制blob并发数

	for blobIndex, blob := range blobs {
		wg.Add(1)
		sem <- struct{}{}

		go func(blobIndex int, blob distribution.Descriptor) {
			defer func() {
				<-sem
				wg.Done()
			}()

			if transferInfo.CurReTryNum != 0 {
				exists, _, _, _ := targetRepo.BlobExist(targetRepoName, blob.Digest.String())
				if exists {
					return
				}
			}

			syncWriter := &SyncWriter{
				RegistryClient:   targetRepo,
				RepositoryName:   targetRepoName,
				RepositoryDigest: blob.Digest.String(),
			}
			sourceRepo := sourceRepos[(blobIndex+int(transferInfo.CurReTryNum))%len(sourceRepos)]
			slog.Info("transfer blob source selected", "repoName", sourceRepoName, "targetRepoName", targetRepoName, "blob", blob.Digest.String(), "blobSize", blob.Size, "source", sourceRepo.serverUrl)
			err := Storage{}.DownloadBlob(context.Background(), sourceRepo.client.PullBlobChunk, sourceRepoName, blob.Digest.String(), blob.Size, syncWriter, false)
			if err == nil {
				err = syncWriter.End()
			}
			if err != nil {
				slog.Error("transfer download blob error", "repoName", sourceRepoName, "blob", blob.Digest.String(), "err", err)
				errChan <- err
			}
		}(blobIndex, blob)
	}

	wg.Wait()
	close(errChan)

	// 检查错误
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (l Transfer) transferSourceClients(sourceRepo client.Client, transferInfo TransferInfo) []TransferSourceClient {
	sourceRepos := []TransferSourceClient{
		{
			serverUrl: transferInfo.SourceRegistryServerUrl,
			client:    sourceRepo,
		},
	}
	if len(transferInfo.CacheSetting.RegistrySources) <= 1 {
		slog.Info("transfer source check skipped", "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "source", transferInfo.SourceRegistryServerUrl, "reason", "single source")
		return sourceRepos
	}

	seen := map[string]bool{
		transferInfo.SourceRegistryServerUrl: true,
	}
	candidates := make([]RegistrySource, 0, len(transferInfo.CacheSetting.RegistrySources)-1)

	for _, registry := range transferInfo.CacheSetting.RegistrySources {
		if registry.ServerUrl == "" || seen[registry.ServerUrl] {
			continue
		}
		seen[registry.ServerUrl] = true
		candidates = append(candidates, registry)
	}
	if len(candidates) == 0 {
		slog.Info("transfer source check skipped", "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "source", transferInfo.SourceRegistryServerUrl, "reason", "no candidates")
		return sourceRepos
	}
	slog.Info("transfer source check begin", "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "source", transferInfo.SourceRegistryServerUrl, "candidate_count", len(candidates), "timeout", TransferSourceCheckTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), TransferSourceCheckTimeout)
	defer cancel()

	doneChan := make(chan struct{}, len(candidates))
	available := make(map[string]client.Client)
	var mu sync.Mutex
	for _, registry := range candidates {
		go func(registry RegistrySource) {
			defer func() {
				select {
				case doneChan <- struct{}{}:
				case <-ctx.Done():
				}
			}()

			registryClient := RegistryClient{}.GetRegistryClient(transferInfo.CacheSetting.Host, registry.ServerUrl, nil)
			if registryClient == nil {
				slog.Warn("transfer source registry client is nil", "registry", registry.ServerUrl)
				return
			}
			exists, _, err := registryClient.ManifestExist(transferInfo.RepositoryName, transferInfo.Reference)
			if err != nil {
				slog.Warn("transfer source manifest check failed", "registry", registry.ServerUrl, "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "err", err)
				return
			}
			if !exists {
				slog.Info("transfer source manifest not exists", "registry", registry.ServerUrl, "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference)
				return
			}
			if ctx.Err() != nil {
				return
			}

			slog.Info("transfer source manifest exists", "registry", registry.ServerUrl, "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference)
			mu.Lock()
			available[registry.ServerUrl] = registryClient
			mu.Unlock()
		}(registry)
	}

	for i := 0; i < len(candidates); i++ {
		select {
		case <-doneChan:
		case <-ctx.Done():
			slog.Info("transfer source manifest check timeout", "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "timeout", TransferSourceCheckTimeout)
			i = len(candidates)
		}
	}

	mu.Lock()
	for _, registry := range candidates {
		registryClient, ok := available[registry.ServerUrl]
		if ok {
			sourceRepos = append(sourceRepos, TransferSourceClient{
				serverUrl: registry.ServerUrl,
				client:    registryClient,
			})
		}
	}
	mu.Unlock()
	slog.Info("transfer source check complete", "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "source_count", len(sourceRepos), "candidate_count", len(candidates))

	return sourceRepos
}

func (l Transfer) Loop() {
	go func() {
		for {
			select {
			case transferInfo := <-transferChan:
				err := transferPool.Invoke(transferInfo)
				if err != nil {
					slog.Error("transferPool Invoke", "err", err, "transferInfo", transferInfo)
					return
				}
			}
		}
	}()
}
