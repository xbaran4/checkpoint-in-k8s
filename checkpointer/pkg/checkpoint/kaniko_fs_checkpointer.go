package checkpoint

import (
	"checkpoint-in-k8s/internal"
	"checkpoint-in-k8s/pkg/config"
	"context"
	"fmt"
	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"time"
)

// kanikoFSCheckpointer represents the Kaniko File system strategy of checkpointing.
type kanikoFSCheckpointer struct {

	// PodController is used to manipulate with Kubernetes Pods.
	internal.PodController

	// KubeletController is used to request checkpoint from Kubelet.
	internal.KubeletController

	// DockerfileFactory is used to generate Dockerfile for checkpoint image.
	internal.DockerfileFactory

	// CheckpointConfig contains configuration settings influencing checkpointing.
	config.CheckpointConfig
}

func newKanikoFSCheckpointer(podController internal.PodController,
	kubeletController internal.KubeletController, dockerfileFactory internal.DockerfileFactory, checkpointConfig config.CheckpointConfig) Checkpointer {
	return &kanikoFSCheckpointer{
		podController,
		kubeletController,
		dockerfileFactory,
		checkpointConfig,
	}
}

func (cp *kanikoFSCheckpointer) Checkpoint(ctx context.Context, params CheckpointerParams) (string, error) {
	lg := zerolog.Ctx(ctx)
	checkpointImageName := cp.CheckpointImagePrefix + ":" + params.CheckpointIdentifier

	checkpointTarName, err := cp.CallKubeletCheckpoint(ctx, params.ContainerIdentifier.String())
	if err != nil {
		return "", fmt.Errorf("could not checkpointer container: %s with error: %w", params.ContainerIdentifier, err)
	}
	defer os.Remove(checkpointTarName)
	lg.Debug().Str("tarName", checkpointTarName).Msg("successfully created checkpointer tar")

	filledDockerfileTemplate, err := cp.DockerfileFromTemplate(cp.CheckpointBaseImage, checkpointTarName)
	if err != nil {
		return "", fmt.Errorf("could not create checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer os.Remove(filledDockerfileTemplate)
	lg.Debug().Msg("successfully created new Dockerfile from template")

	buildContextDir, err := internal.PrepareKanikoBuildContext(cp.KanikoBuildContextDir, checkpointTarName, filledDockerfileTemplate)
	if err != nil {
		return "", fmt.Errorf("could not create checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer os.RemoveAll(buildContextDir)

	kanikoPodName, err := cp.CreatePod(ctx, cp.getKanikoManifest(checkpointImageName, buildContextDir), cp.CheckpointerNamespace)
	if err != nil {
		return "", fmt.Errorf("could not create checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer cp.DeletePod(context.WithoutCancel(ctx), cp.CheckpointerNamespace, kanikoPodName)

	err = cp.WaitForPodSucceeded(ctx, kanikoPodName, cp.CheckpointerNamespace, time.Second*time.Duration(cp.KanikoTimeoutSeconds))
	if err != nil {
		return "", fmt.Errorf("timed out waiting for Pod to complete: %w", err)
	}

	if params.DeletePod {
		if err := cp.DeleteAndWaitForRemoval(ctx, params.ContainerIdentifier.Namespace, params.ContainerIdentifier.Pod, time.Second*10); err != nil {
			lg.Warn().Err(err).Msg("could not delete checkpointed pod") // Do not fail if we cannot delete the Pod.
		}
		lg.Debug().Msg("successfully deleted checkpointed Pod")
	}

	lg.Debug().Msg("checkpointing done, about to cleanup resources")
	return checkpointImageName, nil
}

func (cp *kanikoFSCheckpointer) getKanikoManifest(checkpointImageName, buildContextPath string) *v1.Pod {
	hostPathType := v1.HostPathDirectory
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kaniko-",
		},
		Spec: v1.PodSpec{
			NodeName: cp.CheckpointerNode,
			Containers: []v1.Container{
				{
					Name:  "kaniko",
					Image: "gcr.io/kaniko-project/executor:latest",
					Args: []string{
						"--dockerfile=/kaniko-build-context/Dockerfile",
						"--context=dir:///kaniko-build-context",
						"--destination=" + checkpointImageName,
					},
					Stdin:     true,
					StdinOnce: true,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "kaniko-secret",
							MountPath: "/kaniko/.docker",
						},
						{
							Name:      "build-context",
							MountPath: "/kaniko-build-context",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "kaniko-secret",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: cp.KanikoSecretName,
							Items: []v1.KeyToPath{
								{
									Key:  ".dockerconfigjson",
									Path: "config.json",
								},
							},
						},
					},
				},
				{
					Name: "build-context",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Type: &hostPathType,
							Path: buildContextPath,
						},
					},
				},
			},
		},
	}
	return pod
}
