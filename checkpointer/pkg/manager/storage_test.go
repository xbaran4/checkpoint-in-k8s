package manager

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"checkpoint-in-k8s/pkg/config"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const marshalledCheckpointEntry = "{\"containerIdentifier\":{\"namespace\":\"ns\",\"pod\":\"pod\",\"container\":\"ctrn\"},\"beginTimestamp\":100000000,\"endTimestamp\":200000000,\"containerImageName\":\"quay.io/image:abcd\"}"

var checkpointEntry = CheckpointEntry{
	ContainerIdentifier: checkpoint.ContainerIdentifier{
		Namespace: "ns",
		Pod:       "pod",
		Container: "ctrn",
	},
	BeginTimestamp:     100000000,
	EndTimestamp:       200000000,
	ContainerImageName: "quay.io/image:abcd",
	Error:              nil,
}

func Test_checkpointDiskStorage_ReadEntry(t *testing.T) {
	tempDir := os.TempDir()
	defer os.RemoveAll(tempDir)

	testFile, err := os.CreateTemp(tempDir, "test-file-")
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer testFile.Close()

	_, err = io.WriteString(testFile, marshalledCheckpointEntry)
	if err != nil {
		t.Fatalf("failed to write to a test file: %v", err)
	}

	storage := NewCheckpointStorage(config.GlobalConfig{StorageBasePath: tempDir})

	readEntry, err := storage.ReadEntry(filepath.Base(testFile.Name()))
	if err != nil {
		t.Fatalf("failed to to read CheckpointEntry: %v", err)
	}
	if readEntry == nil {
		t.Fatalf("did not find any CheckpointEntry")
	}
	if !reflect.DeepEqual(*readEntry, checkpointEntry) {
		t.Fatalf("did not match CheckpointEntry: %v", checkpointEntry)
	}
}

func Test_checkpointDiskStorage_StoreEntry(t *testing.T) {
	tempDir := os.TempDir()
	defer os.RemoveAll(tempDir)

	storage := NewCheckpointStorage(config.GlobalConfig{StorageBasePath: tempDir})

	err := storage.StoreEntry("test", checkpointEntry)
	if err != nil {
		t.Errorf("failed to to store CheckpointEntry: %v", err)
	}

	fileContent, err := os.ReadFile(filepath.Join(tempDir, "test"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != marshalledCheckpointEntry {
		t.Errorf("file contents don't match CheckpointEntry: \n%s\n%s", string(fileContent), marshalledCheckpointEntry)
	}
}
