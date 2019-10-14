package handler

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesClient interface {
	Apply(obj runtime.Object, namespace string) error
}

type kubernetesClient struct {
	dynamic.Interface
	meta.RESTMapper
}

func NewKubernetesClient(kubeconfig string) (*kubernetesClient, error) {
	config, err := buildKubernetesConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	intf, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	groupResources, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return nil, err
	}
	return &kubernetesClient{
		Interface:  intf,
		RESTMapper: restmapper.NewDiscoveryRESTMapper(groupResources),
	}, nil
}

func buildKubernetesConfig(kubeconfig string) (config *rest.Config, err error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func (k *kubernetesClient) Apply(obj runtime.Object, namespace string) error {
	switch obj := obj.(type) {
	case *unstructured.Unstructured:
		gvk := obj.GroupVersionKind()
		gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}

		mapping, err := k.RESTMapper.RESTMapping(gk, gvk.Version)
		if err != nil {
			return err
		}
		if _, err := k.Interface.Resource(mapping.Resource).Namespace(namespace).Create(obj, metav1.CreateOptions{}); err != nil {
			return err
		}
	case *unstructured.UnstructuredList:
		return obj.EachListItem(func(o runtime.Object) error { return k.Apply(o, namespace) })
	}
	return nil
}
