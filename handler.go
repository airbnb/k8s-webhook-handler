package handler

import (
	"context"
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
	Namespace      string
	ResourcePath   string
	Secret         []byte
	IgnoreRefRegex *regexp.Regexp
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
func (h *Handler) HandleEvent(ctx context.Context, event interface{}) (*handlerResponse, error) {
	switch e := event.(type) {
	case *github.PushEvent:
		if h.Config.IgnoreRefRegex != nil && h.Config.IgnoreRefRegex.MatchString(*e.Ref) {
			level.Debug(h.Logger).Log("msg", "Ref is ignored, skipping", "ref", *e.Ref, "regex", h.Config.IgnoreRefRegex)
			break
		}
		obj, err := h.Loader.Load(ctx, *e.Repo.Owner.Login+"/"+*e.Repo.Name, h.Config.ResourcePath, *e.After)
		if err != nil {
			return &handlerResponse{message: "Couldn't downlaod manifest"}, err
		}
		meta.NewAccessor().SetAnnotations(obj, map[string]string{
			annotationPrefix + "ref":       *e.Ref,
			annotationPrefix + "before":    *e.Before,
			annotationPrefix + "revision":  *e.After,
			annotationPrefix + "repo_name": *e.Repo.FullName,
			annotationPrefix + "repo_url":  *e.Repo.GitURL,
			annotationPrefix + "repo_ssh":  *e.Repo.SSHURL,
		})
		level.Info(logger).Log("msg", "Downloaded manifest succesfully", "obj", obj)
		if err := h.KubernetesClient.Apply(obj, h.Config.Namespace); err != nil {
			return &handlerResponse{message: "Couldn't apply resource"}, err
		}
	case *github.DeleteEvent:
	default:
		return &handlerResponse{http.StatusBadRequest, "Webhook not supported"}, nil
	}
	return nil, nil
}

type handlerResponse struct {
	status  int
	message string
}
