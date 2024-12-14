package manager

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"context"
	"sync"
)

// CheckpointManager is responsible for running initiating synchronous and asynchronous checkpoints.
// In case of asynchronous checkpoints, it is also responsible for storing and fetching the results.
type CheckpointManager interface {

	// Checkpoint will checkpoint container (a)synchronously based on the async parameter. If the caller requests
	// async checkpoint, it is his responsibility to save the checkpointIdentifier which is part of checkpointParams.
	// In case of async=true, Checkpoint returns (nil, nil) and the result of the checkpoint should be obtained through
	// CheckpointResult. Otherwise, returns CheckpointEntry pointer or error on failure.
	Checkpoint(ctx context.Context, async bool, checkpointParams checkpoint.CheckpointerParams) (*CheckpointEntry, error)

	// CheckpointResult returns CheckpointEntry pointer based on the checkpointIdentifier.
	CheckpointResult(checkpointIdentifier string) (*CheckpointEntry, error)
}

func NewCheckpointManager(checkpointer checkpoint.Checkpointer, checkpointStorage CheckpointStorage) CheckpointManager {
	return &checkpointManager{
		&checkpointsInProgress{doneMap: make(map[string]chan struct{})},
		checkpointer,
		checkpointStorage,
	}
}

// checkpointsInProgress represents an in memory map where the key is checkpointIdentifier and value is the done
// channel, which can be used by other goroutine to wait for checkpointing to finish.
type checkpointsInProgress struct {
	mu      sync.Mutex
	doneMap map[string]chan struct{}
}

func (c *checkpointsInProgress) Put(key string, value chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.doneMap[key] = value
}

func (c *checkpointsInProgress) Get(key string) chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.doneMap[key]
}

func (c *checkpointsInProgress) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.doneMap, key)
}
