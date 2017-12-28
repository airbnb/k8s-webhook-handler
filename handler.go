package main

import (
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/github"
)

func newGithubHandler(p *purger) webhooks.Webhook {
	hook := github.New(&github.Config{Secret: os.Getenv("GITHUB_SECRET")})
	hook.RegisterEvents(githubHandler(p), github.DeleteEvent)
	return hook
}

func githubHandler(p *purger) func(payload interface{}, header webhooks.Header) {
	return func(payload interface{}, header webhooks.Header) {
		logger := log.With(logger, "payload", payload)
		level.Debug(logger).Log("msg", "Handling webhook")
		if err := handleDelete(p, payload, header); err != nil {
			level.Error(logger).Log("msg", "Couldn't handle webhook", "error", err)
		}
	}
}

func handleDelete(p *purger, payload interface{}, header webhooks.Header) error {
	deletePayload, ok := payload.(github.DeletePayload)
	if !ok {
		return errors.New("Unexpected payload type")
	}
	if deletePayload.RefType != "branch" {
		return errors.New("Unexpected ref type")
	}
	// FIXME: We should return a failure to the webhook or retry
	selectorVal := strings.Replace(deletePayload.Repository.FullName, "/", ".", -1)
	namespace := deletePayload.Ref
	return p.purge(selectorVal, namespace)
}
