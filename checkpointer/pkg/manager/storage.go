package manager

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"checkpoint-in-k8s/pkg/config"
	"encoding/json"
	"fmt"
	"github.com/peterbourgon/diskv/v3"
)

// CheckpointEntry represent the result of a container checkpointing request.
type CheckpointEntry struct {
	// ContainerIdentifier represents the container that was checkpointed.
	ContainerIdentifier checkpoint.ContainerIdentifier `json:"containerIdentifier"`

	// BeginTimestamp is a Unix timestamp representing the time checkpointing was initiated.
	BeginTimestamp int64 `json:"beginTimestamp"`

	// EndTimestamp is a Unix timestamp representing the time checkpointing was finished.
	EndTimestamp int64 `json:"endTimestamp"`

	// ContainerImageName represents the container image that is pushed to a remote container registry.
	ContainerImageName string `json:"containerImageName"`

	// Error is the error that might have occurred during checkpointing.
	Error error `json:"error,omitempty"`
}

// CheckpointStorage is responsible for storing CheckpointEntry instances.
type CheckpointStorage interface {
	// StoreEntry stores CheckpointEntry under the given checkpointIdentifier key.
	// Returns error on fail or nil otherwise.
	StoreEntry(checkpointIdentifier string, entry CheckpointEntry) error

	// ReadEntry reads CheckpointEntry stored under checkpointIdentifier key.
	// Returns error on fail. If there is CheckpointEntry stored under given key, returns pointer to a CheckpointEntry
	// instance, otherwise returns nil pointer.
	ReadEntry(checkpointIdentifier string) (*CheckpointEntry, error)
}

// checkpointDiskStorage stores instances of CheckpointEntry as files on the file system using storageBackend.
// As storageBackend backend only support storage of bytes, the CheckpointEntry instances are unmarshalled/marshalled
// for read/write.
type checkpointDiskStorage struct {
	storageBackend *diskv.Diskv
}

func NewCheckpointStorage(config config.GlobalConfig) CheckpointStorage {
	storageBackend := diskv.New(diskv.Options{
		BasePath:     config.StorageBasePath,
		CacheSizeMax: 1024 * 1024,
	})
	return &checkpointDiskStorage{storageBackend}
}

func (cs *checkpointDiskStorage) StoreEntry(checkpointIdentifier string, entry CheckpointEntry) error {
	marshalled, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint entry: %w", err)
	}
	if err := cs.storageBackend.Write(checkpointIdentifier, marshalled); err != nil {
		return fmt.Errorf("failed to write checkpoint entry: %w", err)
	}
	return nil
}

func (cs *checkpointDiskStorage) ReadEntry(checkpointIdentifier string) (*CheckpointEntry, error) {
	if !cs.storageBackend.Has(checkpointIdentifier) {
		return nil, nil
	}
	marshalled, err := cs.storageBackend.Read(checkpointIdentifier)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint entry: %w", err)
	}
	entry := &CheckpointEntry{}
	if err := json.Unmarshal(marshalled, entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint entry: %w", err)
	}
	return entry, nil
}
