package handler

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewKubernetesConfig(kubeconfig string) (config *rest.Config, err error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
