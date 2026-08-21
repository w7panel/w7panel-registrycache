package logic

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

const RegistrySyncStatusLifecycleName = "registry-sync-status"

const (
	syncStatusRetention       = 24 * time.Hour
	syncStatusPersistInterval = 10 * time.Second
	syncStatusCleanupInterval = 10 * time.Minute
)

type RegistrySyncStatusLifecycle struct {
	mu         sync.Mutex
	persistMu  sync.Mutex
	statusPath string
	tasks      map[string]*registrySyncStatusTask
	dirty      bool
	version    uint64
}

type registrySyncStatusTask struct {
	mu     sync.Mutex
	status RegistrySyncStatus
}

type RegistrySyncStatusBlob struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"media_type,omitempty"`
}

type RegistrySyncStatus struct {
	Repository           string                   `json:"repository"`
	Reference            string                   `json:"reference"`
	SourceManifestDigest string                   `json:"source_manifest_digest"`
	Status               string                   `json:"status"`
	ExpectedBlobs        []RegistrySyncStatusBlob `json:"expected_blobs"`
	CompletedBlobs       []string                 `json:"completed_blobs"`
	FailedBlobs          map[string]string        `json:"failed_blobs,omitempty"`
	Error                string                   `json:"error,omitempty"`
	UpdatedAt            time.Time                `json:"updated_at"`
}

func NewRegistrySyncStatusLifecycle() *RegistrySyncStatusLifecycle {
	lifecycle := &RegistrySyncStatusLifecycle{
		statusPath: filepath.Join(storageSettingDir(), "registry-sync-status.json"),
		tasks:      make(map[string]*registrySyncStatusTask),
	}
	if err := lifecycle.load(); err != nil {
		slog.Warn("加载镜像同步状态失败", "err", err)
	}
	if err := lifecycle.cleanupAndPersist(); err != nil {
		slog.Warn("清理镜像同步状态失败", "err", err)
	}
	go lifecycle.cleanupLoop()
	go lifecycle.persistLoop()
	return lifecycle
}

// RegisterEvents connects this optional feature to rangine's event bus.
func (l *RegistrySyncStatusLifecycle) RegisterEvents() error {
	event := facade.GetEvent()
	if event == nil {
		return errors.New("rangine event bus is not initialized")
	}
	if err := event.Subscribe(TransferLifecycleStarted, func(payload TransferLifecyclePayload) {
		logTransferLifecycleError(TransferLifecycleStarted, l.Started(transferLifecycleContext(), payload.Event))
	}); err != nil {
		return err
	}
	if err := event.Subscribe(TransferLifecycleBlobCompleted, func(payload TransferLifecyclePayload) {
		logTransferLifecycleError(TransferLifecycleBlobCompleted, l.BlobCompleted(transferLifecycleContext(), payload.Event, payload.Blob, payload.Skipped))
	}); err != nil {
		return err
	}
	if err := event.Subscribe(TransferLifecycleBlobFailed, func(payload TransferLifecyclePayload) {
		logTransferLifecycleError(TransferLifecycleBlobFailed, l.BlobFailed(transferLifecycleContext(), payload.Event, payload.Blob, payload.Err))
	}); err != nil {
		return err
	}
	if err := event.Subscribe(TransferLifecycleCompleted, func(payload TransferLifecyclePayload) {
		logTransferLifecycleError(TransferLifecycleCompleted, l.Completed(transferLifecycleContext(), payload.Event))
	}); err != nil {
		return err
	}
	return event.Subscribe(TransferLifecycleFailed, func(payload TransferLifecyclePayload) {
		logTransferLifecycleError(TransferLifecycleFailed, l.Failed(transferLifecycleContext(), payload.Event, payload.Err))
	})
}

func (l *RegistrySyncStatusLifecycle) Started(_ context.Context, event TransferLifecycleEvent) error {
	status := RegistrySyncStatus{
		Repository:           event.TargetRepository,
		Reference:            event.Reference,
		SourceManifestDigest: event.SourceManifestDigest,
		Status:               "syncing",
		ExpectedBlobs:        make([]RegistrySyncStatusBlob, 0, len(event.Blobs)),
		CompletedBlobs:       make([]string, 0, len(event.Blobs)),
		FailedBlobs:          make(map[string]string),
		UpdatedAt:            time.Now().UTC(),
	}
	for _, blob := range event.Blobs {
		status.ExpectedBlobs = append(status.ExpectedBlobs, RegistrySyncStatusBlob{
			Digest: blob.Digest.String(), Size: blob.Size, MediaType: blob.MediaType,
		})
	}

	l.mu.Lock()
	key := l.taskKey(event)
	if l.tasks == nil {
		l.tasks = make(map[string]*registrySyncStatusTask)
	}
	l.tasks[key] = &registrySyncStatusTask{status: status}
	l.dirty = true
	l.version++
	l.mu.Unlock()
	return l.persistNow()
}

func (l *RegistrySyncStatusLifecycle) BlobCompleted(_ context.Context, event TransferLifecycleEvent, blob distribution.Descriptor, _ bool) error {
	task := l.task(event)
	if task == nil {
		return errors.New("sync status task not found")
	}
	task.mu.Lock()
	digestString := blob.Digest.String()
	if !containsString(task.status.CompletedBlobs, digestString) {
		task.status.CompletedBlobs = append(task.status.CompletedBlobs, digestString)
		sort.Strings(task.status.CompletedBlobs)
	}
	delete(task.status.FailedBlobs, digestString)
	task.status.UpdatedAt = time.Now().UTC()
	task.mu.Unlock()
	l.markDirty()
	return nil
}

func (l *RegistrySyncStatusLifecycle) BlobFailed(_ context.Context, event TransferLifecycleEvent, blob distribution.Descriptor, transferErr error) error {
	task := l.task(event)
	if task == nil {
		return errors.New("sync status task not found")
	}
	task.mu.Lock()
	task.status.FailedBlobs[blob.Digest.String()] = transferErr.Error()
	task.status.UpdatedAt = time.Now().UTC()
	task.mu.Unlock()
	l.markDirty()
	return nil
}

func (l *RegistrySyncStatusLifecycle) Completed(_ context.Context, event TransferLifecycleEvent) error {
	key := l.taskKey(event)
	l.mu.Lock()
	if _, ok := l.tasks[key]; !ok {
		l.mu.Unlock()
		return nil
	}
	delete(l.tasks, key)
	l.version++
	l.dirty = true
	l.mu.Unlock()
	return l.persistNow()
}

func (l *RegistrySyncStatusLifecycle) Failed(_ context.Context, event TransferLifecycleEvent, transferErr error) error {
	task := l.task(event)
	if task == nil {
		return nil
	}
	task.mu.Lock()
	task.status.Status = "failed"
	task.status.Error = transferErr.Error()
	task.status.UpdatedAt = time.Now().UTC()
	task.mu.Unlock()
	l.markDirty()
	return l.persistNow()
}

// ListStatuses returns a snapshot of active and failed tasks for a cache host.
func (l *RegistrySyncStatusLifecycle) ListStatuses(host string) []RegistrySyncStatus {
	_ = l.cleanupAndPersist()
	l.mu.Lock()
	tasks := make([]*registrySyncStatusTask, 0, len(l.tasks))
	for key, task := range l.tasks {
		if host == "" || strings.HasPrefix(key, host+"\x00") {
			tasks = append(tasks, task)
		}
	}
	l.mu.Unlock()

	statuses := make([]RegistrySyncStatus, 0, len(tasks))
	for _, task := range tasks {
		task.mu.Lock()
		status := copySyncStatus(task.status)
		task.mu.Unlock()
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].Repository == statuses[j].Repository {
			return statuses[i].Reference < statuses[j].Reference
		}
		return statuses[i].Repository < statuses[j].Repository
	})
	return statuses
}

func (l *RegistrySyncStatusLifecycle) load() error {
	if l.tasks == nil {
		l.tasks = make(map[string]*registrySyncStatusTask)
	}
	content, err := os.ReadFile(l.statusPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var records map[string]RegistrySyncStatus
	if err := json.Unmarshal(content, &records); err != nil {
		return err
	}
	for key, status := range records {
		if status.Status != "syncing" && status.Status != "failed" {
			continue
		}
		l.tasks[key] = &registrySyncStatusTask{status: status}
	}
	return nil
}

func (l *RegistrySyncStatusLifecycle) persistNow() error {
	l.persistMu.Lock()
	defer l.persistMu.Unlock()

	l.mu.Lock()
	version := l.version
	statuses := make(map[string]RegistrySyncStatus, len(l.tasks))
	for key, task := range l.tasks {
		task.mu.Lock()
		statuses[key] = copySyncStatus(task.status)
		task.mu.Unlock()
	}
	l.mu.Unlock()

	content, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.statusPath), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(l.statusPath), ".registry-sync-status-*.sync")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, l.statusPath); err != nil {
		return err
	}
	l.mu.Lock()
	if l.version == version {
		l.dirty = false
	}
	l.mu.Unlock()
	return nil
}

func (l *RegistrySyncStatusLifecycle) markDirty() {
	l.mu.Lock()
	l.dirty = true
	l.version++
	l.mu.Unlock()
}

func copySyncStatus(status RegistrySyncStatus) RegistrySyncStatus {
	status.ExpectedBlobs = append([]RegistrySyncStatusBlob(nil), status.ExpectedBlobs...)
	status.CompletedBlobs = append([]string(nil), status.CompletedBlobs...)
	status.FailedBlobs = make(map[string]string, len(status.FailedBlobs))
	for digest, message := range status.FailedBlobs {
		status.FailedBlobs[digest] = message
	}
	return status
}

func (l *RegistrySyncStatusLifecycle) cleanupAndPersist() error {
	now := time.Now().UTC()
	changed := l.cleanup(now)
	if !changed {
		return nil
	}
	l.markDirty()
	return l.persistNow()
}
func (l *RegistrySyncStatusLifecycle) cleanup(now time.Time) bool {
	changed := false
	l.mu.Lock()
	for key, task := range l.tasks {
		task.mu.Lock()
		status := task.status
		task.mu.Unlock()
		if status.UpdatedAt.IsZero() || now.Sub(status.UpdatedAt) < syncStatusRetention {
			continue
		}
		if current, ok := l.tasks[key]; ok && current == task {
			delete(l.tasks, key)
			l.version++
			changed = true
		}
	}
	l.mu.Unlock()
	return changed
}

func (l *RegistrySyncStatusLifecycle) cleanupLoop() {
	ticker := time.NewTicker(syncStatusCleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := l.cleanupAndPersist(); err != nil {
			slog.Warn("定时清理镜像同步状态失败", "err", err)
		}
	}
}

func (l *RegistrySyncStatusLifecycle) persistLoop() {
	ticker := time.NewTicker(syncStatusPersistInterval)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		dirty := l.dirty
		l.mu.Unlock()
		if !dirty {
			continue
		}
		if err := l.persistNow(); err != nil {
			slog.Warn("定时保存镜像同步状态失败", "err", err)
		}
	}
}

func (l *RegistrySyncStatusLifecycle) task(event TransferLifecycleEvent) *registrySyncStatusTask {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.tasks[l.taskKey(event)]
}

func (l *RegistrySyncStatusLifecycle) taskKey(event TransferLifecycleEvent) string {
	return event.TransferInfo.CacheSetting.Host + "\x00" + event.TargetRepository + "\x00" + event.Reference
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
