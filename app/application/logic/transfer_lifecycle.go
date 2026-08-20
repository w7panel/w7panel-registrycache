package logic

import (
	"context"
	"log/slog"

	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/client"
	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
)

const (
	TransferLifecycleStarted       = "started"
	TransferLifecycleBlobCompleted = "blob_completed"
	TransferLifecycleBlobFailed    = "blob_failed"
	TransferLifecycleCompleted     = "completed"
	TransferLifecycleFailed        = "failed"
)

// TransferLifecycleEvent is passed through the rangine event bus.
type TransferLifecycleEvent struct {
	TransferInfo         TransferInfo
	TargetClient         client.Client
	SourceRepository     string
	TargetRepository     string
	Reference            string
	SourceManifestDigest string
	Blobs                []distribution.Descriptor
}

// TransferLifecyclePayload is the single payload passed through the rangine
// event bus. Optional fields are used by blob and failed events.
type TransferLifecyclePayload struct {
	Event   TransferLifecycleEvent
	Blob    distribution.Descriptor
	Skipped bool
	Err     error
}

func newTransferLifecycleEvent(transferInfo TransferInfo, targetClient client.Client, sourceRepository, targetRepository, reference string, manifestPayload []byte, blobs []distribution.Descriptor) TransferLifecycleEvent {
	return TransferLifecycleEvent{
		TransferInfo:         transferInfo,
		TargetClient:         targetClient,
		SourceRepository:     sourceRepository,
		TargetRepository:     targetRepository,
		Reference:            reference,
		SourceManifestDigest: digest.FromBytes(manifestPayload).String(),
		Blobs:                blobs,
	}
}

func logTransferLifecycleError(topic string, err error) {
	if err != nil {
		slog.Warn("transfer lifecycle failed", "event", topic, "err", err)
	}
}

// Keep context creation in one place for event subscribers.
func transferLifecycleContext() context.Context { return context.Background() }
