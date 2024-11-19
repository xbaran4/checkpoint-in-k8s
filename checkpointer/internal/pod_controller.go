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

type PodController struct {
	client kubernetes.Interface
	config *restclient.Config
}

func NewPodController(client kubernetes.Interface, config *restclient.Config) *PodController {
	return &PodController{client, config}
}

func (pc *PodController) CreatePod(pod *v1.Pod, namespace string) (string, error) {
	pod, err := pc.client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}
	return pod.GetName(), nil
}

func (pc *PodController) DeletePod(namespace, podName string) error {
	log.Printf("deleting pod %s/%s", namespace, podName)
	err := pc.client.CoreV1().Pods(namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete %s/%s pod: %w", namespace, podName, err)
	}
	return nil
}

func (pc *PodController) AttachAndStreamToPod(podName, namespace, buildContextTar string) error {
	req := pc.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("attach").
		Param("container", "kaniko").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false")

	log.Printf("creating new executor with %s", req.URL())

	executor, err := remotecommand.NewSPDYExecutor(pc.config, "POST", req.URL())
	if err != nil {
		return err
	}

	buildContextFile, err := os.Open(buildContextTar)
	if err != nil {
		return err
	}
	defer buildContextFile.Close()

	err = pc.waitForPodRunning(podName, namespace, time.Second*30) // TODO: make timeout env var?
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

	// TODO: check kaniko pod completed without fail
	return err
}

// TODO: improve error handling
func (pc *PodController) GetPodIPForNode(nodeName string) (string, error) {
	pods, err := pc.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=checkpointer",
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return "", fmt.Errorf("error listing pods on node %s: %s", nodeName, err)
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning && pod.Status.PodIP != "" {
			return pod.Status.PodIP, nil
		}
	}

	return "", nil
}

func (pc *PodController) GetNodeOfPod(podName, namespace string) (string, error) {
	pod, err := pc.client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error getting pod  %s/%s", podName, namespace)
	}
	return pod.Spec.NodeName, nil
}

func (pc *PodController) waitForPodRunning(podName, namespace string, timeout time.Duration) error {
	isPodRunning := func(ctx context.Context) (bool, error) {
		log.Printf("waiting for %s pod to be running...", podName)

		pod, err := pc.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
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
