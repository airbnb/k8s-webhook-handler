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
	listenAddr        = flag.String("l", ":8080", "Address to listen on for webhook requests")
	sourceSelectorKey = flag.String("sk", "ci-source-repo", "Label key that identifies source repo")
	namespace         = flag.String("ns", "ci", "Namespace to deploy workflows to")
	kubeconfig        = flag.String("kubeconfig", "", "If set, use this kubeconfig to connect to kubernetes")
	dryRun            = flag.Bool("dry", false, "Enable dry-run, print resources instead of deleting them")
	baseURL           = flag.String("gh-base-url", "", "GitHub Enterprise: Base URL")
	uploadURL         = flag.String("gh-upload-url", "", "GitHub Enterprise: Upload URL")
	gitAddress        = flag.String("git", "git@github.com", "Git address")
	debug             = flag.Bool("debug", false, "Enable debug logging")
	insecure          = flag.Bool("insecure", false, "Allow omitting WEBHOOK_SECRET for testing")

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

	kconfig, err := handler.NewKubernetesConfig(*kubeconfig)
	if err != nil {
		fatal(err)
	}

	dh, err := handler.NewDeleteHandler(logger, kconfig, *sourceSelectorKey, *dryRun)
	dh.GitAddress = *gitAddress
	if err != nil {
		fatal(err)
	}

	ghClient, err := handler.NewGitHubClient(os.Getenv("GITHUB_TOKEN"), *baseURL, *uploadURL)
	if err != nil {
		fatal(err)
	}

	ph, err := handler.NewPushHandler(logger, kconfig, ghClient)
	if err != nil {
		fatal(err)
	}
	ph.Namespace = *namespace

	ticker := time.NewTicker(*statsdInterval)
	defer ticker.Stop()
	statsdClient := statsd.New("k8s-ci-purger.", logger)
	go statsdClient.SendLoop(ticker.C, *statsdProto, *statsdAddress)

	h := handler.NewGithubHookHandler([]byte(githubSecret), statsdClient)
	h.DeleteHandler = dh
	h.PushHandler = ph

	http.Handle("/", h)
	level.Info(logger).Log("msg", "Start listening", "addr", *listenAddr)
	fatal(http.ListenAndServe(*listenAddr, nil))
}
