package internal

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"os"
	"time"
)

const checkpointerNamespace = "kube-system"

func CreateKanikoPod(c kubernetes.Interface, newContainerImageName string) (string, error) {
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
						"--dockerfile=Dockerfile",
						"--context=tar://stdin",
						"--destination=" + newContainerImageName,
						"--label=\"org.criu.checkpoint.container.name=value\"",
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

	pod, err := c.CoreV1().Pods(checkpointerNamespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}

	return pod.GetName(), nil
}

func DeleteKanikoPod(clientset *kubernetes.Clientset, podName string) error {
	return DeletePod(clientset, checkpointerNamespace, podName)
}

func DeletePod(clientset *kubernetes.Clientset, namespace, podName string) error {
	log.Printf("deleting pod %s/%s", namespace, podName)
	err := clientset.CoreV1().Pods(namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete %s/%s pod: %w", namespace, podName, err)
	}
	return nil
}

func AttachToPod(clientset *kubernetes.Clientset, config *restclient.Config, podName, buildContextTar string) error {
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(checkpointerNamespace).
		SubResource("attach").
		Param("container", "kaniko").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false")

	log.Printf("creating new executor with %s", req.URL())

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	buildContextFile, err := os.Open(buildContextTar)
	if err != nil {
		return err
	}
	defer buildContextFile.Close()

	err = waitForPodRunning(clientset, podName, time.Second*30) // TODO: make timeout env var
	if err != nil {
		return fmt.Errorf("timed out waiting for kaniko to start: %w", err)
	}
	log.Printf("about to stream to kaniko stdin...")

	err = executor.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:  buildContextFile,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})

	return err
}

func waitForPodRunning(c kubernetes.Interface, podName string, timeout time.Duration) error {
	isPodRunning := func(ctx context.Context) (bool, error) {
		log.Printf("waiting for %s pod to be running...", podName)

		pod, err := c.CoreV1().Pods(checkpointerNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		switch pod.Status.Phase {
		case v1.PodRunning:
			return true, nil
		case v1.PodFailed, v1.PodSucceeded:
			return false, fmt.Errorf("pod failed or completed")
		}
		return false, nil
	}

	return wait.PollUntilContextTimeout(context.TODO(), time.Second, timeout, true, isPodRunning)
}
