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

const kanikoContainerName = "kaniko"

type kanikoStdinCheckpointer struct {
	*internal.PodController
	kubeletController internal.KubeletController
	config.CheckpointConfig
}

func newKanikoStdinCheckpointer(podController *internal.PodController,
	kubeletController internal.KubeletController,
	checkpointConfig config.CheckpointConfig) Checkpointer {
	return &kanikoStdinCheckpointer{
		podController,
		kubeletController,
		checkpointConfig,
	}
}

func (cp *kanikoStdinCheckpointer) Checkpoint(ctx context.Context, params CheckpointerParams) (string, error) {
	lg := zerolog.Ctx(ctx)
	checkpointImageName := cp.CheckpointImagePrefix + ":" + params.CheckpointIdentifier

	lg.Debug().Msg("creating kaniko pod")
	kanikoPodName, err := cp.CreatePod(ctx, cp.getKanikoManifest(checkpointImageName), cp.CheckpointerNamespace)
	if err != nil {
		return "", fmt.Errorf("could not create checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer cp.DeletePod(context.WithoutCancel(ctx), cp.CheckpointerNamespace, kanikoPodName)

	lg.Debug().Msg("calling Kubelet checkpointer")
	checkpointTarName, err := cp.kubeletController.CallKubeletCheckpoint(ctx, params.ContainerIdentifier.String())
	if err != nil {
		return "", fmt.Errorf("could not checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer os.Remove(checkpointTarName)
	lg.Debug().Str("tarName", checkpointTarName).Msg("successfully created checkpointer tar")

	filledDockerfileTemplate, err := internal.DockerfileFromTemplate(cp.CheckpointBaseImage, checkpointTarName)
	if err != nil {
		return "", fmt.Errorf("could not create checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer os.Remove(filledDockerfileTemplate)
	lg.Debug().Msg("successfully created new Dockerfile from template")

	buildContextTar, err := internal.CreateTarGzTempFile(map[string]string{
		filledDockerfileTemplate: "Dockerfile",
		checkpointTarName:        checkpointTarName,
	})

	if err != nil {
		return "", fmt.Errorf("could not create checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer os.Remove(buildContextTar)
	lg.Debug().Msg("successfully built tar.gz with build context")

	if err = cp.AttachAndStreamToPod(ctx,
		kanikoContainerName,
		kanikoPodName,
		cp.CheckpointerNamespace,
		buildContextTar,
		time.Second*time.Duration(cp.KanikoTimeoutSeconds),
	); err != nil {
		return "", fmt.Errorf("failed to attach to pod: %w", err)
	}

	if params.DeletePod {
		defer func() {
			if err := cp.DeletePod(ctx, params.ContainerIdentifier.Namespace, params.ContainerIdentifier.Pod); err != nil {
				lg.Warn().Err(err).Msg("could not delete checkpointed pod")
			}
			lg.Debug().Msg("successfully deleted checkpointed Pod")
		}()
	}

	lg.Debug().Msg("checkpointing done, about to cleanup resources")
	return checkpointImageName, nil
}

func (cp *kanikoStdinCheckpointer) getKanikoManifest(checkpointImageName string) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kaniko-",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  kanikoContainerName,
					Image: "gcr.io/kaniko-project/executor:latest",
					Args: []string{
						"--dockerfile=Dockerfile",
						"--context=tar://stdin",
						"--destination=" + checkpointImageName,
					},
					Stdin:     true,
					StdinOnce: true,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "kaniko-secret",
							MountPath: "/kaniko/.docker",
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
			},
		},
	}

	return pod
}
