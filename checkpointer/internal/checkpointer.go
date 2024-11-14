package internal

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type CheckpointRequest struct {
	ContainerPath      string `json:"containerPath"`
	ContainerImageName string `json:"containerImageName"`
	DeletePod          bool   `json:"deletePod"`
}

func (cr CheckpointRequest) Checkpoint(clientset *kubernetes.Clientset, config *restclient.Config) error {
	log.Printf("creating kaniko pod")
	kanikoPodName, err := CreateKanikoPod(clientset, cr.ContainerImageName)
	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", cr.ContainerPath, err)
	}
	defer DeleteKanikoPod(clientset, kanikoPodName)
	log.Printf("successfully created kaniko pod")

	log.Printf("calling Kubelet checkpoint")
	checkpointTarName, err := CallKubeletCheckpoint(cr.ContainerPath)
	if err != nil {
		return fmt.Errorf("could not checkpoint container: %s with error %s", cr.ContainerPath, err)
	}
	defer os.Remove(checkpointTarName)
	log.Printf("successfully created checkpoint tar: %s", checkpointTarName)

	// TODO: clean this up
	if cr.DeletePod {
		splitted := strings.Split(cr.ContainerPath, "/")
		if len(splitted) != 3 {
			return fmt.Errorf("containerPath: '%s' has unknown format", cr.ContainerPath)
		}
		if err := DeletePod(clientset, splitted[0], splitted[1]); err != nil {
			log.Printf("could not delete notebook pod: %s with error %s", cr.ContainerPath, err) // TODO: fail on delete?
		}
		log.Printf("successfully deleted notebook pod %s/%s", splitted[0], splitted[1])
	}

	filledDockerfileTemplate, err := DockerfileFromTemplate(checkpointTarName)
	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", cr.ContainerPath, err)
	}
	defer os.Remove(filledDockerfileTemplate)
	log.Printf("successfully created new Dockerfile from template")

	buildContextTar, err := CreateTarGzTempFile(map[string]string{
		filledDockerfileTemplate: "Dockerfile",
		checkpointTarName:        checkpointTarName,
	})

	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", cr.ContainerPath, err)
	}
	defer os.Remove(buildContextTar)
	log.Printf("successfully built tar.gz with buildcontext")

	if err = AttachToPod(clientset, config, kanikoPodName, buildContextTar); err != nil {
		return fmt.Errorf("failed to attach to pod: %v\n", err)
	}

	log.Printf("checkpointing done, about to cleanup resources")
	return nil
}

// CheckpointV2 Currently unused
func (cr CheckpointRequest) CheckpointV2(clientset *kubernetes.Clientset, config *restclient.Config) error {
	checkpointTarName, err := CallKubeletCheckpoint(cr.ContainerPath)
	if err != nil {
		return fmt.Errorf("could not checkpoint container: %s with error %s", cr.ContainerPath, err)
	}
	defer os.Remove(checkpointTarName)
	log.Printf("successfully created checkpoint tar: %s", checkpointTarName)

	filledDockerfileTemplate, err := DockerfileFromTemplate(checkpointTarName)
	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", cr.ContainerPath, err)
	}
	defer os.Remove(filledDockerfileTemplate)
	log.Printf("successfully created new Dockerfile from template")

	tmpDir, err := PrepareDir(checkpointTarName, filledDockerfileTemplate)
	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", cr.ContainerPath, err)
	}

	_, err = CreateKanikoPodV2(clientset, cr.ContainerImageName, filepath.Base(filledDockerfileTemplate), tmpDir)
	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", cr.ContainerPath, err)
	}

	log.Printf("checkpointing done, about to cleanup resources")
	return nil
}
