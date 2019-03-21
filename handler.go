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

type Handler struct {
	*DeleteHandler
	*PushHandler
	secret []byte

	requestCounter metrics.Counter
	errorCounter   metrics.Counter
	callDuration   metrics.Histogram
}

func NewGithubHookHandler(secret []byte, statsdClient *statsd.Statsd) *Handler {
	return &Handler{
		secret:         secret,
		requestCounter: statsdClient.NewCounter("requests", 1.0),
		errorCounter:   statsdClient.NewCounter("errors", 1.0),
		callDuration:   statsdClient.NewTiming("duration", 1.0),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func(begin time.Time) { h.callDuration.Observe(time.Since(begin).Seconds()) }(time.Now())
	logger := log.With(logger, "client", r.RemoteAddr)
	h.requestCounter.Add(1)
	hr, err := h.handle(w, r)
	if hr == nil {
		hr = &handlerResponse{}
	}
	if err != nil {
		h.errorCounter.Add(1)
		level.Error(logger).Log("msg", err)
		if hr.status == 0 {
			hr.status = http.StatusInternalServerError
		}
		if hr.message == "" {
			hr.message = err.Error()
		}
	} else {
		if hr.status == 0 {
			hr.status = http.StatusOK
		}
		if hr.message == "" {
			hr.message = "Webhook handled successfully"
		}
	}
	http.Error(w, hr.message, hr.status)
}

func (h *Handler) handle(w http.ResponseWriter, r *http.Request) (*handlerResponse, error) {
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

	errNotRegistered := fmt.Errorf("No handler for event %T registered", event)
	switch e := event.(type) {
	case *github.DeleteEvent:
		if h.DeleteHandler == nil {
			return &handlerResponse{http.StatusBadRequest, errNotRegistered.Error()}, errNotRegistered
		}
		return h.DeleteHandler.Handle(r.Context(), e)
	case *github.PushEvent:
		if h.PushHandler == nil {
			return &handlerResponse{http.StatusBadRequest, errNotRegistered.Error()}, errNotRegistered
		}
		return h.PushHandler.Handle(r.Context(), e)

	default:
		return &handlerResponse{http.StatusBadRequest, "Webhook not supported"}, nil
	}
}

type handlerResponse struct {
	status  int
	message string
}
