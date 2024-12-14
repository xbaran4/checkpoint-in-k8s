package internal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallKubeletCheckpoint_Success(t *testing.T) {
	archiveName := "/checkpoint/tar-archive-123"
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkpoint/ns/pod/contr" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(kubeletCheckpointResponse{
			[]string{archiveName},
		})
		w.Write(body)
	}))
	defer mockServer.Close()

	kubeletCtrl := kubeletController{
		mockServer.URL,
		&http.Client{},
	}

	checkpointArchive, err := kubeletCtrl.CallKubeletCheckpoint(context.TODO(), "ns/pod/contr")
	if err != nil {
		t.Errorf("CallKubeletCheckpoint failed with error %v", err)
	}

	if checkpointArchive != archiveName {
		t.Errorf("CallKubeletCheckpoint returned wrong checkpoint archive name: %s", checkpointArchive)
	}
}

func TestCallKubeletCheckpoint_NotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
		return
	}))
	defer mockServer.Close()

	kubeletCtrl := kubeletController{
		mockServer.URL,
		&http.Client{},
	}

	_, err := kubeletCtrl.CallKubeletCheckpoint(context.TODO(), "ns/pod/contr")
	if err == nil {
		t.Errorf("CallKubeletCheckpoint returned no error")
	}
	if !errors.Is(err, ErrContainerNotFound) {
		t.Errorf("CallKubeletCheckpoint failed with wrong error: %v", err)
	}
}

func TestCallKubeletCheckpoint_MalformedResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("unexpected"))
	}))
	defer mockServer.Close()

	kubeletCtrl := kubeletController{
		mockServer.URL,
		&http.Client{},
	}

	_, err := kubeletCtrl.CallKubeletCheckpoint(context.TODO(), "ns/pod/contr")
	if err == nil {
		t.Errorf("CallKubeletCheckpoint returned no error")
	}
}
