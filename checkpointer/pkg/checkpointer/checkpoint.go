package checkpointer

import "context"

type CheckpointRequest struct {
	ContainerIdentifier ContainerIdentifier
	ContainerImageName  string `json:"containerImageName"`
	DeletePod           bool   `json:"deletePod"`
}

type Checkpointer interface {
	Checkpoint(ctx context.Context, cr CheckpointRequest) error
}

type ContainerIdentifier struct {
	Namespace     string
	PodName       string
	ContainerName string
}

func (ci ContainerIdentifier) String() string {
	return ci.Namespace + "/" + ci.PodName + "/" + ci.ContainerName
}
