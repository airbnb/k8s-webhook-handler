package git

import (
	"testing"
)

func TestBranchExists(t *testing.T) {
	cases := []struct {
		repo    string
		branch  string
		exists  bool
		errored bool
	}{
		{"https://github.com/itskoko/k8s-ci-purger.git", "master", true, false},
		{"https://github.com/itskoko/k8s-ci-purger.git", "does-not-exist", false, false},
		{"https://invalid.example.com", "master", true, true},
	}

	for i, c := range cases {
		exists, err := BranchExists(c.repo, c.branch)
		t.Log("err", err)
		if c.errored {
			if err != nil {
				continue
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
		}
		if exists != c.exists {
			t.Fatalf("%d. failed. Expected exists to be %t but is %t", i, c.exists, exists)
		}
	}
}
