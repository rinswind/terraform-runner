package main

import (
	"os"

	lib "github.com/kube-champ/terraform-runner/internal"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.InfoLevel)

	if err := lib.LoadEnv(); err != nil {
		log.Error("unable to load environment variables")
		log.Error(err)
		os.Exit(1)
	}

	if _, err := lib.CreateK8SConfig(); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	tf, err := lib.Setup()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	lib.AddSSHKeyIfExist()

	if err := tf.Init(); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	if lib.Env.Workspace != "" {
		if err := tf.SelectWorkspace(lib.Env.Workspace); err != nil {
			log.WithField("workspace", lib.Env.Workspace).Panic(err)
		}
	}

	// run an initial plan
	if err := tf.Plan(tf.GetPlanOptions()...); err != nil {
		log.Panic(err)
	}

	if !lib.Env.Destroy {
		if err := tf.Apply(tf.GetApplyOptions()...); err != nil {
			log.Panic(err)
		}
	} else {
		if err := tf.Destroy(tf.GetDestroyOptions()...); err != nil {
			log.Panic(err)
		}
	}

	log.Info("getting outputs from the run")

	outputs, err := tf.GetOutputs()

	if err != nil {
		log.Error("could not get outputs")
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

	log.Info("Run finished successfully")
}
