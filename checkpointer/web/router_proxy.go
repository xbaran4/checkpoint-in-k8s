package web

import (
	"checkpoint-in-k8s/internal"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type ProxyCheckpointHandler struct {
	checkpointHandler *CheckpointHandler
	podController     *internal.PodController
	myNodeName        string
}

// NewProxyCheckpointHandler TODO: make as middleware
func NewProxyCheckpointHandler(client *kubernetes.Clientset, config *rest.Config, cpHandler *CheckpointHandler, myNodeName string) *ProxyCheckpointHandler {
	return &ProxyCheckpointHandler{
		checkpointHandler: cpHandler,
		podController:     internal.NewPodController(client, config),
		myNodeName:        myNodeName,
	}
}

func (proxy *ProxyCheckpointHandler) HandleCheckpoint(rw http.ResponseWriter, req *http.Request) {
	proxy.doHandle(rw, req, proxy.checkpointHandler.HandleCheckpoint)
}

func (proxy *ProxyCheckpointHandler) HandleCheckState(rw http.ResponseWriter, req *http.Request) {
	proxy.doHandle(rw, req, proxy.checkpointHandler.HandleCheckState)
}

func (proxy *ProxyCheckpointHandler) doHandle(rw http.ResponseWriter, req *http.Request, localHandler func(http.ResponseWriter, *http.Request)) {
	containerIdentifier := getContainerIdentifier(req)
	if containerIdentifier == nil {
		http.Error(rw, "container path in format /{namespace}{pod}/{container} expected", http.StatusBadRequest)
		return
	}
	log.Printf("looking for a Node where container %s is running on", containerIdentifier)

	podsNodeName, err := proxy.podController.GetNodeOfPod(containerIdentifier.PodName, containerIdentifier.Namespace)
	if err != nil {
		log.Printf("Error getting container %s node name: %s", containerIdentifier, err)
		http.Error(rw, fmt.Sprintf("failed while looking for Node name of a Pod: %s", err), http.StatusInternalServerError)
		return
	}
	log.Printf("container %s is running on %s node", containerIdentifier, podsNodeName)

	if proxy.myNodeName == podsNodeName {
		log.Printf("using local handler")
		localHandler(rw, req)
		return
	}

	log.Printf("looking for a checkpointer on other node")
	maybeIp, err := proxy.podController.GetPodIPForNode(podsNodeName)
	if err != nil {
		http.Error(rw, fmt.Sprintf("failed while looking for a Pod to forward the request to %s", err), http.StatusInternalServerError)
		return
	}

	log.Printf("other checkpointer's IP: %s", maybeIp)
	if maybeIp != "" {
		podUrl, err := url.Parse("http://" + maybeIp + ":3333")
		if err != nil {
			http.Error(rw, fmt.Sprintf("failed to parse the Pod IP to forward the request to %s", err), http.StatusInternalServerError)
			return
		}
		log.Printf("forwarding request")
		proxy := httputil.NewSingleHostReverseProxy(podUrl)
		proxy.ServeHTTP(rw, req)
		return
	}

	http.Error(rw, fmt.Sprintf("no Pod to forward the request to"), http.StatusNotFound)
}
