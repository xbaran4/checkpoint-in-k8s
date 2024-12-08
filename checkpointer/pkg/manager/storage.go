package manager

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"checkpoint-in-k8s/pkg/config"
	"encoding/json"
	"fmt"
	"github.com/peterbourgon/diskv/v3"
)

type CheckpointEntry struct {
	ContainerIdentifier checkpoint.ContainerIdentifier `json:"containerIdentifier"`
	BeginTimestamp      int64                          `json:"beginTimestamp"`
	EndTimestamp        int64                          `json:"endTimestamp"`
	ContainerImageName  string                         `json:"containerImageName"`
	Error               error                          `json:"error,omitempty"`
}

type CheckpointStorage interface {
	StoreEntry(checkpointIdentifier string, entry CheckpointEntry) error
	ReadEntry(checkpointIdentifier string) (*CheckpointEntry, error)
}

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
	return cs.storageBackend.Write(checkpointIdentifier, marshalled)
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
