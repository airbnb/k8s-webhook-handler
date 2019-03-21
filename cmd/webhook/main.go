package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics/statsd"
	"github.com/pkg/errors"

	handler "github.com/itskoko/k8s-webhook-handler"
)

var (
	listenAddr        = flag.String("l", ":8080", "Address to listen on for webhook requests")
	sourceSelectorKey = flag.String("sk", "ci-source-repo", "Label key that identifies source repo")
	namespace         = flag.String("ns", "stage", "Namespace to use when -source-selector is given")
	kubeconfig        = flag.String("kubeconfig", "", "If set, use this kubeconfig to connect to kubernetes")
	dryRun            = flag.Bool("dry", false, "Enable dry-run, print resources instead of deleting them")
	debug             = flag.Bool("debug", false, "Enable debug logging")

	statsdAddress  = flag.String("statsd.address", "localhost:8125", "Address to send statsd metrics to")
	statsdProto    = flag.String("statsd.proto", "udp", "Protocol to use for statsd")
	statsdInterval = flag.Duration("statsd.interval", 30*time.Second, "statsd flush interval")

	logger = log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), "caller", log.Caller(5))
)

func fatal(err error) {
	// FIXME: override caller, not add it
	logger := log.With(logger, "caller", log.Caller(4))
	level.Error(logger).Log("msg", err.Error())
	os.Exit(1)
}

func main() {
	flag.Parse()
	githubSecret := os.Getenv("GITHUB_SECRET")
	if githubSecret == "" {
		fatal(errors.New("GITHUB_SECRET env variable required"))
	}
	if *debug {
		logger = level.NewFilter(logger, level.AllowAll())
	} else {
		logger = level.NewFilter(logger, level.AllowInfo())
	}
	dh, err := handler.NewDeleteHandler(logger, *kubeconfig, *sourceSelectorKey, *dryRun)
	if err != nil {
		fatal(err)
	}

	ticker := time.NewTicker(*statsdInterval)
	defer ticker.Stop()
	statsdClient := statsd.New("k8s-ci-purger.", logger)
	go statsdClient.SendLoop(ticker.C, *statsdProto, *statsdAddress)

	http.Handle("/", handler.NewGithubHook(dh, []byte(githubSecret), statsdClient))
	fatal(http.ListenAndServe(*listenAddr, nil))
}
