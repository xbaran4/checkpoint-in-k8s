package internal

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

func (pc *PodController) CreatePod(ctx context.Context, pod *v1.Pod, namespace string) (string, error) {
	pod, err := pc.client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}
	return pod.GetName(), nil
}

func (pc *PodController) DeletePod(ctx context.Context, namespace, podName string) error {
	log.Printf("deleting pod %s/%s", namespace, podName)
	err := pc.client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete %s/%s pod: %w", namespace, podName, err)
	}
	return nil
}

func (pc *PodController) AttachAndStreamToPod(ctx context.Context, container, podName, namespace, buildContextTar string, timeout time.Duration) error {
	lg := zerolog.Ctx(ctx).With().
		Str("namespace", namespace).
		Str("pod", podName).
		Str("container", container).Logger()

	req := pc.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("attach").
		Param("container", container).
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false")

	lg.Debug().Msg("creating new executor")

	executor, err := remotecommand.NewSPDYExecutor(pc.config, "POST", req.URL())
	if err != nil {
		return err
	}

	buildContextFile, err := os.Open(buildContextTar)
	if err != nil {
		return err
	}
	defer buildContextFile.Close()

	err = pc.WaitForPodRunning(ctx, podName, namespace, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for pod to start: %w", err)
	}
	lg.Info().Msg("about to stream to container stdin...")

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  buildContextFile,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return fmt.Errorf("failed to stream to container stdin: %w", err)
	}

	err = pc.WaitForPodSucceeded(ctx, podName, namespace, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for pod to complete: %w", err)
	}
	return nil
}

func (pc *PodController) GetPodIPForNode(ctx context.Context, nodeName string) (string, error) {
	pods, err := pc.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=checkpointer",
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return "", fmt.Errorf("error listing pods on node %s: %w", nodeName, err)
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning && pod.Status.PodIP != "" {
			return pod.Status.PodIP, nil
		}
	}

	return "", nil
}

func (pc *PodController) GetNodeOfPod(ctx context.Context, podName, namespace string) (string, error) {
	pod, err := pc.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return pod.Spec.NodeName, nil
		}
		return "", fmt.Errorf("error getting pod %s/%s: %w", podName, namespace, err)
	}
	return pod.Spec.NodeName, nil
}

func (pc *PodController) WaitForPodRunning(ctx context.Context, podName, namespace string, timeout time.Duration) error {
	return pc.waitForPodPhase(ctx, podName, namespace, timeout, v1.PodRunning, v1.PodFailed, v1.PodSucceeded)
}

func (pc *PodController) WaitForPodSucceeded(ctx context.Context, podName, namespace string, timeout time.Duration) error {
	return pc.waitForPodPhase(ctx, podName, namespace, timeout, v1.PodSucceeded, v1.PodFailed)
}

func (pc *PodController) waitForPodPhase(
	ctx context.Context, podName, namespace string, timeout time.Duration,
	targetPhase v1.PodPhase, failurePhases ...v1.PodPhase) error {
	checkPodPhase := func(ctx context.Context) (bool, error) {
		zerolog.Ctx(ctx).Debug().Str("namespace", namespace).Str("podName", podName).Msg("polling for Pod phase...")
		pod, err := pc.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, phase := range failurePhases {
			if pod.Status.Phase == phase {
				return false, fmt.Errorf("pod reached unexpected phase: %s", phase)
			}
		}
		return pod.Status.Phase == targetPhase, nil
	}

	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, checkPodPhase)
}
