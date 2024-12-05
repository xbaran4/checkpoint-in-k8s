package main

import (
	"checkpoint-in-k8s/pkg/config"
	"checkpoint-in-k8s/web"
	"errors"
	"github.com/rs/zerolog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/rs/zerolog/log"
	"net/http"
	"os"
)

func main() {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get Kubernetes in-cluster inClusterConfig")
	}

	clientset, err := kubernetes.NewForConfig(inClusterConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get Kubernetes clientset")
	}

	checkpointerConfig, err := config.LoadCheckpointerConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to bootstrap Checkpointer configuration")
	}

	// set plaintext logs for better dev experience
	if checkpointerConfig.Environment == config.DevelopmentEnvironment {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	mux := http.NewServeMux()
	ch := web.NewCheckpointHandler(clientset, inClusterConfig, checkpointerConfig)
	var checkpointHandler http.Handler = http.HandlerFunc(ch.HandleCheckpoint)
	var stateHandler http.Handler = http.HandlerFunc(ch.HandleCheckState)

	if !checkpointerConfig.DisableRouteForward {
		proxy := web.NewRouteProxyMiddleware(clientset, inClusterConfig, checkpointerConfig.CheckpointerNode)
		checkpointHandler = proxy.RouteProxyMiddleware(checkpointHandler)
		stateHandler = proxy.RouteProxyMiddleware(stateHandler)
	}

	mux.Handle("POST /checkpoint/{ns}/{pod}/{container}", checkpointHandler)
	mux.Handle("GET /checkpoint/{ns}/{pod}/{container}", stateHandler)

	log.Info().Msg("starting http server on port 3333")
	err = http.ListenAndServe(":3333", mux)

	if errors.Is(err, http.ErrServerClosed) {
		log.Info().Msg("server closed")
	} else {
		log.Error().Err(err).Msg("error starting server")
	}
}
