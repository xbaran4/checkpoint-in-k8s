package web

import (
	"checkpoint-in-k8s/internal"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type checkpointRequestData struct {
	ContainerPath      string `json:"containerPath"`
	ContainerImageName string `json:"containerImageName"`
}

type CheckpointHandler struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

func NewCheckpointHandler(clientset *kubernetes.Clientset, config *rest.Config) *CheckpointHandler {
	return &CheckpointHandler{clientset, config}
}

func (ch *CheckpointHandler) Handle(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "Invalid request method, use POST instead", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, fmt.Sprintf("Unable to read request body: %s", err), http.StatusBadRequest)
		return
	}
	defer request.Body.Close()

	var requestData checkpointRequestData
	if err := json.Unmarshal(body, &requestData); err != nil {
		http.Error(writer, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	log.Printf("request to checkpoint '%s' as '%s'", requestData.ContainerPath, requestData.ContainerImageName)

	err = ch.doCheckpoint(requestData.ContainerPath, requestData.ContainerImageName)

	if errors.Is(err, internal.ErrContainerNotFound) {
		log.Printf("did not checkpoint container: %s", err)
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("could not checkpoint container: %s with error %s", requestData.ContainerPath, err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}

// TODO: add context to function calls
// TODO: creating Kaniko could be async at the price of harder error handling.
func (ch *CheckpointHandler) doCheckpoint(containerPath, containerImageName string) error {
	checkpointTarName, err := internal.CallKubeletCheckpoint(containerPath)
	if err != nil {
		return fmt.Errorf("could not checkpoint container: %s with error %s", containerPath, err)
	}
	defer os.Remove(checkpointTarName)
	log.Printf("successfully created checkpoint tar: %s", checkpointTarName)

	filledDockerfileTemplate, err := internal.DockerfileFromTemplate(checkpointTarName)
	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", containerPath, err)
	}
	defer os.Remove(filledDockerfileTemplate)
	log.Printf("successfully created new Dockerfile from template")

	kanikoPodName, err := internal.CreateKanikoPod(ch.clientset, containerImageName, filepath.Base(filledDockerfileTemplate))
	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", containerPath, err)
	}
	defer internal.DeleteKanikoPod(ch.clientset, kanikoPodName)
	log.Printf("successfully created kaniko pod")

	buildContextTar, err := internal.CreateTarGzTempFile([]string{filledDockerfileTemplate, checkpointTarName})
	if err != nil {
		return fmt.Errorf("could not create checkpoint container: %s with error %s", containerPath, err)
	}
	defer os.Remove(buildContextTar)
	log.Printf("successfully built tar.gz with buildcontext")

	if err = internal.AttachToPod(ch.clientset, ch.config, kanikoPodName, buildContextTar); err != nil {
		return fmt.Errorf("failed to attach to pod: %v\n", err)
	}

	log.Printf("checkpointing done, about to cleanup resources")
	return nil
}
