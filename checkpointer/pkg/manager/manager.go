package manager

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"context"
	"sync"
)

type CheckpointManager interface {
	Checkpoint(ctx context.Context, async bool, checkpointRequest checkpoint.CheckpointerParams) (*CheckpointEntry, error)
	CheckpointResult(checkpointIdentifier string) (*CheckpointEntry, error)
}

func NewCheckpointManager(checkpointer checkpoint.Checkpointer, checkpointStorage CheckpointStorage) CheckpointManager {
	dm := &checkpointsInProgress{doneMap: make(map[string]chan struct{})}

	return &defaultCheckpointManager{dm, checkpointer, checkpointStorage}
}

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
