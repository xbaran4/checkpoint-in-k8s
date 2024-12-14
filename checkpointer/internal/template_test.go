package internal

import (
	"os"
	"testing"
)

const dockerfileContent = `FROM quay.io/baseimage
ADD checkpoint-archive /
`

func Test_dockerfileFactory_DockerfileFromTemplate(t *testing.T) {
	factory, err := NewDockerfileFactory("templates/dockerfile.tmpl")
	if err != nil {
		t.Fatalf("error creating docker file factory: %v", err)
	}
	dockerfile, err := factory.DockerfileFromTemplate("quay.io/baseimage", "checkpoint-archive")
	if err != nil {
		t.Fatalf("error creating docker file from template: %v", err)
	}
	defer os.Remove(dockerfile)

	fileContent, err := os.ReadFile(dockerfile)
	if err != nil {
		t.Fatalf("error reading docker file: %v", err)
	}

	if string(fileContent) != dockerfileContent {
		t.Fatalf("dockerfile contents don't match: %v", string(fileContent))
	}
}
