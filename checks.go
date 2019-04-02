package handler

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/google/go-github/v24/github"
)

type CheckSuiteHandler struct {
	logger   log.Logger
	ghClient *github.Client
}

func NewCheckSuiteHandler(logger log.Logger, ghClient *github.Client) (*CheckSuiteHandler, error) {
	return &CheckSuiteHandler{
		logger:   logger,
		ghClient: ghClient,
	}, nil
}

func (h *CheckSuiteHandler) Handle(ctx context.Context, event *github.CheckSuiteEvent) (*handlerResponse, error) {
	switch *event.Action {
	case "request", "rerequested":
		h.ghClient.Checks.CreateCheckRun(ctx, *event.Repo.Owner.Name, *event.Repo.Name, github.CreateCheckRunOptions{
			Name:       "CI",
			HeadBranch: *event.CheckSuite.HeadBranch,
			HeadSHA:    *event.CheckSuite.HeadSHA,
		})
		level.Debug(h.logger).Log("msg", "Created check run")
	default:
		return nil, fmt.Errorf("Couldn't handle action %s", *event.Action)
	}
	return nil, nil
}
