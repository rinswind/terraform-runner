package internal

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type EnvConfig struct {
	LogLevel string

	// Terraform execution
	TerraformVersion string
	Workspace        string
	Destroy          bool

	PluginCache string

	// Terraform Project files
	ProjectDir   string
	VarFilesPath string

	// Output to K8S Secret
	PodNamespace     string
	OutputSecretName string
	KubeConfigPath   string
}

var Env *EnvConfig

func LoadEnv() {
	env := &EnvConfig{}

	env.LogLevel = strings.ToLower(getEnvWithDefault("LOG_LEVEL", "info"))

	env.TerraformVersion = getEnvOrPanic("TERRAFORM_VERSION")
	env.Workspace = getEnvWithDefault("TERRAFORM_WORKSPACE", "default")
	env.Destroy = getEnvWithDefaultAsBool("TERRAFORM_DESTROY", false)

	env.PluginCache = getEnvOrPanic("TF_PLUGIN_CACHE_DIR")

	env.ProjectDir = getEnvWithDefault("TERRAFORM_PROJECT_PATH", "/tmp/tf-project")
	env.VarFilesPath = getEnvWithDefault("TERRAFORM_VAR_FILES_PATH", "/tmp/tf-vars")

	env.PodNamespace = getEnvOrPanic("POD_NAMESPACE")
	env.OutputSecretName = getEnvOrPanic("OUTPUT_SECRET_NAME")
	env.KubeConfigPath = getEnvWithDefault("KUBECONFIG", "")

	Env = env
}

func getEnvOrPanic(name string) string {
	env, present := os.LookupEnv(name)
	if !present {
		log.Panicf("environment variable '%s' is required but was not found", name)
	}

	return env
}

func getEnvWithDefault(name string, def string) string {
	env, present := os.LookupEnv(name)
	if def != "" && !present {
		return def
	}

	return env
}

func getEnvWithDefaultAsBool(name string, def bool) bool {
	env, present := os.LookupEnv(name)
	if !def && !present {
		return def
	}

	val, _ := strconv.ParseBool(env)

	return val
}
