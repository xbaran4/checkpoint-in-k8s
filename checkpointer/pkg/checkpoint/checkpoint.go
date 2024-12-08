package checkpoint

import (
	"checkpoint-in-k8s/internal"
	"checkpoint-in-k8s/pkg/config"
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CheckpointerParams struct {
	ContainerIdentifier  ContainerIdentifier
	DeletePod            bool
	CheckpointIdentifier string
}

type Checkpointer interface {
	Checkpoint(ctx context.Context, params CheckpointerParams) (string, error)
}

func NewCheckpointer(client *kubernetes.Clientset, config *rest.Config, globalConfig config.GlobalConfig) (Checkpointer, error) {
	kubeletController, err := internal.NewKubeletController(globalConfig.KubeletConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubelet controller; %w", err)
	}
	podController := internal.NewPodController(client, config)

	if globalConfig.UseKanikoFS {
		return newKanikoFSCheckpointer(podController, kubeletController, globalConfig.CheckpointConfig), nil
	}
	return newKanikoStdinCheckpointer(podController, kubeletController, globalConfig.CheckpointConfig), nil
}

type ContainerIdentifier struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

func (ci ContainerIdentifier) String() string {
	return ci.Namespace + "/" + ci.Pod + "/" + ci.Container
}
