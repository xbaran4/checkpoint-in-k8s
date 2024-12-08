package web

import (
	"checkpoint-in-k8s/internal"
	"checkpoint-in-k8s/pkg/checkpoint"
	"checkpoint-in-k8s/pkg/manager"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strings"
)

type CheckpointRequestBody struct {
	DeletePod bool `json:"deletePod,omitempty"`
	Async     bool `json:"async,omitempty"`
}

type TrackingHandleResponseBody struct {
	CheckpointIdentifier string `json:"checkpointIdentifier"`
}

type CheckpointHandler struct {
	manager.CheckpointManager
	checkpointerNode string
}

func NewCheckpointHandler(checkpointManager manager.CheckpointManager, checkpointerNode string) *CheckpointHandler {
	return &CheckpointHandler{checkpointManager, checkpointerNode}
}

func (ch *CheckpointHandler) HandleCheckpoint(rw http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Unable to read req body: %s", err), http.StatusBadRequest)
		return
	}

	var requestBody CheckpointRequestBody
	if err := json.Unmarshal(body, &requestBody); err != nil {
		http.Error(rw, fmt.Sprintf("Invalid JSON format: %s", err), http.StatusBadRequest)
		return
	}

	containerIdentifier := getContainerIdentifier(req)
	if containerIdentifier == nil {
		http.Error(rw, "container path in format /{namespace}{pod}/{container} expected", http.StatusBadRequest)
		return
	}

	lg := log.With().Str("containerIdentifier", containerIdentifier.String()).Logger()
	lg.Info().Msg("request to checkpoint container")

	checkpointIdentifier, err := generateCheckpointIdentifier()
	if err != nil {
		lg.Error().Err(err).Msg("failed to generate checkpoint identifier")
		http.Error(rw, "failed to generate checkpoint identifier", http.StatusInternalServerError)
		return
	}

	cp, err := ch.Checkpoint(req.Context(), requestBody.Async, checkpoint.CheckpointerParams{
		ContainerIdentifier:  *containerIdentifier,
		DeletePod:            requestBody.DeletePod,
		CheckpointIdentifier: checkpointIdentifier,
	})

	if err != nil {
		if errors.Is(err, internal.ErrContainerNotFound) {
			http.Error(rw, "checkpointer could not find the container", http.StatusNotFound)
			return
		}
		lg.Error().Err(err).Msg("checkpointing failed")
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	if cp == nil {
		response := TrackingHandleResponseBody{CheckpointIdentifier: ch.checkpointerNode + ":" + checkpointIdentifier}
		rw.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(rw).Encode(response); err != nil {
			lg.Error().Err(err).Msg("unable to encode JSON")
			http.Error(rw, "unable to encode JSON", http.StatusInternalServerError)
			return
		}
		return
	}

	rw.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(rw).Encode(cp); err != nil {
		lg.Error().Err(err).Msg("unable to encode JSON")
		http.Error(rw, "unable to encode JSON", http.StatusInternalServerError)
		return
	}
}

func (ch *CheckpointHandler) HandleCheckState(rw http.ResponseWriter, req *http.Request) {
	_, checkpointIdentifier := getCheckpointIdentifier(req)
	if checkpointIdentifier == "" {
		http.Error(rw, "query param checkpointIdentifier empty or malformed", http.StatusBadRequest)
		return
	}

	lg := log.With().
		Str("checkpointIdentifier", checkpointIdentifier).
		Logger()

	lg.Info().Msg("received request to check status of checkpointing")

	checkpointState, err := ch.CheckpointResult(checkpointIdentifier)
	if err != nil {
		http.Error(rw, "failed to get the state of a checkpoint", http.StatusInternalServerError)
		return
	}

	if checkpointState == nil {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if checkpointState.Error != nil {
		if errors.Is(checkpointState.Error, internal.ErrContainerNotFound) {
			http.Error(rw, "checkpointer could not find the container", http.StatusNotFound)
			return
		}
		http.Error(rw, "checkpointing failed", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(rw).Encode(checkpointState); err != nil {
		lg.Error().Err(err).Msg("unable to encode JSON")
		http.Error(rw, "unable to encode JSON", http.StatusInternalServerError)
		return
	}
}

func generateCheckpointIdentifier() (string, error) {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func getContainerIdentifier(req *http.Request) *checkpoint.ContainerIdentifier {
	namespace := req.PathValue("ns")
	pod := req.PathValue("pod")
	container := req.PathValue("container")

	if namespace == "" || pod == "" || container == "" {
		return nil
	}
	return &checkpoint.ContainerIdentifier{
		Namespace: namespace, Pod: pod, Container: container,
	}
}

func getCheckpointIdentifier(req *http.Request) (leftSide, rightSide string) {
	l, r, found := strings.Cut(req.URL.Query().Get("checkpointIdentifier"), ":")
	if found {
		return l, r
	}
	return "", ""
}
