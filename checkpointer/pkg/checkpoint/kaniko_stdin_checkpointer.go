package checkpoint

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
	kubeletPort           int64
	kanikoSecretName      string
	checkpointerNamespace string
	checkpointImagePrefix string
	checkpointImageBase   string
}

func newKanikoStdinCheckpointer(client kubernetes.Interface,
	config *restclient.Config,
	kubeletPort int64,
	kanikoSecretName,
	checkpointerNamespace,
	checkpointImagePrefix,
	checkpointImageBase string) Checkpointer {
	return &kanikoStdinCheckpointer{
		internal.NewPodController(client, config),
		kubeletPort,
		kanikoSecretName,
		checkpointerNamespace,
		checkpointImagePrefix,
		checkpointImageBase,
	}
}

func (cp *kanikoStdinCheckpointer) Checkpoint(ctx context.Context, params CheckpointerParams) (string, error) {
	lg := zerolog.Ctx(ctx)
	checkpointImageName := cp.checkpointImagePrefix + ":" + params.CheckpointIdentifier

	lg.Debug().Msg("creating kaniko pod")
	kanikoPodName, err := cp.CreatePod(ctx, cp.getKanikoManifest(checkpointImageName), cp.checkpointerNamespace)
	if err != nil {
		return "", fmt.Errorf("could not create checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer cp.DeletePod(context.WithoutCancel(ctx), cp.checkpointerNamespace, kanikoPodName)

	lg.Debug().Msg("calling Kubelet checkpointer")
	checkpointTarName, err := internal.CallKubeletCheckpoint(ctx, cp.kubeletPort, params.ContainerIdentifier.String())
	if err != nil {
		return "", fmt.Errorf("could not checkpointer container: %s with error %w", params.ContainerIdentifier, err)
	}
	defer os.Remove(checkpointTarName)
	lg.Debug().Str("tarName", checkpointTarName).Msg("successfully created checkpointer tar")

	if params.DeletePod {
		if err := cp.DeletePod(ctx, params.ContainerIdentifier.Namespace, params.ContainerIdentifier.PodName); err != nil {
			lg.Warn().Err(err).Msg("could not delete pod") // TODO: fail on delete?
		}
		lg.Debug().Msg("successfully deleted checkpointed Pod")
	}

	filledDockerfileTemplate, err := internal.DockerfileFromTemplate(cp.checkpointImageBase, checkpointTarName)
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

	if err = cp.AttachAndStreamToPod(ctx, kanikoContainerName, kanikoPodName, cp.checkpointerNamespace, buildContextTar); err != nil {
		return "", fmt.Errorf("failed to attach to pod: %w", err)
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
							SecretName: cp.kanikoSecretName,
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
