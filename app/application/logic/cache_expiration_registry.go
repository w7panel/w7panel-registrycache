package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitee.com/we7coreteam/w7-registry-cache/common/helper"
	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/types"
	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/util"
	"github.com/opencontainers/go-digest"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

// The lock identity is the physical cache repository, shared by cleanup and refresh.
var cacheRepositoryLocks helper.StripedMutex

func lockCacheRepository(registryURL, repository string) func() {
	return cacheRepositoryLocks.Lock(strings.TrimRight(registryURL, "/") + "\x00" + repository)
}

var errManifestStillReferenced = errors.New("manifest is still referenced by a live or unmanaged cache tag")
var errCacheNotExpired = errors.New("cache TTL was extended")

type expirationRegistryClient interface {
	Do(*http.Request) (*http.Response, error)
}

// A different backend/directory or a disabled/permanent rule cancels deletion.
// Credentials are deliberately not part of identity: credential rotation must
// still allow cleanup against the same registry.
func cacheExpirationDeadline(task CacheExpirationTask, setting *RegistryCacheSetting) (time.Time, bool, error) {
	if setting == nil || strings.TrimRight(setting.CacheRegistry.ServerUrl, "/") != task.RegistryURL || (Storage{}).RebuildImageName(task.SourceRepository, setting.CacheRegistry.CacheNamespacePrefix) != task.Repository {
		return time.Time{}, false, nil
	}
	rule, err := (CacheRule{}).MatchRepositoryCacheRule(task.SourceRepository, setting.RepositoryCacheRules)
	if err != nil {
		return time.Time{}, false, err
	}
	if rule == nil || !rule.CacheEnabled() {
		return time.Time{}, false, nil
	}
	ttl, ok := cacheTTLDuration(rule.CacheTtl)
	if !ok {
		return time.Time{}, false, nil
	}
	return task.CachedAt.Add(ttl), true, nil
}

func cleanupExpiredManifest(ctx context.Context, task CacheExpirationTask, siblings []CacheExpirationTask) error {
	setting := (Setting{}).GetStorageCacheSetting(task.Host)
	deadline, active, err := cacheExpirationDeadline(task, setting)
	if err != nil || !active {
		return err
	}
	if time.Now().Before(deadline) {
		return errCacheNotExpired
	}
	modifiedAt, err := (Storage{}).GetRepositoryModifyTime(task.Host, task.SourceRepository, task.Reference)
	if err != nil {
		return err
	}
	if modifiedAt.After(task.CachedAt) {
		return nil
	}
	registryClient := (RegistryClient{}).GetRegistryClient(task.Host, task.RegistryURL, nil)
	if registryClient == nil {
		return errors.New("cache expiration registry client is unavailable")
	}
	if err = deleteExpiredManifest(ctx, registryClient, task, siblings, func(host string) *RegistryCacheSetting { return (Setting{}).GetStorageCacheSetting(host) }, time.Now()); err != nil {
		return err
	}
	// Marker deletion is local and idempotent as well. Keep it on failure so the
	// operation retries rather than forgetting an incomplete cleanup.
	marker := filepath.Join(facade.GetConfig().GetString("setting.registry_cache_dir"), task.Host, task.SourceRepository, task.Reference)
	if err = os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func deleteExpiredManifest(ctx context.Context, client expirationRegistryClient, task CacheExpirationTask, siblings []CacheExpirationTask, settingForHost func(string) *RegistryCacheSetting, now time.Time) error {
	if err := digest.Digest(task.Digest).Validate(); err != nil {
		return fmt.Errorf("invalid cached manifest digest: %w", err)
	}
	current, _, exists, err := expirationManifestDigest(ctx, client, task, task.Reference)
	if err != nil || !exists {
		return err
	} // Registry already removed it: success.
	if current != task.Digest {
		return nil
	} // The tag was refreshed outside our queue.
	// Registry v2 deletes by digest, which can remove several tags. Never delete
	// a digest still used by a permanent, unexpired, or unmanaged tag.
	tags, err := expirationRepositoryTags(ctx, client, task)
	if types.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if tag == task.Reference {
			continue
		}
		tagDigest, mediaType, exists, err := expirationManifestDigest(ctx, client, task, tag)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		referenced := tagDigest == task.Digest
		if !referenced && expirationIsIndex(mediaType) {
			referenced, err = expirationIndexReferences(ctx, client, task, tagDigest, task.Digest, map[string]bool{})
			if err != nil {
				return err
			}
		}
		if !referenced {
			continue
		}
		expired := false
		for _, other := range siblings {
			if other.Reference != tag || other.Digest != tagDigest {
				continue
			}
			deadline, active, err := cacheExpirationDeadline(other, settingForHost(other.Host))
			if err != nil {
				return err
			}
			expired = active && !now.Before(deadline)
			break
		}
		if !expired {
			return errManifestStillReferenced
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, expirationManifestURL(task, task.Digest), nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if types.IsNotFound(err) {
		return nil
	} // External GC can race with the HEAD.
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	return expirationResponseError(response)
}

func expirationManifestURL(task CacheExpirationTask, reference string) string {
	return strings.TrimRight(task.RegistryURL, "/") + "/v2/" + task.Repository + "/manifests/" + url.PathEscape(reference)
}

func expirationManifestDigest(ctx context.Context, client expirationRegistryClient, task CacheExpirationTask, reference string) (string, string, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, expirationManifestURL(task, reference), nil)
	if err != nil {
		return "", "", false, err
	}
	for _, mediaType := range []string{"application/vnd.oci.image.index.v1+json", "application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json", "application/vnd.docker.distribution.manifest.v2+json"} {
		request.Header.Add("Accept", mediaType)
	}
	response, err := client.Do(request)
	if types.IsNotFound(err) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "", "", false, nil
	}
	if err = expirationResponseError(response); err != nil {
		return "", "", false, err
	}
	value := response.Header.Get("Docker-Content-Digest")
	if err = digest.Digest(value).Validate(); err != nil {
		return "", "", false, fmt.Errorf("registry returned invalid manifest digest: %w", err)
	}
	return value, response.Header.Get("Content-Type"), true, nil
}

