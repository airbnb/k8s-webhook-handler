package main

import (
	"testing"

	"gopkg.in/go-playground/webhooks.v3/github"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dfake "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
)

// // We need to wrap discoveryFake.FakeDiscovery to implemented ServerPreferredResources()
// type fakeDiscovery struct {
// 	discoveryFake.FakeDiscovery
// }
//
// func (c *fakeDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
// 	logger.Log("msg", "here", "resources", fmt.Sprintf("%v", c.FakeDiscovery.Fake.Resources))
// 	return c.Fake.Resources, nil
// }

func TestHandleDelete(t *testing.T) {
	selectorKey := "ci-source-repo"
	selectorValue := "foo.bar"
	branch := "master"

	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      "foo",
		Namespace: branch,
		Labels:    map[string]string{selectorKey: selectorValue},
	}}
	clientset := fake.NewSimpleClientset(service)

	discoveryInterface := clientset.Discovery()
	// FIXME: DiscoveryInterface mock isn't complete, so
	// ServerPreferredResources() returns nothing and breaks the purger
	t.Log(discoveryInterface.ServerPreferredResources())
	t.Log(clientset.Fake.Resources)
	p := &purger{
		DiscoveryInterface: discoveryInterface,
		ClientPool:         &dfake.FakeClientPool{Fake: clientset.Fake},
		selectorKey:        "ci-source-repo",
	}
	payload := github.DeletePayload{
		RefType: "branch",
		Ref:     branch,
	}
	payload.Repository.FullName = "foo/bar"
	if err := handleDelete(p, payload, nil); err != nil {
		t.Fatal(err)
	}
}
