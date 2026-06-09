package logic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

type childManifestRef struct {
	digest   string
	manifest distribution.Manifest
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
	childManifests, blobs, err := l.collectManifestReferences(sourceRepo, repoName, tag, sourceManifest)
	if err != nil {
		return err
	}

	targetRepoName := repoName
	targetRepoName = l.RebuildImageName(targetRepoName, transferInfo.CacheSetting.CacheRegistry.CacheNamespacePrefix)

	slog.Info("transfer blobs", "repoName", repoName, "targetRepoName", targetRepoName, "tag", tag, "blobs", blobs)
	sourceRepos := l.transferSourceClients(sourceRepo, transferInfo)
	if err := l.copyBlobs(sourceRepos, targetRepo, repoName, targetRepoName, blobs, transferInfo); err != nil {
		return fmt.Errorf("同步blobs失败: %v", err)
	}

	for _, item := range childManifests {
		mediaType, payload, err := item.manifest.Payload()
		if err != nil {
			return err
		}
		_, err = targetRepo.PushManifest(targetRepoName, item.digest, mediaType, payload)
		slog.Info("transfer push platform manifest", "repoName", targetRepoName, "tag", tag, "digest", item.digest, "err", err)
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

func (l Transfer) collectManifestReferences(sourceRepo client.Client, repoName, tag string, manifest distribution.Manifest) ([]childManifestRef, []distribution.Descriptor, error) {
	seenManifest := make(map[string]bool)
	seenBlob := make(map[string]bool)

	var collect func(distribution.Manifest) ([]childManifestRef, []distribution.Descriptor, error)
	collect = func(manifest distribution.Manifest) ([]childManifestRef, []distribution.Descriptor, error) {
		childManifests := make([]childManifestRef, 0)
		blobs := make([]distribution.Descriptor, 0)

		for _, item := range manifest.References() {
			if !l.isManifestDescriptor(item.MediaType) {
				if !l.isForeignLayer(item.MediaType) {
					digest := item.Digest.String()
					if !seenBlob[digest] {
						seenBlob[digest] = true
						blobs = append(blobs, item)
					}
				} else {
					slog.Warn("skip foreign layer", "digest", item.Digest.String(), "mediaType", item.MediaType)
				}
				continue
			}

			digest := item.Digest.String()
			if seenManifest[digest] {
				continue
			}
			seenManifest[digest] = true

			cmanifest, err := Storage{}.DownloadManifest(context.Background(), sourceRepo.PullManifest, repoName, digest)
			if err != nil {
				slog.Error("transfer pull child manifest error", "repoName", repoName, "tag", tag, "digest", digest, "mediaType", item.MediaType, "err", err)
				return nil, nil, err
			}

			nestedChildManifests, nestedBlobs, err := collect(cmanifest)
			if err != nil {
				return nil, nil, err
			}
			childManifests = append(childManifests, nestedChildManifests...)
			childManifests = append(childManifests, childManifestRef{
				digest:   digest,
				manifest: cmanifest,
			})
			blobs = append(blobs, nestedBlobs...)
		}

		return childManifests, blobs, nil
	}

	return collect(manifest)
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

			if l.targetBlobReadable(targetRepo, targetRepoName, blob) {
				slog.Info("transfer blob skipped with readable cache", "repoName", sourceRepoName, "targetRepoName", targetRepoName, "blob", blob.Digest.String(), "blobSize", blob.Size)
				return
			}

			err := l.copyBlobWithFallback(sourceRepos, targetRepo, sourceRepoName, targetRepoName, blob, blobIndex, transferInfo)
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

func (l Transfer) targetBlobReadable(targetRepo client.Client, targetRepoName string, blob distribution.Descriptor) bool {
	exists, size, digest, err := targetRepo.BlobExist(targetRepoName, blob.Digest.String())
	if err != nil {
		slog.Warn("transfer target blob exists check failed", "repoName", targetRepoName, "blob", blob.Digest.String(), "err", err)
		return false
	}
	if !exists {
		return false
	}
	if digest != "" && digest != blob.Digest.String() {
		slog.Warn("transfer target blob digest mismatch", "repoName", targetRepoName, "blob", blob.Digest.String(), "targetDigest", digest)
		return false
	}
	if blob.Size > 0 && size != blob.Size {
		slog.Warn("transfer target blob size mismatch", "repoName", targetRepoName, "blob", blob.Digest.String(), "blobSize", blob.Size, "targetSize", size)
		return false
	}
	if blob.Size <= 0 {
		slog.Warn("transfer target blob size unknown, skip readable cache shortcut", "repoName", targetRepoName, "blob", blob.Digest.String(), "blobSize", blob.Size, "targetSize", size)
		return false
	}

	if l.targetBlobRangeReadable(targetRepo, targetRepoName, blob, 0, 0) {
		if blob.Size == 1 || l.targetBlobRangeReadable(targetRepo, targetRepoName, blob, blob.Size-1, blob.Size-1) {
			return true
		}
	}

	slog.Warn("transfer target blob range check failed", "repoName", targetRepoName, "blob", blob.Digest.String(), "blobSize", blob.Size)
	return false
}

func (l Transfer) targetBlobRangeReadable(targetRepo client.Client, targetRepoName string, blob distribution.Descriptor, start, end int64) bool {
	size, reader, err := targetRepo.PullBlobChunk(targetRepoName, blob.Digest.String(), blob.Size, start, end)
	if err != nil {
		slog.Warn("transfer target blob range pull failed", "repoName", targetRepoName, "blob", blob.Digest.String(), "start", start, "end", end, "err", err)
		return false
	}
	if reader == nil {
		return false
	}
	defer reader.Close()

	readLen, err := io.Copy(io.Discard, reader)
	if err != nil {
		slog.Warn("transfer target blob range read failed", "repoName", targetRepoName, "blob", blob.Digest.String(), "start", start, "end", end, "err", err)
		return false
	}
	expectedSize := end - start + 1
	if size != expectedSize || readLen != expectedSize {
		slog.Warn("transfer target blob range size mismatch", "repoName", targetRepoName, "blob", blob.Digest.String(), "start", start, "end", end, "size", size, "readLen", readLen, "expectedSize", expectedSize)
		return false
	}

	return true
}

func (l Transfer) copyBlobWithFallback(sourceRepos []TransferSourceClient, targetRepo client.Client, sourceRepoName string, targetRepoName string, blob distribution.Descriptor, blobIndex int, transferInfo TransferInfo) error {
	if len(sourceRepos) == 0 {
		return errors.New("source registry clients are empty")
	}

	var errs []error
	for attempt := 0; attempt < len(sourceRepos); attempt++ {
		sourceRepo := sourceRepos[(blobIndex+int(transferInfo.CurReTryNum)+attempt)%len(sourceRepos)]
		syncWriter := &SyncWriter{
			RegistryClient:   targetRepo,
			RepositoryName:   targetRepoName,
			RepositoryDigest: blob.Digest.String(),
		}

		slog.Info("transfer blob source selected",
			"repoName", sourceRepoName,
			"targetRepoName", targetRepoName,
			"blob", blob.Digest.String(),
			"blobSize", blob.Size,
			"source", sourceRepo.serverUrl,
			"attempt", attempt+1,
			"source_count", len(sourceRepos),
		)

		err := Storage{}.DownloadBlob(context.Background(), sourceRepo.client.PullBlobChunk, sourceRepoName, blob.Digest.String(), blob.Size, syncWriter, false)
		if err == nil {
			err = syncWriter.End()
		}
		if err == nil {
			return nil
		}

		slog.Warn("transfer blob source failed",
			"repoName", sourceRepoName,
			"targetRepoName", targetRepoName,
			"blob", blob.Digest.String(),
			"source", sourceRepo.serverUrl,
			"attempt", attempt+1,
			"err", err,
		)
		errs = append(errs, fmt.Errorf("%s: %w", sourceRepo.serverUrl, err))
	}

	return fmt.Errorf("all source registry blob downloads failed: %w", errors.Join(errs...))
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

	repositoryCacheRule, _ := CacheRule{}.MatchRepositoryCacheRule(transferInfo.RepositoryName, transferInfo.CacheSetting.RepositoryCacheRules)
	seen := map[string]bool{
		transferInfo.SourceRegistryServerUrl: true,
	}
	sourceCount := len(sourceRepos)
	slog.Info("transfer source check begin", "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "source", transferInfo.SourceRegistryServerUrl, "timeout", TransferSourceCheckTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), TransferSourceCheckTimeout)
	defer cancel()

	RegistryServer{}.WalkManifestRegistrySourcesFromRule(ctx, transferInfo.CacheSetting.Host, transferInfo.RepositoryName, transferInfo.Reference, repositoryCacheRule, func(registryServerUrl string, registryClient client.Client, _ *distribution.Descriptor) bool {
		if registryServerUrl == "" || seen[registryServerUrl] {
			return false
		}
		seen[registryServerUrl] = true

		slog.Info("transfer source manifest exists", "registry", registryServerUrl, "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference)
		sourceRepos = append(sourceRepos, TransferSourceClient{
			serverUrl: registryServerUrl,
			client:    registryClient,
		})
		return false
	})
	if ctx.Err() != nil {
		slog.Info("transfer source manifest check timeout", "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "timeout", TransferSourceCheckTimeout)
	}
	slog.Info("transfer source check complete", "repoName", transferInfo.RepositoryName, "reference", transferInfo.Reference, "source_count", len(sourceRepos), "available_count", len(sourceRepos)-sourceCount)

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
