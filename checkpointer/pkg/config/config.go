package config

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
)

type Environment uint8

const (
	ProductionEnvironment Environment = iota
	DevelopmentEnvironment
)

type CheckpointerConfig struct {
	CheckpointerNamespace string
	CheckpointerNode      string
	CheckpointImageBase   string
	KubeletPort           string
	KanikoSecretName      string
	DisableRouteForward   bool
	UseKanikoFS           bool
	Environment           Environment
}

func LoadCheckpointerConfig() (CheckpointerConfig, error) {
	var err error
	config := CheckpointerConfig{}

	config.CheckpointImageBase = os.Getenv("CP_IMAGE_BASE")
	if config.CheckpointImageBase == "" {
		err = errors.Join(err, fmt.Errorf("CP_IMAGE_BASE environment variable not set, example: 'quay.io/pbaran/checkpointer'"))
	}

	config.CheckpointerNamespace = os.Getenv("POD_NAMESPACE")
	if config.CheckpointerNamespace == "" {
		err = errors.Join(err, fmt.Errorf("POD_NAMESPACE environment variable not set, should be set by Kubernetes"))
	}

	config.CheckpointerNode = os.Getenv("NODE_NAME")
	if config.CheckpointerNode == "" {
		err = errors.Join(err, fmt.Errorf("NODE_NAME environment variable not set, should be set by Kubernetes"))
	}

	if err != nil {
		return CheckpointerConfig{}, err
	}

	config.KubeletPort = getOrDefault("KUBELET_PORT", "10250")
	config.KanikoSecretName = getOrDefault("KANIKO_SECRET_NAME", "kaniko-secret")

	if os.Getenv("ENVIRONMENT") == "prod" {
		config.Environment = ProductionEnvironment
	} else {
		config.Environment = DevelopmentEnvironment
		log.Info().Msg("assuming development environment")
	}

	config.DisableRouteForward = os.Getenv("DISABLE_ROUTE_FORWARD") == "true"
	if config.DisableRouteForward {
		log.Info().Msg("DISABLE_ROUTE_FORWARD enabled, this should only be set in single-node cluster")
	}

	config.UseKanikoFS = os.Getenv("USE_KANIKO_FS") == "true"
	if config.UseKanikoFS {
		log.Info().Msg("USE_KANIKO_FS enabled, make sure Checkpointer has appropriate volume mounts")
	}

	return config, nil
}

func getOrDefault(env, defaultVal string) string {
	val := os.Getenv(env)
	if val == "" {
		log.Info().Msg(fmt.Sprintf("%s environment variable not set, defaulting to: %s", env, defaultVal))
		return defaultVal
	}
	return val
}
