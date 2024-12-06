package config

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
)

type Environment uint8

const (
	ProductionEnvironment Environment = iota
	DevelopmentEnvironment
)

type CheckpointerConfig struct {
	CheckpointerNamespace string
	CheckpointerNode      string
	CheckpointImagePrefix string
	CheckpointBaseImage   string
	CheckpointerPort      int64
	KubeletPort           int64
	KanikoTimoutSeconds   int64
	KanikoSecretName      string
	StorageBasePath       string
	DisableRouteForward   bool
	UseKanikoFS           bool
	Environment           Environment
}

func LoadCheckpointerConfig() (CheckpointerConfig, error) {
	var err error
	config := CheckpointerConfig{}

	config.CheckpointImagePrefix = os.Getenv("CP_IMAGE_PREFIX")
	if config.CheckpointImagePrefix == "" {
		err = errors.Join(err, fmt.Errorf("CP_IMAGE_BASE environment variable not set, example: 'quay.io/pbaran/checkpointed'"))
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

	config.CheckpointerPort = getOrDefaultNonNegativeNumber("CHECKPOINTER_PORT", 3333)
	config.KubeletPort = getOrDefaultNonNegativeNumber("KUBELET_PORT", 10250)
	config.KanikoTimoutSeconds = getOrDefaultNonNegativeNumber("KANIKO_TIMEOUT", 30)

	config.CheckpointBaseImage = getOrDefault("CHECKPOINT_BASE_IMAGE", "pbaran555/checkpoint-base")
	config.KanikoSecretName = getOrDefault("KANIKO_SECRET_NAME", "kaniko-secret")
	config.StorageBasePath = getOrDefault("STORAGE_BASE_PATH", "/tmp/checkpointer")

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

func getOrDefaultNonNegativeNumber(env string, defaultVal int64) int64 {
	val := os.Getenv(env)
	if val == "" {
		log.Info().Msg(fmt.Sprintf("%s environment variable not set, defaulting to: %d", env, defaultVal))
		return defaultVal
	}
	number, err := strconv.ParseInt(val, 10, 64)
	if err != nil || number < 0 {
		log.Info().Msg(fmt.Sprintf("%s environment variable malformed, defaulting to: %d", env, defaultVal))
		return defaultVal
	}

	return number
}
