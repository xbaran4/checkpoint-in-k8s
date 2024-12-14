package internal

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func addFileToTar(tw *tar.Writer, actualFilepath, filenameInTar string) error {
	file, err := os.Open(actualFilepath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.Base(filenameInTar)

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}

// CreateTarGzTempFile creates gzip compressed tar archive in the system's temp directory.
// The files which should be included in the archive are expected as keys in filesMapping map,
// where the values define the filename inside the archive. All the files will be put to the root of the archive.
// Returns filepath of the archive or error.
//
// It is the responsibility of the caller to remove the archive after use.
func CreateTarGzTempFile(filesMapping map[string]string) (string, error) {
	tmpTarFile, err := os.CreateTemp("", "build-context-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create a file in system's temp directory: %w", err)
	}
	defer tmpTarFile.Close()

	gw := gzip.NewWriter(tmpTarFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for actualFilepath, filenameInTar := range filesMapping {
		if err := addFileToTar(tw, actualFilepath, filenameInTar); err != nil {
			os.Remove(tmpTarFile.Name())
			return "", fmt.Errorf("failed to add %s to the compressed tar archive: %w", actualFilepath, err)
		}
	}

	return tmpTarFile.Name(), nil
}
