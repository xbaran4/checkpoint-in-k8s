package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareKanikoBuildContext(t *testing.T) {
	tempDir := os.TempDir()
	defer os.RemoveAll(tempDir)
	testDockerfile := makeTmpFile(t)
	defer os.Remove(testDockerfile)
	testCheckpointArchive := makeTmpFile(t)
	defer os.Remove(testCheckpointArchive)

	buildContext, err := PrepareKanikoBuildContext(tempDir, testCheckpointArchive, testDockerfile)
	if err != nil {
		t.Fatalf("PrepareKanikoBuildContext returned an error: %v", err)
	}

	dir, err := os.ReadDir(buildContext)
	if err != nil {
		t.Fatalf("could not read build context dir: %v", err)
	}
	if len(dir) != 2 {
		t.Fatalf("build context dir contains wrong number of files")
	}

	if "Dockerfile" != dir[0].Name() && "Dockerfile" != dir[1].Name() {
		t.Fatalf("build context dir does not contain Dockerfile")
	}

	if filepath.Base(testCheckpointArchive) != dir[0].Name() && filepath.Base(testCheckpointArchive) != dir[1].Name() {
		t.Fatalf("build context dir does not contain checkpoint archive")
	}
}
