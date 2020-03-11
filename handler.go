package handler

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/statsd"
	"github.com/google/go-github/v24/github"
	"k8s.io/apimachinery/pkg/api/meta"
)

const annotationPrefix = "k8s-webhook-handler.io/"

type Config struct {
	Namespace           string
	ResourcePath        string
	HandlerLivenessPath string
	Secret              []byte
	IgnoreRefRegex      *regexp.Regexp
	DryRun              bool
}

type Handler struct {
	log.Logger
	Config *Config
	Loader
	KubernetesClient

	requestCounter metrics.Counter
	errorCounter   metrics.Counter
	callDuration   metrics.Histogram
}

func NewGithubHookHandler(logger log.Logger, config *Config, kubernetesClient KubernetesClient, loader Loader, statsdClient *statsd.Statsd) *Handler {
	return &Handler{
		Logger:           logger,
		Config:           config,
		Loader:           loader,
		KubernetesClient: kubernetesClient,
		requestCounter:   statsdClient.NewCounter("requests", 1.0),
		errorCounter:     statsdClient.NewCounter("errors", 1.0),
		callDuration:     statsdClient.NewTiming("duration", 1.0),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func(begin time.Time) { h.callDuration.Observe(time.Since(begin).Seconds()) }(time.Now())
	logger := log.With(h.Logger, "client", r.RemoteAddr)
	h.requestCounter.Add(1)

	if r.URL.Path == h.Config.HandlerLivenessPath {
		http.Error(w, "OK", http.StatusOK)
		return
	}
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
	payload, err := github.ValidatePayload(r, h.Config.Secret)
	if err != nil {
		return &handlerResponse{http.StatusBadRequest, "Invalid payload"}, err
	}
	defer r.Body.Close()
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return &handlerResponse{http.StatusBadRequest, "Couldn't parse webhook"}, err
	}
	return h.HandleEvent(r.Context(), event)
}

// Handler handles a webhook.
// We have to use interface{} because of https://github.com/google/go-github/issues/1154.
func (h *Handler) HandleEvent(ctx context.Context, ev interface{}) (*handlerResponse, error) {
	event, err := ParseEvent(ev)
	if err != nil {
		return &handlerResponse{http.StatusBadRequest, "Invalid/unsupported event"}, err
	}
	logger := log.With(h.Logger, "revision", event.Revision, "ref", event.Ref)

	if h.Config.IgnoreRefRegex != nil && h.Config.IgnoreRefRegex.MatchString(event.Ref) {
		level.Debug(logger).Log("msg", "Ref is ignored, skipping", "regex", h.Config.IgnoreRefRegex)
		return &handlerResponse{message: "Ref is ignored, skipping"}, nil
	}

	obj, err := h.Loader.Load(ctx, *event.Repository.FullName, h.Config.ResourcePath, event.Revision)
	if err != nil {
		return &handlerResponse{message: "Couldn't downlaod manifest"}, err
	}

	annotations := event.Annotations()
	if err := meta.NewAccessor().SetAnnotations(obj, annotations); err != nil {
		level.Error(logger).Log("msg", "Couldn't set annotations", "err", err)
	}
	level.Info(logger).Log("msg", "Downloaded manifest succesfully")
	if h.Config.DryRun {
		level.Info(logger).Log("msg", "Dry run enabled, skipping apply", "obj", fmt.Sprintf("%s", obj))
		return nil, nil
	}
	if err := h.KubernetesClient.Apply(obj, h.Config.Namespace); err != nil {
		return &handlerResponse{message: "Couldn't apply resource"}, err
	}

	return nil, nil
}

type handlerResponse struct {
	status  int
	message string
}
