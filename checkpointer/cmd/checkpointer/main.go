package main

import (
	"checkpoint-in-k8s/web"
	"errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
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

	// TODO: cumbersome design?
	proxyLessHandler := web.NewCheckpointHandler(clientset, config, true, false)
	var cpHandler handler = proxyLessHandler
	if os.Getenv("PROXY_FORWARD") != "" {
		cpHandler = web.NewProxyCheckpointHandler(clientset, config, proxyLessHandler, os.Getenv("NODE_NAME"))
	}

	http.HandleFunc("POST /checkpoint/{ns}/{pod}/{container}", cpHandler.HandleCheckpoint)
	http.HandleFunc("GET /checkpoint/{ns}/{pod}/{container}", cpHandler.HandleCheckState)

	log.Println("starting http server")
	err = http.ListenAndServe(":3333", nil)

	if errors.Is(err, http.ErrServerClosed) {
		log.Println("server closed")
	} else {
		log.Fatalf("error starting server: %s\n", err)
	}
}
