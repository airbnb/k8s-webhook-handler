package handler

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/statsd"
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
		config = &Config{Namespace: "namespace", ResourcePath: "foo/bar.yaml", Secret: []byte("foobar")}
	)

	logger := log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), "caller", log.Caller(5))
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
