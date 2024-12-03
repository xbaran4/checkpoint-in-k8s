package checkpointer

import (
	"checkpoint-in-k8s/internal"
	"context"
	"fmt"
	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"os"
)

const kanikoContainerName = "kaniko"

type kanikoStdinCheckpointer struct {
	*internal.PodController
	checkpointerNamespace string
}

func NewKanikoStdinCheckpointer(client kubernetes.Interface, config *restclient.Config, checkpointerNamespace string) Checkpointer {
	return &kanikoStdinCheckpointer{internal.NewPodController(client, config), checkpointerNamespace}
}

func (cp *kanikoStdinCheckpointer) Checkpoint(ctx context.Context, cr CheckpointRequest) error {
	lg := zerolog.Ctx(ctx)

	lg.Debug().Msg("creating kaniko pod")
	kanikoPodName, err := cp.CreatePod(ctx, cp.getKanikoManifest(cr.ContainerImageName), cp.checkpointerNamespace)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %w", cr.ContainerIdentifier, err)
	}
	defer cp.DeletePod(context.WithoutCancel(ctx), cp.checkpointerNamespace, kanikoPodName)

	lg.Debug().Msg("calling Kubelet checkpointer")
	checkpointTarName, err := internal.CallKubeletCheckpoint(ctx, cr.ContainerIdentifier.String())
	if err != nil {
		return fmt.Errorf("could not checkpointer container: %s with error %w", cr.ContainerIdentifier, err)
	}
	defer os.Remove(checkpointTarName)
	lg.Debug().Str("tarName", checkpointTarName).Msg("successfully created checkpointer tar")

	if cr.DeletePod {
		if err := cp.DeletePod(ctx, cr.ContainerIdentifier.Namespace, cr.ContainerIdentifier.PodName); err != nil {
			lg.Warn().Err(err).Msg("could not delete pod") // TODO: fail on delete?
		}
		lg.Debug().Msg("successfully deleted checkpointed Pod")
	}

	filledDockerfileTemplate, err := internal.DockerfileFromTemplate(checkpointTarName)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %w", cr.ContainerIdentifier, err)
	}
	defer os.Remove(filledDockerfileTemplate)
	lg.Debug().Msg("successfully created new Dockerfile from template")

	buildContextTar, err := internal.CreateTarGzTempFile(map[string]string{
		filledDockerfileTemplate: "Dockerfile",
		checkpointTarName:        checkpointTarName,
	})

	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %w", cr.ContainerIdentifier, err)
	}
	defer os.Remove(buildContextTar)
	lg.Debug().Msg("successfully built tar.gz with build context")

	if err = cp.AttachAndStreamToPod(ctx, kanikoContainerName, kanikoPodName, cp.checkpointerNamespace, buildContextTar); err != nil {
		return fmt.Errorf("failed to attach to pod: %w", err)
	}

	lg.Debug().Msg("checkpointing done, about to cleanup resources")
	return nil
}

func (cp *kanikoStdinCheckpointer) getKanikoManifest(newContainerImageName string) *v1.Pod {
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
						"--destination=" + newContainerImageName,
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
							SecretName: "kaniko-secret", //TODO: take this from envar?
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
