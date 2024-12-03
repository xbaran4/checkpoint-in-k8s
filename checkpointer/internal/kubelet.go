package internal

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type kubeletCheckpointResponse struct {
	Items []string `json:"items"`
}

var ErrContainerNotFound = fmt.Errorf("kubelet responded with 404 status code")

func httpClient() (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(os.Getenv("KUBELET_CERT_FILE"), os.Getenv("KUBELET_CERT_KEY"))
	if err != nil {
		return nil, fmt.Errorf("could not load client cert-key pair: %w", err)
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: os.Getenv("KUBELET_ALLOW_INSECURE") != "",
			},
		},
	}, nil
}

func CallKubeletCheckpoint(ctx context.Context, containerPath string) (string, error) {
	requestURL := fmt.Sprintf("https://localhost:%s/checkpoint/%s", os.Getenv("KUBELET_PORT"), containerPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("could not create request: %w", err)
	}

	client, err := httpClient()
	if err != nil {
		return "", err
	}

	res, err := client.Do(req)
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

	if len(kubeletCheckpointResponse.Items) == 0 {
		return "", fmt.Errorf("unexpected response body from kubelet: %s", body)
	}
	return kubeletCheckpointResponse.Items[0], nil
}
