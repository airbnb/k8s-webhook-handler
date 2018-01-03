package main

import (
	"fmt"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/google/go-github/github"
)

type hook struct {
	*purger
	secret []byte
}

func newGithubHook(p *purger, secret []byte) *hook {
	return &hook{
		purger: p,
		secret: secret,
	}
}

func (h *hook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := log.With(logger, "client", r.RemoteAddr)
	if err := h.handle(w, r); err != nil {
		level.Error(logger).Log("msg", err)
		return
	}
	fmt.Fprint(w, "OK")
}

func (h *hook) handle(w http.ResponseWriter, r *http.Request) error {
	payload, err := github.ValidatePayload(r, h.secret)
	if err != nil {
		return fmt.Errorf("Couldn't read body: %s", err)
	}
	defer r.Body.Close()
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return fmt.Errorf("Couldn't parse body: %s", err)
	}

	logger := log.With(logger, "payload", fmt.Sprintf("%v", payload))
	switch e := event.(type) {
	case *github.DeleteEvent:
		level.Debug(logger).Log("msg", "Handling DeleteEvent webhook")
		if *e.RefType != "branch" {
			level.Info(logger).Log("msg", "Ignoring delete event for refType", "refType", *e.RefType)
			http.Error(w, "Nothing to do", http.StatusOK)
			return nil
		}
		return h.purger.purge(*e.Repo.FullName, *e.Ref)
	default:
		http.Error(w, "Webhook not supported", http.StatusBadRequest)
		return nil
	}
}
