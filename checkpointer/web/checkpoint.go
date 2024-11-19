package web

import (
	"checkpoint-in-k8s/pkg/checkpointer"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"
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
	checkpointer.Checkpointer
	async bool
}

func NewCheckpointHandler(client *kubernetes.Clientset, config *rest.Config, useKanikoStdin, async bool) *CheckpointHandler {
	if useKanikoStdin {
		return &CheckpointHandler{checkpointer.NewKanikoStdinCheckpointer(client, config, os.Getenv("POD_NAMESPACE")), async}
	}
	return &CheckpointHandler{checkpointer.NewKanikoFSCheckpointer(client, config, os.Getenv("POD_NAMESPACE")), async}
}

func (ch *CheckpointHandler) HandleCheckpoint(rw http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Unable to read req body: %s", err), http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	var checkpointRequest checkpointer.CheckpointRequest
	if err := json.Unmarshal(body, &checkpointRequest); err != nil {
		http.Error(rw, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	containerIdentifier := getContainerIdentifier(req)
	if containerIdentifier == nil {
		http.Error(rw, "container path in format /{namespace}{pod}/{container} expected", http.StatusBadRequest)
		return
	}
	checkpointRequest.ContainerIdentifier = *containerIdentifier

	log.Printf("req to checkpointer '%s' as '%s'", checkpointRequest.ContainerIdentifier, checkpointRequest.ContainerImageName)
	ch.doCheckpoint(rw, checkpointRequest)
}

func (ch *CheckpointHandler) doCheckpoint(writer http.ResponseWriter, checkpointRequest checkpointer.CheckpointRequest) {
	if ch.async {
		errChan := make(chan error)
		c.Put(checkpointRequest.ContainerIdentifier.String(), errChan)
		go func() {
			err := ch.Checkpoint(checkpointRequest)
			if err != nil {
				log.Printf("checkpointer failed: %s", err)
				errChan <- err
			}
			log.Printf("checkpointer done, closing channel")
			close(errChan)
		}()
		writer.WriteHeader(http.StatusOK)
		return
	}

	err := ch.Checkpoint(checkpointRequest)
	if err != nil {
		log.Printf("checkpointer failed: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("checkpointer done")
	writer.WriteHeader(http.StatusOK)
}

func (ch *CheckpointHandler) HandleCheckState(rw http.ResponseWriter, req *http.Request) {
	containerIdentifier := getContainerIdentifier(req)
	if containerIdentifier == nil {
		http.Error(rw, "container path in format /{namespace}{pod}/{container} expected", http.StatusBadRequest)
		return
	}
	log.Printf("req to check status of '%s'", containerIdentifier)
	shouldHang := req.URL.Query().Get("hang") != ""
	errChan := c.Get(containerIdentifier.String())

	if errChan == nil {
		rw.WriteHeader(http.StatusNotFound)
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
		log.Printf("checkpointer is already done")
		handleErr(rw, err)
		return
	default:
		if shouldHang {
			log.Printf("waiting for checkpointer to finish")
			err := <-errChan
			handleErr(rw, err)
			return
		}
	}
	log.Printf("checkpointing still in progress")
	rw.WriteHeader(http.StatusNoContent)
}

func getContainerIdentifier(req *http.Request) *checkpointer.ContainerIdentifier {
	namespace := req.PathValue("ns")
	pod := req.PathValue("pod")
	container := req.PathValue("container")

	if namespace == "" || pod == "" || container == "" {
		return nil
	}
	return &checkpointer.ContainerIdentifier{
		Namespace: namespace, PodName: pod, ContainerName: container,
	}
}
