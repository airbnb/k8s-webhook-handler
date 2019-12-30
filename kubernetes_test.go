package handler

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

type fakeRESTMapper struct {
	schema.GroupKind
}

func (m *fakeRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	panic("not implemented")
}
func (m *fakeRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	panic("not implemented")
}
func (m *fakeRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	panic("not implemented")
}

func (m *fakeRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	panic("not implemented")
}

func (m *fakeRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	m.GroupKind = gk
	return &meta.RESTMapping{Resource: schema.GroupVersionResource{Group: gk.Group}}, nil
}

func (m *fakeRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	panic("not implemented")
}

func (m *fakeRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	panic("not implemented")
}

func TestApply(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "argoproj.io/v1alpha1", "kind": "Workflow", "metadata": map[string]interface{}{"generateName": "hello-world-", "namespace": "default"}, "spec": map[string]interface{}{"entrypoint": "whalesay", "templates": []interface{}{map[string]interface{}{"container": map[string]interface{}{"args": []interface{}{"hello world"}, "command": []interface{}{"cowsay"}, "image": "docker/whalesay"}, "name": "whalesay"}}}}}

	scheme := runtime.NewScheme()
	client := &kubernetesClient{
		RESTMapper: &fakeRESTMapper{},
		Interface:  fake.NewSimpleDynamicClient(scheme),
	}
	if err := client.Apply(obj, "default"); err != nil {
		t.Fatal(err)
	}

	gvk := obj.GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}

	mapping, err := client.RESTMapper.RESTMapping(gk, gvk.Version)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := client.Interface.Resource(mapping.Resource).Namespace("default").Get("", metav1.GetOptions{})
	if diff := cmp.Diff(obj, got); diff != "" {
		t.Fatalf("Not Equal (-want +got):\n%s", diff)
	}
}
