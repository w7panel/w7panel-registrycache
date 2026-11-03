package logic

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/we7coreteam/w7-registry-cache/common/helper"

	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

const cacheExpirationRetry = time.Minute

// The key is the physical cache location, not the host alias. Re-caching through
// another alias replaces the task rather than leaving an old deletion behind.
type CacheExpirationTask struct {
	Host             string    `json:"host"`
	RegistryURL      string    `json:"registry_url"`
	Repository       string    `json:"repository"`
	SourceRepository string    `json:"source_repository"`
	Reference        string    `json:"reference"`
	Digest           string    `json:"digest"`
	CachedAt         time.Time `json:"cached_at"`
	ExpireAt         time.Time `json:"expire_at"`
	RetryAt          time.Time `json:"retry_at,omitempty"`
}

func (t CacheExpirationTask) key() string {
	return t.RegistryURL + "\x00" + t.Repository + "\x00" + t.Reference
}

func (t CacheExpirationTask) deadline() time.Time {
	if t.RetryAt.After(t.ExpireAt) {
		return t.RetryAt
	}
	return t.ExpireAt
}

// CacheExpirationQueue restores registered tasks on restart, sleeps until the
// next deadline, and bounds network cleanup concurrency to four workers.
type CacheExpirationQueue struct {
	mu      sync.Mutex
	store   CacheExpirationPersistence
	tasks   map[string]CacheExpirationTask
	heap    *helper.IndexedMinHeap[string, CacheExpirationTask]
	wake    chan struct{}
	cleanup func(context.Context, CacheExpirationTask, []CacheExpirationTask) error
}

var defaultExpirationQueue atomic.Pointer[CacheExpirationQueue]
var cacheExpirationStartMu sync.Mutex

func StartCacheExpirationWorker() error {
	cacheExpirationStartMu.Lock()
	defer cacheExpirationStartMu.Unlock()
	if defaultExpirationQueue.Load() != nil {
		return nil
	}
	store, err := NewCacheExpirationStore(facade.GetConfig().GetString("cache.cleanup.queue_dir"))
	if err != nil {
		return err
	}
	queue, err := NewCacheExpirationQueue(store)
	if err != nil {
		_ = store.Close()
		return err
	}
	if !defaultExpirationQueue.CompareAndSwap(nil, queue) {
		return store.Close()
	}
	go queue.Run(context.Background())
	return nil
}

func NewCacheExpirationQueue(store CacheExpirationPersistence) (*CacheExpirationQueue, error) {
	tasks, err := store.Snapshot()
	if err != nil {
		return nil, err
	}
	queue := &CacheExpirationQueue{
		heap:    helper.NewIndexedMinHeap[string, CacheExpirationTask](func(a, b CacheExpirationTask) bool { return a.deadline().Before(b.deadline()) }),
		store:   store,
		tasks:   map[string]CacheExpirationTask{},
		wake:    make(chan struct{}, 1),
		cleanup: cleanupExpiredManifest,
	}
	for _, task := range tasks {
		if task.ExpireAt.IsZero() {
			continue
		}
		queue.tasks[task.key()] = task
		queue.scheduleLocked(task)
	}
	return queue, nil
}

func (q *CacheExpirationQueue) Enqueue(task CacheExpirationTask) error {
	task.CachedAt = task.CachedAt.UTC()
	task.ExpireAt = task.ExpireAt.UTC()
	task.RetryAt = task.RetryAt.UTC()
	q.mu.Lock()
	defer q.mu.Unlock()
	if task.ExpireAt.IsZero() {
		if err := q.store.Delete(task); err != nil {
			return err
		}
		delete(q.tasks, task.key())
	} else {
		if err := q.store.Save(task); err != nil {
			return err
		}
		q.tasks[task.key()] = task
	}
	q.scheduleLocked(task)
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return nil
}

// Refresh the existing heap entry instead of accumulating stale entries for
// frequently refreshed images with long TTLs. Caller holds q.mu.
func (q *CacheExpirationQueue) scheduleLocked(task CacheExpirationTask) {
	if task.ExpireAt.IsZero() {
		q.heap.Remove(task.key())
	} else {
		q.heap.Upsert(task.key(), task)
	}
}

