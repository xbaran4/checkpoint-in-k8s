package web

import (
	"checkpoint-in-k8s/pkg/checkpointer"
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
		http.Error(rw, fmt.Sprintf("Invalid JSON format: %s", err), http.StatusBadRequest)
		return
	}

	containerIdentifier := getContainerIdentifier(req)
	if containerIdentifier == nil {
		http.Error(rw, "container path in format /{namespace}{pod}/{container} expected", http.StatusBadRequest)
		return
	}
	checkpointRequest.ContainerIdentifier = *containerIdentifier
	lg := log.With().Str("containerIdentifier", containerIdentifier.String()).Logger()

	lg.Info().Str("imageName", checkpointRequest.ContainerImageName).Msg("request to checkpoint container")

	if ch.async {
		ch.doCheckpointAsync(checkpointRequest)
		rw.WriteHeader(http.StatusOK)
		return
	}

	err = ch.Checkpoint(lg.WithContext(req.Context()), checkpointRequest)
	if err != nil {
		lg.Error().Err(err).Msg("checkpointer failed")
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	lg.Info().Msg("checkpointer done")
	rw.WriteHeader(http.StatusOK)
}

func (ch *CheckpointHandler) doCheckpointAsync(checkpointRequest checkpointer.CheckpointRequest) {
	errChan := make(chan error)
	c.Put(checkpointRequest.ContainerIdentifier.String(), errChan)
	go func() {
		err := ch.Checkpoint(context.Background(), checkpointRequest)
		if err != nil {
			log.Error().Err(err).Msg("checkpointer failed")
			errChan <- err
		}
		log.Debug().Msg("checkpointer done, closing channel")
		close(errChan)
	}()
}

func (ch *CheckpointHandler) HandleCheckState(rw http.ResponseWriter, req *http.Request) {
	containerIdentifier := getContainerIdentifier(req)
	if containerIdentifier == nil {
		http.Error(rw, "container path in format /{namespace}{pod}/{container} expected", http.StatusBadRequest)
		return
	}
	lg := log.With().Str("containerIdentifier", containerIdentifier.String()).Logger()

	shouldHang := req.URL.Query().Get("hang") != ""

	lg.Info().Bool("shouldHang", shouldHang).Msg("received request to check status of checkpointing")

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
		lg.Debug().Msg("checkpointer is already done")
		handleErr(rw, err)
		return
	default:
		if shouldHang {
			lg.Debug().Msg("waiting for checkpointer to finish")
			err := <-errChan
			handleErr(rw, err)
			return
		}
	}
	lg.Debug().Msg("checkpointing still in progress")
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
