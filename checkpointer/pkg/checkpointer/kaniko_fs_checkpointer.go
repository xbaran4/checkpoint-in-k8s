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

type kanikoFSCheckpointer struct {
	*internal.PodController
	checkpointerNamespace string
}

func NewKanikoFSCheckpointer(client kubernetes.Interface, config *restclient.Config, checkpointerNamespace string) Checkpointer {
	return &kanikoFSCheckpointer{internal.NewPodController(client, config), checkpointerNamespace}
}

// Checkpoint Currently unused
func (cp *kanikoFSCheckpointer) Checkpoint(req CheckpointRequest) error {
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

	tmpDir, err := internal.PrepareDir(checkpointTarName, filledDockerfileTemplate)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %s", req.ContainerIdentifier, err)
	}

	_, err = cp.CreatePod(cp.getKanikoManifest(req.ContainerImageName, tmpDir), cp.checkpointerNamespace)
	if err != nil {
		return fmt.Errorf("could not create checkpointer container: %s with error %s", req.ContainerIdentifier, err)
	}
	// TODO: wait for kaniko to finish.
	// defer cp.DeletePod(cp.checkpointerNamespace, kanikoPodName)

	log.Printf("checkpointing done, about to cleanup resources")
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
