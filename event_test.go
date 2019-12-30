package handler

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v24/github"
)

func TestFormatRef(t *testing.T) {
	for _, test := range []struct {
		refType string
		ref     string
		out     string
	}{
		{"branch", "master", "refs/heads/master"},
		{"tag", "v123", "refs/tags/v123"},
	} {
		out := formatRef(test.refType, test.ref)
		if out != test.out {
			t.Fatalf("Expected %s but got %s", test.out, out)
		}
	}
}

func p(s string) *string {
	return &s
}

func TestParseEventPush(t *testing.T) {
	for ghEvent, event := range map[*github.PushEvent]*Event{
		&github.PushEvent{
			Ref:    p("refs/heads/master"),
			After:  p("abc"),
			Before: p("def"),
			Repo: &github.PushEventRepository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		}: &Event{
			Type:     "push",
			Revision: "abc",
			Ref:      "refs/heads/master",
			Before:   "def",
			Repository: &github.Repository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		},
	} {
		out, err := ParseEvent(ghEvent)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(event, out); diff != "" {
			t.Fatalf("Not Equal (-want +got):\n%s", diff)
		}
	}

}

func TestParseEventDelete(t *testing.T) {
	for ghEvent, event := range map[*github.DeleteEvent]*Event{
		&github.DeleteEvent{
			RefType: p("branch"),
			Ref:     p("feature-123"),
			Repo: &github.Repository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		}: &Event{
			Type: "delete",
			Ref:  "refs/heads/feature-123",
			Repository: &github.Repository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		},
	} {
		out, err := ParseEvent(ghEvent)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(event, out); diff != "" {
			t.Fatalf("Not Equal (-want +got):\n%s", diff)
		}
	}
}

func TestParseEventCheckRun(t *testing.T) {
	for ghEvent, event := range map[*github.CheckRunEvent]*Event{
		&github.CheckRunEvent{
			Action: p("created"),
			CheckRun: &github.CheckRun{
				HeadSHA: p("abc"),
				CheckSuite: &github.CheckSuite{
					HeadBranch: p("feature-123"),
				},
			},
			Repo: &github.Repository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		}: &Event{
			Type:     "check_run",
			Action:   "created",
			Ref:      "refs/heads/feature-123",
			Revision: "abc",
			Repository: &github.Repository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		},
	} {
		out, err := ParseEvent(ghEvent)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(event, out); diff != "" {
			t.Fatalf("Not Equal (-want +got):\n%s", diff)
		}
	}
}

func TestParseEventCheckSuite(t *testing.T) {
	for ghEvent, event := range map[*github.CheckSuiteEvent]*Event{
		&github.CheckSuiteEvent{
			Action: p("completed"),
			CheckSuite: &github.CheckSuite{
				HeadBranch: p("feature-123"),
				AfterSHA:   p("abc"),
				BeforeSHA:  p("def"),
			},
			Repo: &github.Repository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		}: &Event{
			Type:     "check_suite",
			Action:   "completed",
			Ref:      "refs/heads/feature-123",
			Revision: "abc",
			Before:   "def",
			Repository: &github.Repository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		},
	} {
		out, err := ParseEvent(ghEvent)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(event, out); diff != "" {
			t.Fatalf("Not Equal (-want +got):\n%s", diff)
		}
	}
}

func TestAnnotations(t *testing.T) {
	for ev, annotations := range map[*Event]map[string]string{
		&Event{
			Type:     "push",
			Revision: "abc",
			Ref:      "refs/heads/master",
			Before:   "def",
			Repository: &github.Repository{
				FullName: p("foo/bar"),
				GitURL:   p("git://example.com/foo.git"),
				SSHURL:   p("git@example.com:foo.git"),
			},
		}: map[string]string{
			annotationPrefix + "event_type":   "push",
			annotationPrefix + "event_action": "",
			annotationPrefix + "repo_name":    "foo/bar",
			annotationPrefix + "repo_url":     "git://example.com/foo.git",
			annotationPrefix + "repo_ssh":     "git@example.com:foo.git",
			annotationPrefix + "ref":          "refs/heads/master",
			annotationPrefix + "revision":     "abc",
			annotationPrefix + "before":       "def",
		},
	} {
		if diff := cmp.Diff(annotations, ev.Annotations()); diff != "" {
			t.Fatalf("Not Equal (-want +got):\n%s", diff)
		}
	}
}
