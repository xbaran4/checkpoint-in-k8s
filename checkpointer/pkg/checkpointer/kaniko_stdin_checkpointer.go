package checkpointer

import (
	"checkpoint-in-k8s/internal"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"log"
	"os"
)

type kanikoStdinCheckpointer struct {
	*internal.PodController
	checkpointerNamespace string
}

func NewKanikoStdinCheckpointer(client kubernetes.Interface, config *restclient.Config, checkpointerNamespace string) Checkpointer {
	return &kanikoStdinCheckpointer{internal.NewPodController(client, config), checkpointerNamespace}
}

func (cp *kanikoStdinCheckpointer) Checkpoint(req CheckpointRequest) error {
	log.Printf("creating kaniko pod")
	kanikoPodName, err := cp.CreatePod(cp.getKanikoManifest(req.ContainerImageName), cp.checkpointerNamespace)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %s", req.ContainerIdentifier, err)
	}
	defer cp.DeletePod(cp.checkpointerNamespace, kanikoPodName)
	log.Printf("successfully created kaniko pod")

	log.Printf("calling Kubelet checkpointer")
	checkpointTarName, err := internal.CallKubeletCheckpoint(req.ContainerIdentifier.String())
	if err != nil {
		return fmt.Errorf("could not checkpointer container: %s with error %s", req.ContainerIdentifier, err)
	}
	defer os.Remove(checkpointTarName)
	log.Printf("successfully created checkpointer tar: %s", checkpointTarName)

	if req.DeletePod {
		if err := cp.DeletePod(req.ContainerIdentifier.Namespace, req.ContainerIdentifier.PodName); err != nil {
			log.Printf("could not delete pod: %s with error %s", req.ContainerIdentifier, err) // TODO: fail on delete?
		}
		log.Printf("successfully deleted pod %s/%s", req.ContainerIdentifier.Namespace, req.ContainerIdentifier.PodName)
	}

	filledDockerfileTemplate, err := internal.DockerfileFromTemplate(checkpointTarName)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %s", req.ContainerIdentifier, err)
	}
	defer os.Remove(filledDockerfileTemplate)
	log.Printf("successfully created new Dockerfile from template")

	buildContextTar, err := internal.CreateTarGzTempFile(map[string]string{
		filledDockerfileTemplate: "Dockerfile",
		checkpointTarName:        checkpointTarName,
	})

	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %s", req.ContainerIdentifier, err)
	}
	defer os.Remove(buildContextTar)
	log.Printf("successfully built tar.gz with buildcontext")

	if err = cp.AttachAndStreamToPod(kanikoPodName, cp.checkpointerNamespace, buildContextTar); err != nil {
		return fmt.Errorf("failed to attach to pod: %v\n", err)
	}

	log.Printf("checkpointing done, about to cleanup resources")
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
					Name:  "kaniko",
					Image: "gcr.io/kaniko-project/executor:debug",
					Args: []string{
						"--dockerfile=Dockerfile",
						"--context=tar://stdin",
						"--destination=" + newContainerImageName,
						"--label=\"org.criu.checkpointer.container.name=value\"",
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
