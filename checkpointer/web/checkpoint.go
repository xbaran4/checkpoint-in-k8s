package web

import (
	"checkpoint-in-k8s/internal"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"sync"
)

type SafeMap struct {
	mu sync.Mutex
	v  map[string]chan error
}

func (c *SafeMap) Put(key string, value chan error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.v[key] = value
}

func (c *SafeMap) Get(key string) chan error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.v[key]
}

// TODO: CHEATING
var c *SafeMap

func init() {
	c = &SafeMap{v: make(map[string]chan error)}
}

type CheckpointHandler struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

func NewCheckpointHandler(clientset *kubernetes.Clientset, config *rest.Config) *CheckpointHandler {
	return &CheckpointHandler{clientset, config}
}

func (ch *CheckpointHandler) HandleCheckpointAsync(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, fmt.Sprintf("Unable to read request body: %s", err), http.StatusBadRequest)
		return
	}
	defer request.Body.Close()

	var checkpointRequest internal.CheckpointRequest
	if err := json.Unmarshal(body, &checkpointRequest); err != nil {
		http.Error(writer, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	log.Printf("request to checkpoint '%s' as '%s'", checkpointRequest.ContainerPath, checkpointRequest.ContainerImageName)

	errChan := make(chan error)
	c.Put(checkpointRequest.ContainerPath, errChan)
	go func() {
		err := checkpointRequest.Checkpoint(ch.clientset, ch.config)
		if err != nil {
			log.Printf("checkpoint failed: %s", err)
			errChan <- err
		}
		log.Printf("checkpoint done, closing channel")
		close(errChan)
	}()
	writer.WriteHeader(http.StatusOK)
}

func (ch *CheckpointHandler) HandleCheckState(writer http.ResponseWriter, request *http.Request) {
	containerPath := request.URL.Query().Get("containerPath")
	log.Printf("request to check status of '%s'", containerPath)
	shouldHang := request.URL.Query().Get("hang") != ""
	errChan := c.Get(containerPath)

	if errChan == nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	handleErr := func(writer http.ResponseWriter, err error) {
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}

	select {
	case err := <-errChan:
		log.Printf("checkpoint is already done")
		handleErr(writer, err)
		return
	default:
		if shouldHang {
			log.Printf("waiting for checkpoint to finish")
			err := <-errChan
			handleErr(writer, err)
			return
		}
	}
	log.Printf("checkpointing still in progress")
	writer.WriteHeader(http.StatusNoContent)
}
