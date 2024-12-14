package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// PrepareKanikoBuildContext copies dockerfileFilepath and files into newly generated subdirectory of
// parentBuildContextDir directory. Additionally, dockerfileFilepath is renamed to 'Dockerfile'.
// Returns path to the newly generated subdirectory or error.
//
// It is the responsibility of the caller to remove the directory after use.
func PrepareKanikoBuildContext(parentBuildContextDir, checkpointTarFilepath, dockerfileFilepath string) (string, error) {
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
