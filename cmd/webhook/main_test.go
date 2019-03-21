package main

import (
	"testing"

	handler "github.com/itskoko/k8s-webhook-handler"
	"k8s.io/apimachinery/pkg/labels"
)

func TestSelector(t *testing.T) {
	dh := &handler.DeleteHandler{
		SelectorKey: "ci-source-repo",
	}
	selector, err := dh.NewSelector("foo")
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range []struct {
		ls          labels.Labels
		shouldMatch bool
	}{
		{labels.Set{"ci-source-repo": "foo"}, true},
		{labels.Set{"ci-source-repo": "bar"}, false},
		{labels.Set{}, false},
	} {
		if selector.Matches(e.ls) != e.shouldMatch {
			t.Fatalf("Test %d) Expected %v.Matches(%v) == %v", i, selector, e.ls, e.shouldMatch)
		}
	}
}
