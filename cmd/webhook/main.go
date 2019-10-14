package main

import (
	"errors"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics/statsd"

	handler "github.com/itskoko/k8s-webhook-handler"
)

var (
	listenAddr  = flag.String("l", ":8080", "Address to listen on for webhook requests")
	namespace   = flag.String("ns", "ci", "Namespace to deploy workflows to")
	resoucePath = flag.String("p", ".ci/workflow.yaml", "Path to resource manifest in repository")
	kubeconfig  = flag.String("kubeconfig", "", "If set, use this kubeconfig to connect to kubernetes")
	baseURL     = flag.String("gh-base-url", "", "GitHub Enterprise: Base URL")
	uploadURL   = flag.String("gh-upload-url", "", "GitHub Enterprise: Upload URL")
	gitAddress  = flag.String("git", "git@github.com", "Git address")
	debug       = flag.Bool("debug", false, "Enable debug logging")
	insecure    = flag.Bool("insecure", false, "Allow omitting WEBHOOK_SECRET for testing")

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
	githubSecret := os.Getenv("WEBHOOK_SECRET")
	if githubSecret == "" && !*insecure {
		fatal(errors.New("WEBHOOK_SECRET not set. Use -insecure to disable webhook verification"))
	}
	if *debug {
		logger = level.NewFilter(logger, level.AllowAll())
	} else {
		logger = level.NewFilter(logger, level.AllowInfo())
	}

	kClient, err := handler.NewKubernetesClient(*kubeconfig)
	if err != nil {
		fatal(err)
	}

	loader, err := handler.NewGithubLoader(os.Getenv("GITHUB_TOKEN"), *baseURL, *uploadURL)
	if err != nil {
		fatal(err)
	}

	config := &handler.Config{
		Namespace:    *namespace,
		ResourcePath: *resoucePath,
		Secret:       []byte(githubSecret),
	}

	ticker := time.NewTicker(*statsdInterval)
	defer ticker.Stop()
	statsdClient := statsd.New("k8s-ci-purger.", logger)
	go statsdClient.SendLoop(ticker.C, *statsdProto, *statsdAddress)

	server := handler.NewGithubHookHandler(logger, config, kClient, loader, statsdClient)

	http.Handle("/", server)
	level.Info(logger).Log("msg", "Start listening", "addr", *listenAddr)
	fatal(http.ListenAndServe(*listenAddr, nil))
}
