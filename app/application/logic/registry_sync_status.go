package logic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

const (
	RegistrySyncStatusLifecycleName = "registry-sync-status"
	defaultSyncStatusNamespace      = "w7-sync"
	syncStatusArtifactType          = "application/vnd.w7.registry-sync-status.v1+json"
)

// RegistrySyncStatusLocation returns the repository and tag used by the
// optional registry status artifact. A consumer on the cache side can use the
// normal Registry API to read this tag while a transfer is in progress.
func RegistrySyncStatusLocation(namespace, targetRepository, reference string) (string, string) {
	namespace = strings.Trim(namespace, "/")
	if namespace == "" {
		namespace = defaultSyncStatusNamespace
	}
	sum := sha256.Sum256([]byte(targetRepository + "\x00" + reference))
	return namespace + "/" + targetRepository, "sync-" + hex.EncodeToString(sum[:])
}

type RegistrySyncStatusOptions struct {
	Namespace        string
	DeleteOnComplete bool
}

type RegistrySyncStatusLifecycle struct {
	options RegistrySyncStatusOptions
	mu      sync.Mutex
	tasks   map[string]*registrySyncStatusTask
}

type registrySyncStatusTask struct {
	mu                     sync.Mutex
	status                 RegistrySyncStatus
	lastStatusManifestDgst string
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

func NewRegistrySyncStatusLifecycle(options RegistrySyncStatusOptions) *RegistrySyncStatusLifecycle {
	options.Namespace = strings.Trim(options.Namespace, "/")
	if options.Namespace == "" {
		options.Namespace = defaultSyncStatusNamespace
	}
	return &RegistrySyncStatusLifecycle{options: options, tasks: make(map[string]*registrySyncStatusTask)}
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
	task := &registrySyncStatusTask{status: RegistrySyncStatus{
		Repository:           event.TargetRepository,
		Reference:            event.Reference,
		SourceManifestDigest: event.SourceManifestDigest,
		Status:               "syncing",
		ExpectedBlobs:        make([]RegistrySyncStatusBlob, 0, len(event.Blobs)),
		CompletedBlobs:       make([]string, 0, len(event.Blobs)),
		FailedBlobs:          make(map[string]string),
	}}
	for _, blob := range event.Blobs {
		task.status.ExpectedBlobs = append(task.status.ExpectedBlobs, RegistrySyncStatusBlob{
			Digest: blob.Digest.String(), Size: blob.Size, MediaType: blob.MediaType,
		})
	}

	l.mu.Lock()
	l.tasks[l.taskKey(event)] = task
	l.mu.Unlock()
	return l.publish(event, task)
}

func (l *RegistrySyncStatusLifecycle) BlobCompleted(_ context.Context, event TransferLifecycleEvent, blob distribution.Descriptor, _ bool) error {
	task := l.task(event)
	if task == nil {
		return errors.New("sync status task not found")
	}
	task.mu.Lock()
	defer task.mu.Unlock()

	digestString := blob.Digest.String()
	if !containsString(task.status.CompletedBlobs, digestString) {
		task.status.CompletedBlobs = append(task.status.CompletedBlobs, digestString)
		sort.Strings(task.status.CompletedBlobs)
	}
	delete(task.status.FailedBlobs, digestString)
	return l.publishLocked(event, task)
}

func (l *RegistrySyncStatusLifecycle) BlobFailed(_ context.Context, event TransferLifecycleEvent, blob distribution.Descriptor, transferErr error) error {
	task := l.task(event)
	if task == nil {
		return errors.New("sync status task not found")
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	task.status.FailedBlobs[blob.Digest.String()] = transferErr.Error()
	return l.publishLocked(event, task)
}

func (l *RegistrySyncStatusLifecycle) Completed(_ context.Context, event TransferLifecycleEvent) error {
	task := l.removeTask(event)
	if task == nil {
		return nil
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	task.status.Status = "completed"
	task.status.Error = ""
	if err := l.publishLocked(event, task); err != nil {
		return err
	}
	if !l.options.DeleteOnComplete {
		return nil
	}
	statusRepository, _ := RegistrySyncStatusLocation(l.options.Namespace, event.TargetRepository, event.Reference)
	return event.TargetClient.DeleteManifest(statusRepository, task.lastStatusManifestDgst)
}

func (l *RegistrySyncStatusLifecycle) Failed(_ context.Context, event TransferLifecycleEvent, transferErr error) error {
	task := l.removeTask(event)
	if task == nil {
		return nil
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	task.status.Status = "failed"
	task.status.Error = transferErr.Error()
	return l.publishLocked(event, task)
}

func (l *RegistrySyncStatusLifecycle) publish(event TransferLifecycleEvent, task *registrySyncStatusTask) error {
	task.mu.Lock()
	defer task.mu.Unlock()
	return l.publishLocked(event, task)
}

func (l *RegistrySyncStatusLifecycle) publishLocked(event TransferLifecycleEvent, task *registrySyncStatusTask) error {
	task.status.UpdatedAt = time.Now().UTC()
	statusPayload, err := json.Marshal(task.status)
	if err != nil {
		return err
	}
	statusDigest := digest.FromBytes(statusPayload)
	statusRepository, statusTag := RegistrySyncStatusLocation(l.options.Namespace, event.TargetRepository, event.Reference)
	if err := event.TargetClient.PushBlob(statusRepository, statusDigest.String(), int64(len(statusPayload)), bytes.NewReader(statusPayload)); err != nil {
		return fmt.Errorf("push sync status blob: %w", err)
	}

	manifest := v1.Manifest{
		Versioned:    specs.Versioned{SchemaVersion: 2},
		MediaType:    v1.MediaTypeImageManifest,
		ArtifactType: syncStatusArtifactType,
		Config: v1.Descriptor{
			// Use the standard OCI config media type for compatibility with
			// registries that validate descriptor media types strictly.
			MediaType: v1.MediaTypeImageConfig,
			Digest:    statusDigest,
			Size:      int64(len(statusPayload)),
		},
		Layers: []v1.Descriptor{},
	}
	manifestPayload, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	manifestDigest, err := event.TargetClient.PushManifest(statusRepository, statusTag, v1.MediaTypeImageManifest, manifestPayload)
	if err != nil {
		return fmt.Errorf("push sync status manifest: %w", err)
	}
	if manifestDigest == "" {
		manifestDigest = digest.FromBytes(manifestPayload).String()
	}

	oldManifestDigest := task.lastStatusManifestDgst
	task.lastStatusManifestDgst = manifestDigest
	if oldManifestDigest != "" && oldManifestDigest != manifestDigest {
		_ = event.TargetClient.DeleteManifest(statusRepository, oldManifestDigest)
	}
	return nil
}

func (l *RegistrySyncStatusLifecycle) task(event TransferLifecycleEvent) *registrySyncStatusTask {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.tasks[l.taskKey(event)]
}

func (l *RegistrySyncStatusLifecycle) removeTask(event TransferLifecycleEvent) *registrySyncStatusTask {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := l.taskKey(event)
	task := l.tasks[key]
	delete(l.tasks, key)
	return task
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
