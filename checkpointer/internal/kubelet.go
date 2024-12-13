package internal

import (
	"checkpoint-in-k8s/pkg/config"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"io"
	"net/http"
)

var ErrContainerNotFound = fmt.Errorf("kubelet responded with 404 status code")

// kubeletCheckpointResponse represents the JSON body that Kubelet responds with.
type kubeletCheckpointResponse struct {
	Items []string `json:"items"`
}

// KubeletController is responsible for invoking the Kubelet checkpoint API.
type KubeletController interface {

	// CallKubeletCheckpoint sends an HTTP request to the Kubelet checkpoint API.
	// Expects path to the container to be checkpointed in format {namespace}/{pod}/{container}.
	// Returns a filepath to the checkpoint tar archive on a Node, or error.
	// In case Kubelet responds with 404 status code, returns the ErrContainerNotFound error.
	CallKubeletCheckpoint(ctx context.Context, containerPath string) (string, error)
}

func NewKubeletController(kubeletConfig config.KubeletConfig) (KubeletController, error) {
	httpClient, err := newHttpClient(kubeletConfig.CertFile, kubeletConfig.KeyFile, kubeletConfig.AllowInsecure)
	if err != nil {
		return nil, fmt.Errorf("failed creating http client for kubelet: %w", err)
	}
	return &kubeletController{
		kubeletConfig.BaseUrl,
		httpClient,
	}, nil
}

// newHttpClient creates a new instance of http.Client, which has the appropriate certificate and private key
// for Kubelet authentication. The client can be configured to not verify Kubelet's certificate by allowInsecure.
// Returns the client or error if cert-key pair could not be loaded.
func newHttpClient(kubeletCertFile, kubeletKeyFile string, allowInsecure bool) (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(kubeletCertFile, kubeletKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not load client X509 cert-key pair: %w", err)
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: allowInsecure,
			},
		},
	}, nil
}

type kubeletController struct {
	// Base URL of Kubelet used to send a checkpoint request.
	kubeletBaseUrl string
	// http client with appropriate certificate and key loaded
	*http.Client
}

func (kc kubeletController) CallKubeletCheckpoint(ctx context.Context, containerPath string) (string, error) {
	requestURL := fmt.Sprintf("%s/checkpoint/%s", kc.kubeletBaseUrl, containerPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("could not create http request: %w", err)
	}

	zerolog.Ctx(ctx).Debug().Str("requestURL", requestURL).Msg("sending an HTTP request to kubelet")

	res, err := kc.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send an http request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return "", ErrContainerNotFound
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("could not read response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kubelet responded with unexpected status code: %d and body: %s", res.StatusCode, body)
	}

	var kubeletCheckpointResponse kubeletCheckpointResponse
	err = json.Unmarshal(body, &kubeletCheckpointResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON response from kubelet: %w", err)
	}

	if len(kubeletCheckpointResponse.Items) != 1 {
		return "", fmt.Errorf("unexpected response body from kubelet: %s", body)
	}
	return kubeletCheckpointResponse.Items[0], nil
}
