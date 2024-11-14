package internal

import (
	"archive/tar"
	"compress/gzip"
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

func CreateTarGzTempFile(filesMapping map[string]string) (string, error) {
	tmpTarFile, err := os.CreateTemp("", "build-context-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer tmpTarFile.Close()

	gw := gzip.NewWriter(tmpTarFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for actualFilepath, filenameInTar := range filesMapping {
		if err := addFileToTar(tw, actualFilepath, filenameInTar); err != nil {
			os.Remove(tmpTarFile.Name())
			return "", err
		}
	}

	return tmpTarFile.Name(), nil
}
