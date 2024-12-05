package web

import (
	"checkpoint-in-k8s/internal"
	"fmt"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type ProxyCheckpointHandler struct {
	podController    *internal.PodController
	checkpointerNode string
}

func NewProxyCheckpointHandler(client *kubernetes.Clientset, config *rest.Config, checkpointerNode string) *ProxyCheckpointHandler {
	return &ProxyCheckpointHandler{
		podController:    internal.NewPodController(client, config),
		checkpointerNode: checkpointerNode,
	}
}

func (proxy *ProxyCheckpointHandler) RouteProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		containerIdentifier := getContainerIdentifier(req)
		if containerIdentifier == nil {
			log.Info().Msg("malformed container identifier")
			http.Error(rw, "container path in format /{namespace}{pod}/{container} expected", http.StatusBadRequest)
			return
		}
		lg := log.With().Str("containerIdentifier", containerIdentifier.String()).Logger()

		lg.Debug().Msg("looking for a Node on which the container is running on")

		podsNodeName, err := proxy.podController.GetNodeOfPod(req.Context(), containerIdentifier.PodName, containerIdentifier.Namespace)
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

		if proxy.checkpointerNode == podsNodeName {
			log.Info().Msg("using local handler")
			next.ServeHTTP(rw, req)
			return
		}

		lg.Info().Msg("request is meant for another checkpointer, looking for it")
		maybeIp, err := proxy.podController.GetPodIPForNode(req.Context(), podsNodeName)
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

		podUrl, err := url.Parse("http://" + maybeIp + ":3333")
		if err != nil {
			lg.Error().Err(err).Msg("error parsing Pod URL")
			http.Error(rw, fmt.Sprintf("failed to parse the Pod IP to forward the request to %s", err), http.StatusInternalServerError)
			return
		}
		lg.Info().Msg("forwarding request")
		reverseProxy := httputil.NewSingleHostReverseProxy(podUrl)
		reverseProxy.ServeHTTP(rw, req)
		lg.Info().Msg("request complete")
	})
}
