package internal

import (
	"context"

	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

var ClientSet kubernetes.Interface

func CreateK8SConfig() (*rest.Config, error) {
	var clientset *kubernetes.Clientset
	var config *rest.Config
	var err error

	if fileExists(Env.KubeConfigPath) {
		if config, err = clientcmd.BuildConfigFromFlags("", Env.KubeConfigPath); err != nil {
			log.Error(err, "failed to create K8s config from kubeconfig")
			return nil, err
		}
	} else {
		if config, err = rest.InClusterConfig(); err != nil {
			log.Error(err, "Failed to create in-cluster k8s config")
			return nil, err
		}
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create K8s clientset")
		return nil, err
	}

	ClientSet = clientset

	return config, nil
}

func UpdateSecretWithOutputs(outputs map[string][]byte) error {
	secrets := ClientSet.CoreV1().Secrets(Env.PodNamespace)

	secret, err := secrets.Get(context.Background(), Env.OutputSecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	secret.Data = outputs

	if _, err := secrets.Update(context.Background(), secret, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}
