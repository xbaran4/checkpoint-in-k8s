package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
)

type KubeletCheckpointResponse struct {
	Items []string `json:"items"`
}

var serverPort = os.Getenv("KUBELET_PORT")

func main() {
	http.HandleFunc("/checkpoint", checkpoint)
	log.Println("starting http server")
	err := http.ListenAndServe(":3333", nil)

	if errors.Is(err, http.ErrServerClosed) {
		log.Println("server closed")
	} else if err != nil {
		log.Fatalf("error starting server: %s\n", err)
	}
}

func httpClient() (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(os.Getenv("KUBELET_CERT_FILE"), os.Getenv("KUBELET_CERT_KEY"))
	if err != nil {
		return nil, fmt.Errorf("could not load client cert-key pair: %w", err)
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true, // TODO: handle self-singed certs
			},
		},
	}, nil
}

func checkpoint(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	containerPath := query.Get("containerPath")
	containerImageName := query.Get("containerImageName")

	checkpointTarName, err := callKubeletCheckpoint(containerPath)
	if err != nil {
		log.Printf("could not checkpoint container: %s with error %s", containerPath, err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = createCheckpointImage(checkpointTarName, containerImageName)
	if err != nil {
		log.Printf("could not create checkpoint container: %s with error %s", containerPath, err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}

// TODO: extract callKubeletCheckpoint and createCheckpointImage into separate interfaces

func callKubeletCheckpoint(containerPath string) (string, error) {
	requestURL := fmt.Sprintf("http://localhost:%s%s", serverPort, containerPath)

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
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

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kubelet responded with unexpected status code: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("could not read response body: %w", err)
	}

	var kubeletCheckpointResponse KubeletCheckpointResponse
	err = json.Unmarshal(body, &kubeletCheckpointResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON response from kubelet: %w", err)
	}
	if len(kubeletCheckpointResponse.Items) == 0 {
		return "", fmt.Errorf("unexpected response body from kubelet: %s", body)
	}

	return kubeletCheckpointResponse.Items[0], nil
}

// TODO: investigate if buildah could be run as go library
func createCheckpointImage(checkpointTarName, newContainerImageName string) error {

	stdout, err := exec.Command("buildah", "from", "scratch").Output()
	if err != nil {
		return err
	}
	workingContainer := string(stdout)
	_, err = exec.Command("buildah", "add", workingContainer, checkpointTarName, "/").Output()
	if err != nil {
		return err
	}
	_, err = exec.Command("buildah", "config", "--annotation=io.kubernetes.cri-o.annotations.checkpoint.name=checkpoint-name", workingContainer).Output()
	if err != nil {
		return err
	}
	_, err = exec.Command("buildah", "commit", workingContainer, "checkpoint-container").Output()
	if err != nil {
		return err
	}

	_, err = exec.Command("buildah",
		"push",
		"--creds",
		os.Getenv("REGISTRY_USERNAME")+":"+os.Getenv("REGISTRY_PASSWORD"),
		workingContainer,
		newContainerImageName).Output()

	return err
}
