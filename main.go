package main

import (
	"os"

	lib "github.com/kube-champ/terraform-runner/internal"
	log "github.com/sirupsen/logrus"
)

func main() {
	lib.LoadEnv()

	loglLevel, err := log.ParseLevel(lib.Env.LogLevel)
	if err != nil {
		log.Panicf("failed to parse log level from '%s': %v", lib.Env.LogLevel, err)
	}

	log.SetLevel(loglLevel)

	if _, err := lib.CreateK8SConfig(); err != nil {
		log.Panic(err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(lib.Env.PluginCache, 0755); err != nil {
		log.WithField("cacheDir", lib.Env.PluginCache).Panic(err)
	}

	lib.AddSSHKeyIfExist()

	tr := lib.NewTerraformRunner(lib.Env.TerraformVersion, lib.Env.ProjectDir, lib.Env.PluginCache, lib.Env.VarFilesPath)

	err = tr.Setup()
	if err != nil {
		log.Panic(err)
	}

	if err := tr.Init(tr.GetInitOptions()...); err != nil {
		log.Panic(err)
	}

	if lib.Env.Workspace != "" {
		if err := tr.SelectWorkspace(lib.Env.Workspace); err != nil {
			log.WithField("workspace", lib.Env.Workspace).Panic(err)
		}
	}

	if err := tr.Plan(tr.GetPlanOptions()...); err != nil {
		log.Panic(err)
	}

	if !lib.Env.Destroy {
		if err := tr.Apply(tr.GetApplyOptions()...); err != nil {
			log.Panic(err)
		}
	} else {
		if err := tr.Destroy(tr.GetDestroyOptions()...); err != nil {
			log.Panic(err)
		}
	}

	outputs, err := tr.GetOutputs()
	if err != nil {
		log.Panic(err)
	}

	if len(outputs) > 0 {
		if err := lib.UpdateSecretWithOutputs(outputs); err != nil {
			log.Panic(err)
		}

		log.WithField("secretName", lib.Env.OutputSecretName).Info("secret was updated with outputs")
	} else {
		log.Info("no outputs where found in module")
	}

	log.Info("run finished successfully")
}
