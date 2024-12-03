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
	"time"
)

type kanikoFSCheckpointer struct {
	*internal.PodController
	checkpointerNamespace string
}

func NewKanikoFSCheckpointer(client kubernetes.Interface, config *restclient.Config, checkpointerNamespace string) Checkpointer {
	return &kanikoFSCheckpointer{internal.NewPodController(client, config), checkpointerNamespace}
}

// Checkpoint Currently unused
func (cp *kanikoFSCheckpointer) Checkpoint(ctx context.Context, cr CheckpointRequest) error {
	lg := zerolog.Ctx(ctx)

	checkpointTarName, err := internal.CallKubeletCheckpoint(ctx, cr.ContainerIdentifier.String())
	if err != nil {
		return fmt.Errorf("could not checkpointer container: %s with error: %w", cr.ContainerIdentifier, err)
	}
	defer os.Remove(checkpointTarName)
	lg.Debug().Str("tarName", checkpointTarName).Msg("successfully created checkpointer tar")

	if cr.DeletePod {
		defer func() {
			if err := cp.DeletePod(ctx, cr.ContainerIdentifier.Namespace, cr.ContainerIdentifier.PodName); err != nil {
				lg.Warn().Err(err).Msg("could not delete pod")
			}
			lg.Debug().Msg("successfully deleted checkpointed Pod")
		}()
	}

	filledDockerfileTemplate, err := internal.DockerfileFromTemplate(checkpointTarName)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %w", cr.ContainerIdentifier, err)
	}
	defer os.Remove(filledDockerfileTemplate)
	lg.Debug().Msg("successfully created new Dockerfile from template")

	tmpDir, err := internal.PrepareDir(checkpointTarName, filledDockerfileTemplate)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %w", cr.ContainerIdentifier, err)
	}

	kanikoPodName, err := cp.CreatePod(ctx, cp.getKanikoManifest(cr.ContainerImageName, tmpDir), cp.checkpointerNamespace)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %w", cr.ContainerIdentifier, err)
	}
	defer cp.DeletePod(context.WithoutCancel(ctx), cp.checkpointerNamespace, kanikoPodName)

	err = cp.WaitForPodSucceeded(ctx, kanikoPodName, cp.checkpointerNamespace, time.Second*30) // TODO: make timeout env var?
	if err != nil {
		return fmt.Errorf("timed out waiting for Pod to complete: %w", err)
	}

	lg.Debug().Msg("checkpointing done, about to cleanup resources")
	return nil
}

func (cp *kanikoFSCheckpointer) getKanikoManifest(newContainerImageName, buildContextPath string) *v1.Pod {
	hostPathType := v1.HostPathDirectory
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kaniko-",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "kaniko",
					Image: "gcr.io/kaniko-project/executor:latest",
					Args: []string{
						"--dockerfile=/kaniko-build-context/Dockerfile",
						"--context=dir:///kaniko-build-context",
						"--destination=" + newContainerImageName,
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
							SecretName: "kaniko-secret",
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