func expirationIsIndex(mediaType string) bool {
	mediaType = strings.TrimSpace(strings.SplitN(mediaType, ";", 2)[0])
	return mediaType == "application/vnd.oci.image.index.v1+json" || mediaType == "application/vnd.docker.distribution.manifest.list.v2+json"
}

// A cached platform digest may also be referenced by a live multi-architecture
// tag. Protect these dependencies, including nested indexes, before deletion.
func expirationIndexReferences(ctx context.Context, client expirationRegistryClient, task CacheExpirationTask, indexDigest, targetDigest string, seen map[string]bool) (bool, error) {
	if seen[indexDigest] {
		return false, nil
	}
	if len(seen) >= 32 {
		return false, errors.New("manifest index nesting exceeds cleanup limit")
	}
	seen[indexDigest] = true
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, expirationManifestURL(task, indexDigest), nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json")
	response, err := client.Do(request)
	if types.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if err = expirationResponseError(response); err != nil {
		return false, err
	}
	var index struct {
		Manifests []struct {
			Digest    string `json:"digest"`
			MediaType string `json:"mediaType"`
		} `json:"manifests"`
	}
	if err = json.NewDecoder(response.Body).Decode(&index); err != nil {
		return false, err
	}
	for _, child := range index.Manifests {
		if child.Digest == targetDigest {
			return true, nil
		}
		if expirationIsIndex(child.MediaType) {
			referenced, err := expirationIndexReferences(ctx, client, task, child.Digest, targetDigest, seen)
			if referenced || err != nil {
				return referenced, err
			}
		}
	}
	return false, nil
}

func expirationRepositoryTags(ctx context.Context, client expirationRegistryClient, task CacheExpirationTask) ([]string, error) {
	next, err := url.Parse(strings.TrimRight(task.RegistryURL, "/") + "/v2/" + task.Repository + "/tags/list?n=1000")
	if err != nil {
		return nil, err
	}
	origin := *next
	seen := map[string]bool{}
	tags := []string{}
	for next != nil {
		if next.Host != origin.Host || next.Scheme != origin.Scheme || next.Path != origin.Path || seen[next.String()] {
			return nil, errors.New("invalid registry tag pagination link")
		}
		seen[next.String()] = true
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, next.String(), nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		if err = expirationResponseError(response); err != nil {
			response.Body.Close()
			return nil, err
		}
		var page struct {
			Tags []string `json:"tags"`
		}
		err = json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		tags = append(tags, page.Tags...)
		var following *url.URL
		for _, link := range util.ParseLinks(response.Header.Get("Link")) {
			if link.Rel == "next" {
				value, err := url.Parse(link.URL)
				if err != nil {
					return nil, err
				}
				following = next.ResolveReference(value)
				break
			}
		}
		next = following
	}
	return tags, nil
}

func expirationResponseError(response *http.Response) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	return &types.HTTPError{StatusCode: response.StatusCode, Code: types.GeneralCode}
}
