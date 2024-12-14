package manager

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"context"
	"testing"
)

type mockCheckpointer struct {
}

func (m mockCheckpointer) Checkpoint(context.Context, checkpoint.CheckpointerParams) (string, error) {
	return "quay.io/checkpointed", nil
}

type mockStorage struct {
	storage map[string]*CheckpointEntry
}

func (m mockStorage) StoreEntry(checkpointIdentifier string, entry CheckpointEntry) error {
	m.storage[checkpointIdentifier] = &entry
	return nil
}

func (m mockStorage) ReadEntry(checkpointIdentifier string) (*CheckpointEntry, error) {
	entry, _ := m.storage[checkpointIdentifier]
	return entry, nil
}

func Test_checkpointManager_doCheckpoint(t *testing.T) {
	manager := &checkpointManager{
		checkpointsInProgress: &checkpointsInProgress{doneMap: make(map[string]chan struct{})},
		checkpointer:          mockCheckpointer{},
		checkpointStorage:     mockStorage{make(map[string]*CheckpointEntry)},
	}
	params := checkpoint.CheckpointerParams{
		ContainerIdentifier:  checkpoint.ContainerIdentifier{},
		DeletePod:            false,
		CheckpointIdentifier: "id",
	}

	entry, _ := manager.doCheckpoint(context.TODO(), params)
	if entry.ContainerImageName != "quay.io/checkpointed" {
		t.Fatalf("ContainerImageName is malformed")
	}
}

func Test_checkpointManager_CheckpointResult(t *testing.T) {
	entry := &CheckpointEntry{}
	manager := &checkpointManager{
		checkpointsInProgress: &checkpointsInProgress{doneMap: make(map[string]chan struct{})},
		checkpointer:          mockCheckpointer{},
		checkpointStorage:     mockStorage{map[string]*CheckpointEntry{"test": entry}},
	}

	result, err := manager.CheckpointResult("test")
	if err != nil {
		t.Fatalf("CheckpointResult return unexpected error: %v", err)
	}
	if result != entry {
		t.Fatalf("CheckpointResult returned wrong result")
	}
}

func Test_checkpointManager_doCheckpointAsync(t *testing.T) {
	manager := &checkpointManager{
		checkpointsInProgress: &checkpointsInProgress{doneMap: make(map[string]chan struct{})},
		checkpointer:          mockCheckpointer{},
		checkpointStorage:     mockStorage{make(map[string]*CheckpointEntry)},
	}
	params := checkpoint.CheckpointerParams{
		ContainerIdentifier:  checkpoint.ContainerIdentifier{},
		DeletePod:            false,
		CheckpointIdentifier: "id",
	}

	channel := make(chan struct{})
	manager.doCheckpointAsync(params, channel)

	select {
	case <-channel:
	default:
		t.Fatalf("done channel should be close after checkpointing")
	}

	if manager.checkpointsInProgress.Get("id") != nil {
		t.Fatalf("there should be no checkpoint in progress with 'id' identifier")
	}

	entry, _ := manager.checkpointStorage.ReadEntry("id")
	if entry == nil {
		t.Fatalf("manager did not save the checkpoint result")
	}

	if entry.ContainerImageName != "quay.io/checkpointed" {
		t.Fatalf("ContainerImageName is malformed")
	}
}
