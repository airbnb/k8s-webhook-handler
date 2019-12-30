package handler

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type fakeRESTMapper struct {
	schema.GroupKind
	versions []string
}

func (m fakeRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	panic("not implemented")
}
func (m fakeRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	panic("not implemented")
}
func (m fakeRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	panic("not implemented")
}

func (m fakeRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	panic("not implemented")
}

func (m fakeRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	m.GroupKind = gk
	return &meta.RESTMapping{Resource: schema.GroupVersionResource{Group: gk.Group}}, nil
}

func (m fakeRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	panic("not implemented")
}

func (m fakeRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	panic("not implemented")
}

type fakeInterface struct{}

func (i fakeInterface) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return fakeNamespaceableResourceInterface{resource: resource}
}

type fakeNamespaceableResourceInterface struct {
	resource schema.GroupVersionResource
	fakeResourceInterface
}

func (i fakeNamespaceableResourceInterface) Namespace(ns string) dynamic.ResourceInterface {
	return &fakeResourceInterface{
		namespace: ns,
	}
}

type fakeResourceInterface struct {
	namespace string
}

func (i fakeResourceInterface) Create(obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (i fakeResourceInterface) Update(obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (i fakeResourceInterface) UpdateStatus(obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (i fakeResourceInterface) Delete(name string, options *metav1.DeleteOptions, subresources ...string) error {
	panic("not implemented")
}

func (i fakeResourceInterface) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("not implemented")
}

func (i fakeResourceInterface) Get(name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (i fakeResourceInterface) List(opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("not implemented")
}

func (i fakeResourceInterface) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}

func (i fakeResourceInterface) Patch(name string, pt types.PatchType, data []byte, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func TestApply(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "argoproj.io/v1alpha1", "kind": "Workflow", "metadata": map[string]interface{}{"generateName": "hello-world-"}, "spec": map[string]interface{}{"entrypoint": "whalesay", "templates": []interface{}{map[string]interface{}{"container": map[string]interface{}{"args": []interface{}{"hello world"}, "command": []interface{}{"cowsay"}, "image": "docker/whalesay"}, "name": "whalesay"}}}}}

	client := &kubernetesClient{
		RESTMapper: fakeRESTMapper{},
		Interface:  fakeInterface{},
	}
	client.Apply(obj, "default")
}
