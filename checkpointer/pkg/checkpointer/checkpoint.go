package checkpointer

import (
	"checkpoint-in-k8s/pkg/config"
	"context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CheckpointRequest struct {
	ContainerIdentifier  ContainerIdentifier
	DeletePod            bool   `json:"deletePod"`
	CheckpointIdentifier string `json:"checkpointIdentifier"`
	Async                bool   `json:"async"`
}

type Checkpointer interface {
	Checkpoint(ctx context.Context, cr CheckpointRequest) error
}

func NewCheckpointer(client *kubernetes.Clientset, config *rest.Config, checkpointerConfig config.CheckpointerConfig) Checkpointer {
	if checkpointerConfig.UseKanikoFS {
		return newKanikoFSCheckpointer(
			client,
			config,
			checkpointerConfig.KanikoSecretName,
			checkpointerConfig.CheckpointerNamespace,
			checkpointerConfig.CheckpointerNode,
			checkpointerConfig.CheckpointImageBase,
		)
	}
	return newKanikoStdinCheckpointer(
		client,
		config,
		checkpointerConfig.KanikoSecretName,
		checkpointerConfig.CheckpointerNamespace,
		checkpointerConfig.CheckpointImageBase,
	)
}

type ContainerIdentifier struct {
	Namespace     string `json:"namespace"`
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
}

func (ci ContainerIdentifier) String() string {
	return ci.Namespace + "/" + ci.PodName + "/" + ci.ContainerName
}
