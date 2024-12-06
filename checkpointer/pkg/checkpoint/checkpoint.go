package checkpoint

import (
	"checkpoint-in-k8s/pkg/config"
	"context"
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

func NewCheckpointer(client *kubernetes.Clientset, config *rest.Config, checkpointerConfig config.CheckpointerConfig) Checkpointer {
	if checkpointerConfig.UseKanikoFS {
		return newKanikoFSCheckpointer(
			client,
			config,
			checkpointerConfig.KubeletPort,
			checkpointerConfig.KanikoSecretName,
			checkpointerConfig.CheckpointerNamespace,
			checkpointerConfig.CheckpointerNode,
			checkpointerConfig.CheckpointImagePrefix,
			checkpointerConfig.CheckpointBaseImage,
		)
	}
	return newKanikoStdinCheckpointer(
		client,
		config,
		checkpointerConfig.KubeletPort,
		checkpointerConfig.KanikoSecretName,
		checkpointerConfig.CheckpointerNamespace,
		checkpointerConfig.CheckpointImagePrefix,
		checkpointerConfig.CheckpointBaseImage,
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
