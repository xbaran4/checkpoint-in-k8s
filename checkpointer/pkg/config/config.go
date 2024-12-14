package config

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
)

type Environment uint8

const (
	ProductionEnvironment Environment = iota
	DevelopmentEnvironment
)

type KubeletConfig struct {
	CertFile      string
	KeyFile       string
	BaseUrl       string
	AllowInsecure bool
}

type CheckpointConfig struct {
	CheckpointerNamespace string
	CheckpointerNode      string
	CheckpointImagePrefix string
	CheckpointBaseImage   string
	KanikoSecretName      string
	KanikoBuildContextDir string
	KanikoTimeoutSeconds  int64
}

type GlobalConfig struct {
	CheckpointConfig    CheckpointConfig
	KubeletConfig       KubeletConfig
	StorageBasePath     string
	CheckpointerPort    int64
	DisableRouteForward bool
	UseKanikoFS         bool
	Environment         Environment
}

// LoadGlobalConfig
func LoadGlobalConfig() (GlobalConfig, error) {
	var err error
	config := GlobalConfig{}

	if os.Getenv("ENVIRONMENT") == "prod" {
		config.Environment = ProductionEnvironment
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		// set plaintext logs for better dev experience
		config.Environment = DevelopmentEnvironment
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
		log.Info().Msg("assuming development environment, set ENVIRONMENT=prod to assume production")
	}

	config.CheckpointConfig.CheckpointImagePrefix = os.Getenv("CHECKPOINT_IMAGE_PREFIX")
	if config.CheckpointConfig.CheckpointImagePrefix == "" {
		err = errors.Join(err, fmt.Errorf("CHECKPOINT_IMAGE_PREFIX environment variable not set, example: 'quay.io/pbaran/checkpointed'"))
	}

	config.CheckpointConfig.CheckpointerNamespace = os.Getenv("CHECKPOINTER_NAMESPACE")
	if config.CheckpointConfig.CheckpointerNamespace == "" {
		err = errors.Join(err, fmt.Errorf("CHECKPOINTER_NAMESPACE environment variable not set, should be set by Kubernetes"))
	}

	config.CheckpointConfig.CheckpointerNode = os.Getenv("CHECKPOINTER_NODE")
	if config.CheckpointConfig.CheckpointerNode == "" {
		err = errors.Join(err, fmt.Errorf("CHECKPOINTER_NODE environment variable not set, should be set by Kubernetes"))
	}

	checkpointerNodeIP := os.Getenv("CHECKPOINTER_NODE_IP")
	if checkpointerNodeIP == "" {
		err = errors.Join(err, fmt.Errorf("CHECKPOINTER_NODE_IP environment variable not set, should be set by Kubernetes"))
	}

	if err != nil {
		return GlobalConfig{}, err
	}

	config.CheckpointerPort = getOrDefaultNonNegativeNumber("CHECKPOINTER_PORT", 3333)
	config.CheckpointConfig.KanikoTimeoutSeconds = getOrDefaultNonNegativeNumber("KANIKO_TIMEOUT", 30)
	kubeletPort := getOrDefaultNonNegativeNumber("KUBELET_PORT", 10250)
	config.KubeletConfig.BaseUrl = fmt.Sprintf("https://%s:%d", checkpointerNodeIP, kubeletPort)

	config.CheckpointConfig.CheckpointBaseImage = getOrDefault("CHECKPOINT_BASE_IMAGE", "pbaran555/checkpoint-base:latest")
	config.CheckpointConfig.KanikoSecretName = getOrDefault("KANIKO_SECRET_NAME", "kaniko-secret")
	config.StorageBasePath = getOrDefault("STORAGE_BASE_PATH", "/tmp/checkpointer")
	config.KubeletConfig.CertFile = getOrDefault("KUBELET_CERT_FILE", "/etc/kubernetes/pki/apiserver-kubelet-client.crt")
	config.KubeletConfig.KeyFile = getOrDefault("KUBELET_KEY_FILE", "/etc/kubernetes/pki/apiserver-kubelet-client.key")

	if config.KubeletConfig.AllowInsecure = os.Getenv("KUBELET_ALLOW_INSECURE") == "true"; config.KubeletConfig.AllowInsecure {
		log.Warn().Msg("KUBELET_ALLOW_INSECURE enabled, Checkpointer will not verify kubelet certificate")
	}
	if config.DisableRouteForward = os.Getenv("DISABLE_ROUTE_FORWARD") == "true"; config.DisableRouteForward {
		log.Info().Msg("DISABLE_ROUTE_FORWARD enabled, this should only be set in single-node cluster")
	}
	if config.UseKanikoFS = os.Getenv("USE_KANIKO_FS") == "true"; config.UseKanikoFS {
		log.Info().Msg("USE_KANIKO_FS enabled, make sure Checkpointer has appropriate volume mounts")
		config.CheckpointConfig.KanikoBuildContextDir = getOrDefault("KANIKO_BUILD_CTX_DIR", "/tmp/build-contexts")
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
