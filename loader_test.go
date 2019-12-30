package handler

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewGithubLoader(t *testing.T) {
	for _, test := range []struct {
		token           string
		baseURL         string
		uploadURL       string
		expectBaseURL   string
		expectUploadURL string
		expectError     bool
	}{
		{"abc", "", "", "https://api.github.com/", "https://uploads.github.com/", false},
		{"", "", "", "https://api.github.com/", "https://uploads.github.com/", false},
		{"", "https://api.example.com", "https://upload.example.com", "https://api.example.com", "https://upload.example.com", false},
		{"", "%zzzzz", "%yyyyy", "", "", true},
	} {
		l, err := NewGithubLoader(test.token, test.baseURL, test.uploadURL)
		if test.expectError && err == nil {
			t.Fatalf("Expected error but got nil: %v -> %v", test, err)
		}
		if err != nil {
			if test.expectError {
				continue
			}
			t.Fatalf("Failed with %s for %v", err, test)
		}
		if l.Client.BaseURL.String() != test.expectBaseURL || l.Client.UploadURL.String() != test.expectUploadURL {
			t.Fatalf("Unexpected client %v for %v", l.Client, test)
		}
	}
}

func TestDecode(t *testing.T) {

	for _, test := range []struct {
		text        string
		expectError bool
		expectObj   *unstructured.Unstructured
	}{
		{
			`apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: hello-world-
spec:
  entrypoint: whalesay
  templates:
  - name: whalesay
    container:
      image: docker/whalesay
      command: [cowsay]
      args: ["hello world"]`,
			false,
			&unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "argoproj.io/v1alpha1", "kind": "Workflow", "metadata": map[string]interface{}{"generateName": "hello-world-"}, "spec": map[string]interface{}{"entrypoint": "whalesay", "templates": []interface{}{map[string]interface{}{"container": map[string]interface{}{"args": []interface{}{"hello world"}, "command": []interface{}{"cowsay"}, "image": "docker/whalesay"}, "name": "whalesay"}}}}},
		},
		{
			"foobar",
			true,
			nil,
		},
	} {
		obj, err := Decode(strings.NewReader(test.text))
		if test.expectError && err == nil {
			t.Fatalf("Expected error but got nil: %v", test)
		}
		if err != nil {
			if test.expectError {
				continue
			}
		}
		if !reflect.DeepEqual(obj, test.expectObj) {
			t.Fatalf("%#v != %#v", obj, test.expectObj)
		}
	}
}
