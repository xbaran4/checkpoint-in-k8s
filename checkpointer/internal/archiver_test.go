package internal

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"slices"
	"testing"
)

func TestCreateTarGzTempFile_NonExistingFile(t *testing.T) {
	_, err := CreateTarGzTempFile(map[string]string{
		"nonexsting": "nonexsting",
	})
	if err == nil {
		t.Errorf("CreateTarGzTempFile() should have failed with error")
	}
}

func TestCreateTarGzTempFileNameInTarMatches(t *testing.T) {

	testFilename1 := makeTmpFile(t)
	defer os.Remove(testFilename1)

	testFilename2 := makeTmpFile(t)
	defer os.Remove(testFilename2)

	tarFilename, err := CreateTarGzTempFile(map[string]string{
		testFilename1: "path/should/not/matter/tarInFile1.txt",
		testFilename2: "tarInFile2.txt",
	})
	defer os.Remove(tarFilename)

	if err != nil {
		t.Errorf("CreateTarGzTempFile failed with error: %v", err)
	}

	tarFile, err := os.Open(tarFilename)
	if err != nil {
		t.Fatalf("Error opening tarFile: %v", err)
		return
	}
	defer tarFile.Close()

	gzipReader, err := gzip.NewReader(tarFile)
	if err != nil {
		t.Fatalf("Error creating gzip reader: %v", err)
		return
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)

	files := make([]string, 0, 2)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Errorf("Error reading tar archive: %v", err)
			return
		}
		if header.Typeflag == tar.TypeReg {
			files = append(files, header.Name)
		} else {
			t.Errorf("tar contains unknown file %s", header.Name)
		}
	}
	if len(files) != 2 {
		t.Errorf("tar contains wrong number of files")
	}
	if !slices.Contains(files, "tarInFile1.txt") {
		t.Errorf("tar does not contain file tarInFile1.txt")
	}

	if !slices.Contains(files, "tarInFile2.txt") {
		t.Errorf("tar does not contain file tarInFile2.txt")
	}
}

func makeTmpFile(t *testing.T) string {
	testFile, err := os.CreateTemp("", "test-file-")
	if err != nil {
		t.Fatalf("Failed to create test tarFile: %v", err)
	}
	testFile.Close()
	return testFile.Name()
}
