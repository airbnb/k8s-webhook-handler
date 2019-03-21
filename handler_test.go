package handler

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/metrics/statsd"
	"github.com/google/go-github/v24/github"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fake "k8s.io/client-go/kubernetes/fake"
)

func TestHandleDelete(t *testing.T) {
	repo := "foo/bar"
	selectorKey := "ci-source-repo"
	selectorValue := "foo.bar"
	branch := "master"
	refType := "branch"

	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      "foo",
		Namespace: branch,
		Labels:    map[string]string{selectorKey: selectorValue},
	}}
	namespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: branch,
	}}
	clientset := fake.NewSimpleClientset(namespace, service)

	discoveryInterface := clientset.Discovery()
	// FIXME: DiscoveryInterface mock isn't complete, so
	// ServerPreferredResources() returns nothing and breaks the purger
	t.Log(discoveryInterface.ServerPreferredResources())
	t.Log(clientset.Fake.Resources)
	p := &DeleteHandler{
		DiscoveryInterface: discoveryInterface,
		NamespaceInterface: clientset.CoreV1().Namespaces(),
		SelectorKey:        selectorKey,
	}
	h := NewGithubHook(p, []byte("foo"), statsd.New("k8s-foobar.", logger))

	payload := github.DeleteEvent{
		RefType: &refType,
		Ref:     &branch,
		Repo: &github.Repository{
			FullName: &repo,
		},
	}
	fmt.Println(payload)

	/*
		pr, pw := io.Pipe()
		enc := json.NewEncoder(pw)
		go func() { enc.Encode(payload) }()*/

	req := httptest.NewRequest("POST", "http://example.com/", nil) //pr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "DeleteEvent")
	req.Header.Set("X-GitHub-Delivery", "4636fc67-b693-4a27-87a4-18d4021ae789")
	req.Header.Set("X-Hub-Signature", "sha1=1234")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("Content-Type"))
	fmt.Println(string(body))
}
