package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

type checkpointDockerfile struct {
	TarFile             string
	CheckpointBaseImage string
}

// This name is tied to the template file in ./templates and in Dockerfile.
// In case of change, these should not be forgotten about.
const templateFilename = "dockerfile.tmpl"

// DockerfileFromTemplate creates a Dockerfile in the system's temp directory. The Dockerfile is created based on a
// template file located in templates/dockerfile.tmpl but is in working directory of Checkpointer image.
// checkpointBaseImage is used in Dockerfile's FROM command and checkpointTarName in the ADD command.
// Returns the name of the created Dockerfile or error.
func DockerfileFromTemplate(checkpointBaseImage, checkpointTarName string) (string, error) {
	filledTemplate, err := os.CreateTemp("", "dockerfile-*")
	if err != nil {
		return "", fmt.Errorf("failed to create a file in system's temp directory: %w", err)
	}
	defer filledTemplate.Close()

	templateFile, err := template.New(templateFilename).ParseFiles(templateFilename)
	if err != nil {
		os.Remove(filledTemplate.Name())
		return "", fmt.Errorf("failed to parse a template file for Dockerfile: %w", err)
	}

	err = templateFile.Execute(filledTemplate,
		checkpointDockerfile{
			filepath.Base(checkpointTarName),
			checkpointBaseImage,
		},
	)
	if err != nil {
		os.Remove(filledTemplate.Name())
		return "", fmt.Errorf("failed to create Dockerfile from template: %w", err)
	}

	return filledTemplate.Name(), nil
}
