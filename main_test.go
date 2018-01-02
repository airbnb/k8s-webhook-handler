package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"
)

func TestSelector(t *testing.T) {
	p := &purger{
		selectorKey: "ci-source-repo",
	}
	selector, err := p.newSelector("foo")
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
