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

func NewDockerfileFactory(templateFile string) (DockerfileFactory, error) {
	dockerfileTemplate, err := template.New(filepath.Base(templateFile)).ParseFiles(templateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse a template file for Dockerfile: %w", err)
	}
	return dockerfileFactory{
		dockerfileTemplate,
	}, nil
}

type DockerfileFactory interface {
	// DockerfileFromTemplate creates a Dockerfile in the system's temp directory. The Dockerfile is created based on a
	// template file located in templates/dockerfile.tmpl but is in working directory of Checkpointer image.
	// checkpointBaseImage is used in Dockerfile's FROM command and checkpointTarName in the ADD command.
	// Returns the name of the created Dockerfile or error.
	DockerfileFromTemplate(checkpointBaseImage, checkpointTarName string) (string, error)
}

type dockerfileFactory struct {
	template *template.Template
}

func (df dockerfileFactory) DockerfileFromTemplate(checkpointBaseImage, checkpointTarName string) (string, error) {
	filledTemplate, err := os.CreateTemp("", "dockerfile-*")
	if err != nil {
		return "", fmt.Errorf("failed to create a file in system's temp directory: %w", err)
	}
	defer filledTemplate.Close()

	err = df.template.Execute(filledTemplate,
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
