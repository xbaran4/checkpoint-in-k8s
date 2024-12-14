package main

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"checkpoint-in-k8s/pkg/config"
	"checkpoint-in-k8s/pkg/manager"
	"checkpoint-in-k8s/web"
	"errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strconv"

	"github.com/rs/zerolog/log"
	"net/http"
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

	globalConfig, err := config.LoadGlobalConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to bootstrap Checkpointer configuration")
	}

	mux := http.NewServeMux()

	cp, err := checkpoint.NewCheckpointer(clientset, inClusterConfig, globalConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create Checkpointer")
	}
	storage := manager.NewCheckpointStorage(globalConfig)
	mgr := manager.NewCheckpointManager(cp, storage)

	ch := web.NewCheckpointHandler(mgr, globalConfig.CheckpointConfig.CheckpointerNode)
	var checkpointHandler http.Handler = http.HandlerFunc(ch.HandleCheckpoint)
	var stateHandler http.Handler = http.HandlerFunc(ch.HandleCheckState)

	if !globalConfig.DisableRouteForward {
		proxy := web.NewRouteProxyMiddleware(
			clientset,
			inClusterConfig,
			globalConfig.CheckpointConfig.CheckpointerNode,
			globalConfig.CheckpointerPort,
		)
		checkpointHandler = proxy.CheckpointRouteProxyMiddleware(checkpointHandler)
		stateHandler = proxy.StateRouteProxyMiddleware(stateHandler)
	}

	mux.Handle("POST /checkpoint/{ns}/{pod}/{container}", checkpointHandler)
	mux.Handle("GET /checkpoint", stateHandler)

	portNumber := strconv.FormatInt(globalConfig.CheckpointerPort, 10)
	log.Info().Msg("starting http server on port: " + portNumber)
	err = http.ListenAndServe(":"+portNumber, mux)

	if errors.Is(err, http.ErrServerClosed) {
		log.Info().Msg("server closed")
	} else {
		log.Error().Err(err).Msg("error starting server")
	}
}
