package internal

import (
	"os"
	"path/filepath"
	"text/template"
)

type checkpointDockerfile struct {
	CheckpointBaseImage string
	TarFile             string
}

// This name is tied to the filename in ./templates and in Dockerfile.
// In case of change, these should not be forgotten about.
const templateFilename = "dockerfile.tmpl"

func DockerfileFromTemplate(checkpointTarName string) (string, error) {
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

	err = templateFile.Execute(filledTemplate,
		checkpointDockerfile{
			os.Getenv("CHECKPOINT_BASE_IMAGE"),
			filepath.Base(checkpointTarName),
		},
	)
	if err != nil {
		defer os.Remove(filledTemplate.Name())
		return "", err
	}

	return filledTemplate.Name(), nil
}
