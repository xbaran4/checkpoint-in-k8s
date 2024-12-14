package internal

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"io"
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

// PodController is responsible for using the Kubernetes API to manipulate Kubernetes Pods.
type PodController interface {

	// CreatePod creates Kubernetes Pod based on pod manifest in namespace. Returns the name of the Pod or error.
	CreatePod(ctx context.Context, pod *v1.Pod, namespace string) (string, error)

	// DeletePod deletes Kubernetes Pod with podName in namespace. Returns error if a call to Kubernetes API fails.
	DeletePod(ctx context.Context, namespace, podName string) error

	// AttachAndStreamToContainer attaches to a container within podName and streams the content from reader. Before
	// streaming, it waits for the Pod to reach Running phase and after streaming waits for Succeeded phase. The timeout
	// parameter defines how long it will wait until failing. Returns error if any of the Kubernetes API calls fails or
	// if timed-out waiting for Pod.
	AttachAndStreamToContainer(ctx context.Context, container, podName, namespace string, reader io.Reader, timeout time.Duration) error

	// WaitForPodRunning wait until podName in namespace is in Running phase. Returns an error if timeout is exceeded or
	// a call to Kubernetes API fails.
	WaitForPodRunning(ctx context.Context, podName, namespace string, timeout time.Duration) error

	// WaitForPodSucceeded wait until podName in namespace is in Succeeded phase. Returns an error if timeout is exceeded or
	// a call to Kubernetes API fails.
	WaitForPodSucceeded(ctx context.Context, podName, namespace string, timeout time.Duration) error

	// DeleteAndWaitForRemoval deletes a podName in namespace and waits until timeout for Kubernetes API to no longer
	// return the Pod. Returns error if any of the Kubernetes API calls fails or timeout is reached.
	DeleteAndWaitForRemoval(
		ctx context.Context, podName, namespace string, timeout time.Duration) error
}

// NodePodController is responsible for using the Kubernetes API to fetch Node and Pod metadata.
type NodePodController interface {

	// GetPodIPForNode finds a Pod based on nodeName and labelSelector and returns the Pod's IP address. If no such Pod is
	// found, returns empty string instead. Error is returned in case of failed call to Kubernetes API.
	GetPodIPForNode(ctx context.Context, nodeName, labelSelector string) (string, error)

	// GetNodeOfPod returns the name of the Node that the Pod is running on or error a call to Kubernetes API fails.
	// If the Pod does not exist returns empty string and nil error.
	GetNodeOfPod(ctx context.Context, podName, namespace string) (string, error)
}

type podController struct {
	client kubernetes.Interface
	config *restclient.Config
}

func NewPodController(client kubernetes.Interface, config *restclient.Config) PodController {
	return &podController{client, config}
}

func NewNodePodController(client kubernetes.Interface, config *restclient.Config) NodePodController {
	return &podController{client, config}
}

func (pc *podController) CreatePod(ctx context.Context, pod *v1.Pod, namespace string) (string, error) {
	pod, err := pc.client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}
	return pod.GetName(), nil
}

func (pc *podController) DeletePod(ctx context.Context, namespace, podName string) error {
	log.Printf("deleting pod %s/%s", namespace, podName)
	err := pc.client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete %s/%s pod: %w", namespace, podName, err)
	}
	return nil
}

func (pc *podController) AttachAndStreamToContainer(
	ctx context.Context,
	container, podName, namespace string,
	reader io.Reader,
	timeout time.Duration,
) error {

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

	err = pc.WaitForPodRunning(ctx, podName, namespace, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for pod to start: %w", err)
	}
	lg.Info().Msg("about to stream to container stdin...")

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  reader,
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

func (pc *podController) GetPodIPForNode(ctx context.Context, nodeName, labelSelector string) (string, error) {
	pods, err := pc.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
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

func (pc *podController) GetNodeOfPod(ctx context.Context, podName, namespace string) (string, error) {
	pod, err := pc.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("error getting pod %s/%s: %w", podName, namespace, err)
	}
	return pod.Spec.NodeName, nil
}

func (pc *podController) WaitForPodRunning(ctx context.Context, podName, namespace string, timeout time.Duration) error {
	return pc.waitForPodPhase(ctx, podName, namespace, timeout, v1.PodRunning, v1.PodFailed, v1.PodSucceeded)
}

func (pc *podController) WaitForPodSucceeded(ctx context.Context, podName, namespace string, timeout time.Duration) error {
	return pc.waitForPodPhase(ctx, podName, namespace, timeout, v1.PodSucceeded, v1.PodFailed)
}

func (pc *podController) waitForPodPhase(
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

func (pc *podController) DeleteAndWaitForRemoval(
	ctx context.Context, podName, namespace string, timeout time.Duration) error {

	err := pc.DeletePod(ctx, podName, namespace)
	if err != nil {
		return err
	}

	checkPodDeleted := func(ctx context.Context) (bool, error) {
		zerolog.Ctx(ctx).Debug().Str("namespace", namespace).Str("podName", podName).Msg("polling for Pod deletion...")
		_, err := pc.client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("error getting pod %s/%s: %w", podName, namespace, err)
		}
		return false, nil
	}

	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, checkPodDeleted)
}
