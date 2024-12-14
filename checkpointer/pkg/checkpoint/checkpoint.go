package checkpoint

import (
	"checkpoint-in-k8s/internal"
	"checkpoint-in-k8s/pkg/config"
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// This name is tied to the template file in ./templates and in Dockerfile.
// In case of change, these should not be forgotten about.
const templateFilename = "dockerfile.tmpl"

type CheckpointerParams struct {

	// ContainerIdentifier represents the container to be checkpointed.
	ContainerIdentifier ContainerIdentifier

	// DeletePod instructs whether to delete the container Pod after checkpoint.
	DeletePod bool

	// CheckpointIdentifier identifies the checkpoint request. It is also used as a unique image tag.
	CheckpointIdentifier string
}

// Checkpointer is responsible for checkpointing containers in Kubernetes.
type Checkpointer interface {
	// Checkpoint checkpoints a container based on params and returns the checkpoint image name or error.
	Checkpoint(ctx context.Context, params CheckpointerParams) (string, error)
}

// NewCheckpointer constructs new Checkpointer instance with stdin or filesystem strategy based on the UseKanikoFS
// configuration option.
func NewCheckpointer(client *kubernetes.Clientset, config *rest.Config, globalConfig config.GlobalConfig) (Checkpointer, error) {
	kubeletController, err := internal.NewKubeletController(globalConfig.KubeletConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubelet controller; %w", err)
	}

	dockerfileFactory, err := internal.NewDockerfileFactory(templateFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to create Dockerfile factory; %w", err)
	}

	podController := internal.NewPodController(client, config)

	if globalConfig.UseKanikoFS {
		return newKanikoFSCheckpointer(podController, kubeletController, dockerfileFactory, globalConfig.CheckpointConfig), nil
	}
	return newKanikoStdinCheckpointer(podController, kubeletController, dockerfileFactory, globalConfig.CheckpointConfig), nil
}

// ContainerIdentifier represents a single container within Kubernetes cluster.
type ContainerIdentifier struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

func (ci ContainerIdentifier) String() string {
	return ci.Namespace + "/" + ci.Pod + "/" + ci.Container
}
