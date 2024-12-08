package internal

import (
	"checkpoint-in-k8s/pkg/config"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var ErrContainerNotFound = fmt.Errorf("kubelet responded with 404 status code")

type kubeletCheckpointResponse struct {
	Items []string `json:"items"`
}

type KubeletController interface {
	CallKubeletCheckpoint(ctx context.Context, containerPath string) (string, error)
}

func NewKubeletController(kubeletConfig config.KubeletConfig) (KubeletController, error) {
	httpClient, err := newHttpClient(kubeletConfig.CertFile, kubeletConfig.KeyFile, kubeletConfig.AllowInsecure)
	if err != nil {
		return nil, fmt.Errorf("error creating http client for kubelet: %w", err)
	}
	return &kubeletController{
		kubeletConfig.BaseUrl,
		httpClient,
	}, nil
}

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
	kubeletBaseUrl string
	*http.Client
}

func (kc kubeletController) CallKubeletCheckpoint(ctx context.Context, containerPath string) (string, error) {
	requestURL := fmt.Sprintf("%s/checkpoint/%s", kc.kubeletBaseUrl, containerPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("could not create request: %w", err)
	}

	res, err := kc.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making http request: %w", err)
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
