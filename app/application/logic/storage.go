package logic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gitee.com/we7coreteam/w7-registry-cache/common/helper"
	"github.com/docker/distribution"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

const (
	DownloadChunkSize     = 1 << 20 // 分片大小 2MB
	DownloadMaxRetry      = 3       // 单个分片最大重试次数
	DownloadRetryInterval = 3 * time.Second
)

type Storage struct {
	logic
}

func (l Storage) GetRepositoryModifyTime(groupHost string, repository, reference string) (time.Time, error) {
	cacheDir := facade.GetConfig().GetString("setting.registry_cache_dir")
	repositoryBlobCachePath := filepath.Join(cacheDir, groupHost, repository, reference)
	helper.CreateDirIfNotExist(filepath.Dir(repositoryBlobCachePath), os.ModePerm)

	if helper.FileExists(repositoryBlobCachePath) {
		fileInfo, err := os.Stat(repositoryBlobCachePath)
		if err != nil {
			return time.Time{}, err
		}

		return fileInfo.ModTime(), nil
	}

	return time.Time{}, nil
}

func (l Storage) UpdateRepositoryFile(groupHost string, repository, reference string) error {
	cacheDir := facade.GetConfig().GetString("setting.registry_cache_dir")
	repositoryBlobCachePath := filepath.Join(cacheDir, groupHost, repository, reference)
	helper.CreateDirIfNotExist(filepath.Dir(repositoryBlobCachePath), os.ModePerm)

	return os.WriteFile(repositoryBlobCachePath, []byte(time.Now().String()), os.ModePerm)
}

func (l Storage) RepositoryForceReCache(groupHost string, repository string) {
	cacheDir := facade.GetConfig().GetString("setting.registry_cache_dir")
	repositoryBlobCachePath := filepath.Join(cacheDir, groupHost, "__re_cache__", repository)
	helper.CreateDirIfNotExist(filepath.Dir(repositoryBlobCachePath), os.ModePerm)

	_ = os.WriteFile(repositoryBlobCachePath, []byte(time.Now().String()), os.ModePerm)
}

func (l Storage) RepositoryIsForceReCache(groupHost string, repository string) bool {
	cacheDir := facade.GetConfig().GetString("setting.registry_cache_dir")
	repositoryBlobCachePath := filepath.Join(cacheDir, groupHost, "__re_cache__", repository)
	helper.CreateDirIfNotExist(filepath.Dir(repositoryBlobCachePath), os.ModePerm)

	return helper.FileExists(repositoryBlobCachePath)
}

func (l Storage) ResetRepositoryIsForceReCache(groupHost string, repository string) {
	cacheDir := facade.GetConfig().GetString("setting.registry_cache_dir")
	repositoryBlobCachePath := filepath.Join(cacheDir, groupHost, "__re_cache__", repository)
	helper.CreateDirIfNotExist(filepath.Dir(repositoryBlobCachePath), os.ModePerm)

	os.Remove(repositoryBlobCachePath)
}

func (l Storage) DownloadBlob(ctx context.Context, pullChunkFunc func(repository, digest string, blobSize, start, end int64) (size int64, blob io.ReadCloser, err error), repositoryName string, repositoryDigest string, blobSize int64, targetWriter io.Writer, useInHttp bool) error {
	if blobSize <= 0 {
		return nil
	}

	startOffset := int64(0)
	chunkSize := int64(DownloadChunkSize)

	for startOffset < blobSize {
		if helper.CtxDone(ctx) {
			slog.Info("pull interrupted", "repositoryName", repositoryName, "repositoryDigest", repositoryDigest)
			return errors.New("download interrupted")
		}

		endOffset := startOffset + chunkSize - 1
		if endOffset >= blobSize {
			endOffset = blobSize - 1
		}

		var reader io.ReadCloser
		var size int64
		var err error

		// 重试循环
		for attempt := 0; attempt < DownloadMaxRetry; attempt++ {
			if helper.CtxDone(ctx) {
				return errors.New("download interrupted")
			}

			size, reader, err = pullChunkFunc(repositoryName, repositoryDigest, 0, startOffset, endOffset)
			if err == nil || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}

			if attempt < DownloadMaxRetry-1 {
				slog.Info("Download Retrying chunk",
					"repositoryName", repositoryName,
					"repositoryDigest", repositoryDigest,
					"start", startOffset,
					"end", endOffset,
					"attempt", attempt+1,
					"err", err)

				time.Sleep(DownloadRetryInterval * time.Duration(attempt+1))
			}
		}

		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("failed to download chunk after %d attempts: %w", DownloadMaxRetry, err)
		}

		if size == 0 && err == nil {
			// 如果 startOffset 还没到末尾，但大小为 0，说明可能已经结束了或者出错了
			if startOffset < blobSize {
				return errors.New("download stalled: received 0 bytes but more data expected")
			}
			break
		}

		if reader == nil {
			return errors.New("download failed: reader is nil")
		}

		writeLen := int64(0)
		if !useInHttp {
			content, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				return err
			}
			tlen, err := targetWriter.Write(content)
			if err != nil {
				return err
			}
			writeLen = int64(tlen)
		} else {
			writeLen, err = io.Copy(targetWriter, reader)
			if err != nil {
				return err
			}
		}

		if size != writeLen {
			return fmt.Errorf("chunk size mismatch: expected %d, wrote %d (range: %d-%d)", size, writeLen, startOffset, endOffset)
		}

		startOffset += writeLen
	}

	return nil
}
func (l Storage) DownloadManifest(ctx context.Context, pullFunc func(repository, reference string, accepttedMediaTypes ...string) (manifest distribution.Manifest, digest string, err error), repositoryName, repositoryReference string) (distribution.Manifest, error) {
	var err error
	var manifest distribution.Manifest
	for attempt := 0; attempt < DownloadMaxRetry; attempt++ {
		if helper.CtxDone(ctx) {
			slog.Info("pull interrupted", "repositoryName", repositoryName, "repositoryReference", repositoryReference)
			return nil, errors.New("download interrupted")
		}

		manifest, _, err = pullFunc(repositoryName, repositoryReference)
		if err == nil {
			break
		} else if attempt < DownloadMaxRetry-1 {
			slog.Info("Download Retrying chunk", "path", "repositoryName", repositoryName, "repositoryReference", repositoryReference, "attempt", attempt, "err", err)

			time.Sleep(DownloadRetryInterval * time.Duration(attempt+1))
		}
	}

	if manifest == nil {
		return nil, err
	}

	return manifest, err
}
