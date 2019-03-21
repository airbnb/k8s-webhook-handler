package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/statsd"
	"github.com/google/go-github/v24/github"
)

type hook struct {
	*DeleteHandler
	secret []byte

	requestCounter metrics.Counter
	errorCounter   metrics.Counter
	callDuration   metrics.Histogram
}

func NewGithubHook(dh *DeleteHandler, secret []byte, statsdClient *statsd.Statsd) *hook {
	return &hook{
		DeleteHandler:  dh,
		secret:         secret,
		requestCounter: statsdClient.NewCounter("requests", 1.0),
		errorCounter:   statsdClient.NewCounter("errors", 1.0),
		callDuration:   statsdClient.NewTiming("duration", 1.0),
	}
}

func (h *hook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func(begin time.Time) { h.callDuration.Observe(time.Since(begin).Seconds()) }(time.Now())
	logger := log.With(logger, "client", r.RemoteAddr)
	h.requestCounter.Add(1)
	hr, err := h.handle(w, r)
	if err != nil {
		h.errorCounter.Add(1)
		level.Error(logger).Log("msg", err)
	}
	if hr.status == 0 {
		hr.status = http.StatusInternalServerError
	}
	http.Error(w, hr.message, hr.status)
}

func (h *hook) handle(w http.ResponseWriter, r *http.Request) (*handlerResponse, error) {
	if r.Method != http.MethodPost {
		return &handlerResponse{http.StatusBadRequest, "Method not supported"}, nil
	}
	payload, err := github.ValidatePayload(r, h.secret)
	if err != nil {
		return &handlerResponse{http.StatusBadRequest, "Invalid payload"}, err
	}
	defer r.Body.Close()
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return &handlerResponse{http.StatusBadRequest, "Couldn't parse webhook"}, err
	}

	logger := log.With(logger, "payload", fmt.Sprintf("%v", payload))

	switch e := event.(type) {
	case *github.DeleteEvent:
		level.Debug(logger).Log("msg", "Handling DeleteEvent webhook")
		return h.DeleteHandler.Handle(e)
	default:
		return &handlerResponse{http.StatusBadRequest, "Webhook not supported"}, nil
	}
}

type handlerResponse struct {
	status  int
	message string
}
