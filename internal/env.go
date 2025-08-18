package internal

import (
	"log"
	"os"
	"strconv"
)

type EnvConfig struct {
	// Terraform execution
	TerraformVersion string
	Workspace        string
	Destroy          bool

	// Terraform Project files
	ProjectDir   string
	VarFilesPath string

	// Output to K8S Secret
	PodNamespace     string
	OutputSecretName string
	KubeConfigPath   string
}

var Env *EnvConfig

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

func LoadEnv() error {
	env := &EnvConfig{}

	env.TerraformVersion = getEnvOrPanic("TERRAFORM_VERSION")
	env.Workspace = getEnvWithDefault("TERRAFORM_WORKSPACE", "default")
	env.Destroy = getEnvWithDefaultAsBool("TERRAFORM_DESTROY", false)

	env.ProjectDir = getEnvWithDefault("TERRAFORM_PROJECT_PATH", "/tmp/tfproject")
	env.VarFilesPath = getEnvWithDefault("TERRAFORM_VAR_FILES_PATH", "/tmp/tfvars")

	env.PodNamespace = getEnvOrPanic("POD_NAMESPACE")
	env.OutputSecretName = getEnvOrPanic("OUTPUT_SECRET_NAME")
	env.KubeConfigPath = getEnvWithDefault("KUBECONFIG", "")

	Env = env

	return nil
}
