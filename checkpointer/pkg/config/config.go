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

// KubeletConfig represents configuration related to Kubelet.
type KubeletConfig struct {

	// CertFile is path to a file with TLS certificate used to authenticate to Kubelet.
	CertFile string

	// KeyFile is path to a file with private key related to the certificate used to authenticate to Kubelet.
	KeyFile string

	// BaseUrl is a URL Kubelet is listening on with protocol, domain name/IP and port, e.g.: https://142.168.1.1:10250
	BaseUrl string

	// Checkpointer will not verify Kubelet TLS certificate if AllowInsecure is set to true.
	AllowInsecure bool
}

// CheckpointConfig represents configuration related to checkpointing.
type CheckpointConfig struct {

	// CheckpointerNamespace represents the Kubernetes Namespace that Checkpointer is running in.
	CheckpointerNamespace string

	// CheckpointerNode represents the name of Kubernetes Node that Checkpointer is running on.
	CheckpointerNode string

	// CheckpointImagePrefix represents container image name without the tag as: { CheckpointImagePrefix }:tag.
	CheckpointImagePrefix string

	// CheckpointBaseImage will be used as base image in checkpoint Dockerfile as: FROM { CheckpointBaseImage }.
	CheckpointBaseImage string

	// KanikoSecretName represents the name of Kubernetes Secret containing credentials for Kaniko to push container
	// image to a container registry.
	KanikoSecretName string

	// KanikoBuildContextDir defines path to a directory where Checkpointer will prepare build context for Kaniko Pod.
	KanikoBuildContextDir string

	// KanikoTimeoutSeconds represent time in seconds after which Checkpointer will stop waiting for Kaniko Pod to
	// reach a certain Pod phase.
	KanikoTimeoutSeconds int64
}

// GlobalConfig represents the whole configuration of Checkpointer.
type GlobalConfig struct {
	CheckpointConfig CheckpointConfig
	KubeletConfig    KubeletConfig

	// StorageBasePath defines path to a directory where Checkpointer will store checkpoint results.
	StorageBasePath string

	// DockerfileTemplateFile defines path to a file with Dockerfile template used for checkpoint container image.
	DockerfileTemplateFile string

	// CheckpointerPort defines on what port Checkpointer will listen on.
	CheckpointerPort int64

	// DisableRouteForward will disable RouteProxy middleware if set to true.
	DisableRouteForward bool

	// If UseKanikoFS is true, Checkpointer will use Kaniko File system strategy to transfer build context to Kaniko Pod.
	UseKanikoFS bool

	// Environment defines what environment Checkpointer is running in: prod/dev, possibly more in the future.
	Environment Environment
}

// LoadGlobalConfig loads the whole configuration for Checkpointer. It will use defaults where possible, but if there
// are required environment variables missing, it will return error with all the missing variables.
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

	config.DockerfileTemplateFile = os.Getenv("DOCKERFILE_TMPL_FILE")
	if config.DockerfileTemplateFile == "" {
		err = errors.Join(err, fmt.Errorf("DOCKERFILE_TMPL_FILE environment variable not set,"+
			" should be set within Checkpointer container image, this is likely a build problem"))
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
