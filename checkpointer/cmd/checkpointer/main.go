package main

import (
	"checkpoint-in-k8s/web"
	"errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
)

var clientset *kubernetes.Clientset
var config *rest.Config

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

	cpHandler := web.NewCheckpointHandler(clientset, config)
	http.HandleFunc("POST /checkpoint", cpHandler.HandleCheckpointAsync)
	http.HandleFunc("GET /checkpoint", cpHandler.HandleCheckState)

	log.Println("starting http server")
	err = http.ListenAndServe(":3333", nil)

	if errors.Is(err, http.ErrServerClosed) {
		log.Println("server closed")
	} else {
		log.Fatalf("error starting server: %s\n", err)
	}
}
