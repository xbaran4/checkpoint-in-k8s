package main

import (
	"checkpoint-in-k8s/web"
	"errors"
	"github.com/rs/zerolog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/rs/zerolog/log"
	"net/http"
	"os"
)

var clientset *kubernetes.Clientset
var config *rest.Config

type handler interface {
	HandleCheckpoint(http.ResponseWriter, *http.Request)
	HandleCheckState(http.ResponseWriter, *http.Request)
}

func main() {
	var err error
	config, err = rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// set plaintext logs for better dev experience
	if os.Getenv("ENVIRONMENT") != "prod" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	mux := http.NewServeMux()
	ch := web.NewCheckpointHandler(clientset, config, true, false)
	var checkpointHandle http.Handler = http.HandlerFunc(ch.HandleCheckpoint)
	var stateHandle http.Handler = http.HandlerFunc(ch.HandleCheckState)

	if os.Getenv("DISABLE_ROUTE_FORWARD") == "" {
		proxy := web.NewProxyCheckpointHandler(clientset, config, os.Getenv("NODE_NAME"))
		checkpointHandle = proxy.RouteProxyMiddleware(checkpointHandle)
		stateHandle = proxy.RouteProxyMiddleware(stateHandle)
	}

	mux.Handle("POST /checkpoint/{ns}/{pod}/{container}", checkpointHandle)
	mux.Handle("GET /checkpoint/{ns}/{pod}/{container}", stateHandle)

	log.Info().Msg("starting http server on port 3333")
	err = http.ListenAndServe(":3333", mux)

	if errors.Is(err, http.ErrServerClosed) {
		log.Info().Msg("server closed")
	} else {
		log.Error().Err(err).Msg("error starting server")
	}
}
