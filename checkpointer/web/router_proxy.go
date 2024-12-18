package web

import (
	"checkpoint-in-k8s/internal"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/http/httputil"
	"net/url"
)

const checkpointerLabelSelector = "app.kubernetes.io/name=checkpointer"

type ProxyCheckpointHandler struct {
	nodePodController internal.NodePodController
	checkpointerNode  string
	checkpointerPort  int64
}

func NewRouteProxyMiddleware(client *kubernetes.Clientset, config *rest.Config, checkpointerNode string, checkpointerPort int64) *ProxyCheckpointHandler {
	return &ProxyCheckpointHandler{
		internal.NewNodePodController(client, config),
		checkpointerNode,
		checkpointerPort,
	}
}

func (proxy *ProxyCheckpointHandler) CheckpointRouteProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		containerIdentifier := getContainerIdentifier(req)
		if containerIdentifier == nil {
			log.Info().Msg("malformed container identifier")
			http.Error(rw, "container path in format /{namespace}{pod}/{container} expected", http.StatusBadRequest)
			return
		}
		lg := log.With().Str("containerIdentifier", containerIdentifier.String()).Logger()

		lg.Debug().Msg("looking for a Node on which the container is running on")

		podsNodeName, err := proxy.nodePodController.GetNodeOfPod(req.Context(), containerIdentifier.Pod, containerIdentifier.Namespace)
		if err != nil {
			lg.Error().Err(err).Msg("error getting Pod's Node name")
			http.Error(rw, fmt.Sprintf("failed while looking for Node name of a Pod: %s", err), http.StatusInternalServerError)
			return
		}
		if podsNodeName == "" {
			lg.Info().Msg("pod does not exist")
			http.Error(rw, fmt.Sprintf("pod does not exist"), http.StatusNotFound)
			return
		}

		lg.Debug().Str("Node", podsNodeName).Msg("found Node of the Pod")

		proxy.findCheckpointerAndForward(rw, req, podsNodeName, next, lg)
	})
}

func (proxy *ProxyCheckpointHandler) StateRouteProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		node, _ := getCheckpointIdentifier(req)
		if node == "" {
			http.Error(rw, "query param checkpointIdentifier empty or malformed", http.StatusBadRequest)
			return
		}
		lg := log.With().Str("node", node).Logger()
		proxy.findCheckpointerAndForward(rw, req, node, next, lg)
	})
}

func (proxy *ProxyCheckpointHandler) findCheckpointerAndForward(rw http.ResponseWriter, req *http.Request, node string, next http.Handler, lg zerolog.Logger) {
	if proxy.checkpointerNode == node {
		log.Info().Msg("using local handler")
		next.ServeHTTP(rw, req)
		return
	}

	lg.Info().Msg("request is meant for another checkpointer, looking for it")
	maybeIp, err := proxy.nodePodController.GetPodIPForNode(req.Context(), node, checkpointerLabelSelector)
	if err != nil {
		lg.Error().Err(err).Msg("error getting Pod's IP address")
		http.Error(rw, fmt.Sprintf("failed while looking for a Pod to forward the request to %s", err), http.StatusInternalServerError)
		return
	}
	lg.Debug().Str("IP", maybeIp).Msg("found other checkpointer's IP")

	if maybeIp == "" {
		lg.Info().Msg("no Pod to forward the request to")
		http.Error(rw, fmt.Sprintf("no Pod to forward the request to"), http.StatusNotFound)
		return
	}

	podUrl, err := url.Parse(fmt.Sprintf("http://%s:%d", maybeIp, proxy.checkpointerPort))
	if err != nil {
		lg.Error().Err(err).Msg("error parsing Pod URL")
		http.Error(rw, fmt.Sprintf("failed to parse the Pod IP to forward the request to %s", err), http.StatusInternalServerError)
		return
	}
	lg.Info().Msg("forwarding request")
	reverseProxy := httputil.NewSingleHostReverseProxy(podUrl)
	reverseProxy.ServeHTTP(rw, req)
	lg.Info().Msg("request complete")
}
