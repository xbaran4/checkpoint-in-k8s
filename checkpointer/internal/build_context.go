package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const parentBuildContextDir = "/tmp/build-contexts"

func PrepareDir(checkpointTarFilepath, dockerfileFilepath string) (string, error) {
	tempDir, err := os.MkdirTemp(parentBuildContextDir, "context-")
	if err != nil {
		fmt.Println("Error creating temporary directory:", err)
		return "", err
	}

	err = copyFile(checkpointTarFilepath, tempDir+"/"+filepath.Base(checkpointTarFilepath))
	if err != nil {
		fmt.Println("Error moving file:", err)
		return "", err
	}

	err = copyFile(dockerfileFilepath, tempDir+"/Dockerfile")
	if err != nil {
		fmt.Println("Error moving file:", err)
		return "", err
	}

	return tempDir, nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}
