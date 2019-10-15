package handler

import (
	"errors"

	"github.com/google/go-github/v24/github"
)

var (
	ErrEventNotSupported = errors.New("Event not supported")
	ErrEventInvalid      = errors.New("Not a valid event")
)

type Event struct {
	Type     string
	Action   string
	Revision string
	Ref      string
	*github.Repository
}

func (e *Event) Annotations() map[string]string {
	return map[string]string{
		annotationPrefix + "event_type":   e.Type,
		annotationPrefix + "event_action": e.Action,
		annotationPrefix + "repo_name":    *e.Repository.FullName,
		annotationPrefix + "repo_url":     *e.Repository.GitURL,
		annotationPrefix + "repo_ssh":     *e.Repository.SSHURL,
		annotationPrefix + "ref":          e.Ref,
		annotationPrefix + "revision":     e.Revision,
	}
}

func ParseEvent(ev interface{}) (*Event, error) {
	event := &Event{}
	switch e := ev.(type) {
	case *github.PushEvent:
		event.Type = "push"
		event.Repository = pushEventRepoToRepo(e.GetRepo())
		event.Revision = *e.After
		event.Ref = *e.Ref
	case *github.DeleteEvent:
		event.Type = "delete"
		event.Repository = e.GetRepo()
		event.Ref = formatRef(*e.RefType, *e.Ref)
	case *github.CheckRunEvent:
		event.Type = "check_run"
		event.Action = *e.Action
		event.Repository = e.GetRepo()
		event.Revision = *e.CheckRun.HeadSHA
		event.Ref = branchToRef(*e.CheckRun.CheckSuite.HeadBranch)
	case *github.CheckSuiteEvent:
		event.Type = "check_suite"
		event.Action = *e.Action
		event.Repository = e.GetRepo()
		event.Revision = *e.CheckSuite.AfterSHA
		event.Ref = branchToRef(*e.CheckSuite.HeadBranch)
	}

	return event, nil
}

func formatRef(refType, ref string) string {
	if refType == "branch" {
		refType = "head"
	}
	return "refs/" + refType + "s/" + ref
}

func branchToRef(branch string) string {
	return "refs/heads/" + branch
}

// FIXME: We should translate all fields or clean that mess up at upstream.
func pushEventRepoToRepo(r *github.PushEventRepository) *github.Repository {
	return &github.Repository{
		FullName: r.FullName,
		GitURL:   r.GitURL,
		SSHURL:   r.SSHURL,
	}
}