func (q *CacheExpirationQueue) Run(ctx context.Context) {
	jobs := make(chan CacheExpirationTask)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for task := range jobs {
				q.process(ctx, task)
			}
		}()
	}
	defer func() { close(jobs); workers.Wait() }()
	for {
		q.mu.Lock()
		var task CacheExpirationTask
		wait := time.Duration(0)
		if next, ok := q.heap.Peek(); ok {
			task = next
			wait = time.Until(task.deadline())
			if wait <= 0 {
				q.heap.Pop()
			}
		}
		q.mu.Unlock()
		if !task.CachedAt.IsZero() && wait <= 0 {
			select {
			case jobs <- task:
			case <-ctx.Done():
				return
			}
			continue
		}
		if task.CachedAt.IsZero() {
			select {
			case <-ctx.Done():
				return
			case <-q.wake:
			}
			continue
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-q.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (q *CacheExpirationQueue) process(ctx context.Context, task CacheExpirationTask) {
	unlock := lockCacheRepository(task.RegistryURL, task.Repository)
	defer unlock()
	q.mu.Lock()
	if q.tasks[task.key()] != task {
		q.mu.Unlock()
		return
	}
	siblings := []CacheExpirationTask{}
	for _, other := range q.tasks {
		if other.RegistryURL == task.RegistryURL && other.Repository == task.Repository {
			siblings = append(siblings, other)
		}
	}
	q.mu.Unlock()
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err := q.cleanup(requestCtx, task, siblings)
	cancel()
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.tasks[task.key()] != task {
		return
	}
	if err == nil {
		if err = q.store.Delete(task); err == nil {
			delete(q.tasks, task.key())
			q.heap.Remove(task.key())
			slog.Info("cache expiration cleanup succeeded", "host", task.Host, "repository", task.Repository, "reference", task.Reference)
			return
		}
	}
	// Retry remains durable across restarts. Do not discard failed work.
	task.RetryAt = time.Now().Add(cacheExpirationRetry).UTC()
	if saveErr := q.store.Save(task); saveErr != nil {
		slog.Error("persist cache expiration retry", "err", saveErr)
	}
	q.tasks[task.key()] = task
	q.scheduleLocked(task)
	if errors.Is(err, errManifestStillReferenced) || errors.Is(err, errCacheNotExpired) {
		slog.Info("cache expiration deferred", "repository", task.Repository, "reference", task.Reference)
	} else {
		slog.Warn("cache expiration cleanup failed; retry scheduled", "repository", task.Repository, "reference", task.Reference, "err", err)
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func enqueueTransferredCacheExpiration(info TransferInfo, digest string) error {
	cachedAt, err := (Storage{}).GetRepositoryModifyTime(info.CacheSetting.Host, info.RepositoryName, info.Reference)
	if err != nil {
		return err
	}
	repository := (Transfer{}).RebuildImageName(info.RepositoryName, info.CacheSetting.CacheRegistry.CacheNamespacePrefix)
	return EnqueueCacheExpiration(info, repository, digest, cachedAt)
}

// EnqueueCacheExpiration persists a task and wakes the expiration worker.
// Non-expiring refreshes cancel older finite-TTL tasks for the same target.
func EnqueueCacheExpiration(info TransferInfo, repository, digest string, cachedAt time.Time) error {
	queue := defaultExpirationQueue.Load()
	if queue == nil {
		return errors.New("cache expiration worker is not started")
	}
	rule, err := (CacheRule{}).MatchRepositoryCacheRule(info.RepositoryName, info.CacheSetting.RepositoryCacheRules)
	if err != nil {
		return err
	}
	task := CacheExpirationTask{
		Host:             info.CacheSetting.Host,
		RegistryURL:      strings.TrimRight(info.CacheSetting.CacheRegistry.ServerUrl, "/"),
		Repository:       repository,
		SourceRepository: info.RepositoryName,
		Reference:        info.Reference,
		Digest:           digest,
		CachedAt:         cachedAt,
	}
	if rule != nil && rule.CacheEnabled() {
		if ttl, ok := cacheTTLDuration(rule.CacheTtl); ok {
			task.ExpireAt = cachedAt.Add(ttl)
		}
	}
	return queue.Enqueue(task)
}
