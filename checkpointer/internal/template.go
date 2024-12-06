package internal

import (
	"os"
	"path/filepath"
	"text/template"
)

type checkpointDockerfile struct {
	TarFile             string
	CheckpointBaseImage string
}

// This name is tied to the filename in ./templates and in Dockerfile.
// In case of change, these should not be forgotten about.
const templateFilename = "dockerfile.tmpl"

func DockerfileFromTemplate(checkpointBaseImage, checkpointTarName string) (string, error) {
	filledTemplate, err := os.CreateTemp("", "dockerfile-*")
	if err != nil {
		return "", err
	}
	defer filledTemplate.Close()

	templateFile, err := template.New(templateFilename).ParseFiles(templateFilename)
	if err != nil {
		defer os.Remove(filledTemplate.Name())
		return "", err
	}

	// TODO: make base image with both cri-o and containerd labels
	err = templateFile.Execute(filledTemplate,
		checkpointDockerfile{
			checkpointBaseImage,
			filepath.Base(checkpointTarName),
		},
	)
	if err != nil {
		defer os.Remove(filledTemplate.Name())
		return "", err
	}

	return filledTemplate.Name(), nil
}
