package handler

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/metrics/statsd"
	"github.com/google/go-github/v24/github"
	"k8s.io/apimachinery/pkg/runtime"
)

type mockKubernetesClient struct {
	obj       runtime.Object
	namespace string
}

func (k *mockKubernetesClient) Apply(obj runtime.Object, namespace string) error {
	k.obj = obj
	k.namespace = namespace
	return nil
}

type mockLoader struct {
	obj  runtime.Object
	repo string
	path string
	ref  string
}

func (l *mockLoader) Load(ctx context.Context, repo, path, ref string) (runtime.Object, error) {
	return l.obj, nil
}

func TestHandle(t *testing.T) {
	var (
		owner    = "foo"
		name     = "bar"
		fullName = "foo bar"
		gitURL   = "giturl"
		sshURL   = "sshurl"

		ref    = "ref"
		before = "before"
		after  = "after"

		config = &Config{Namespace: "namespace", ResourcePath: "foo/bar.yaml", Secret: []byte("foobar")}
	)
	_ = &github.PushEvent{
		Repo: &github.PushEventRepository{
			Name:     &name,
			Owner:    &github.User{Login: &owner},
			FullName: &fullName,
			GitURL:   &gitURL,
			SSHURL:   &sshURL,
		},
		Ref:    &ref,
		Before: &before,
		After:  &after,
	}
	handler := NewGithubHookHandler(
		logger,
		config,
		&mockKubernetesClient{},
		&mockLoader{},
		statsd.New("k8s-ci-purger.", logger),
	)

	req := httptest.NewRequest("POST", "http://example.com/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "DeleteEvent")
	req.Header.Set("X-GitHub-Delivery", "4636fc67-b693-4a27-87a4-18d4021ae789")
	req.Header.Set("X-Hub-Signature", "sha1=1234")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("Content-Type"))
	fmt.Println(string(body))
}
