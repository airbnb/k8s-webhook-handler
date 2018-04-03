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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	purger "github.com/itskoko/k8s-ci-purger"
)

var (
	listenAddr        = flag.String("l", ":8080", "Address to listen on for webhook requests")
	sourceSelectorKey = flag.String("sk", "ci-source-repo", "Label key that identifies source repo")
	sourceSelectorVal = flag.String("sv", "", "If set, delete all resources matching this selector and exit")
	namespace         = flag.String("ns", "stage", "Namespace to use when -source-selector is given")
	kubeconfig        = flag.String("kubeconfig", "", "If set, use this kubeconfig to connect to kubernetes")
	dryRun            = flag.Bool("dry", false, "Enable dry-run, print resources instead of deleting them")
	debug             = flag.Bool("debug", false, "Enable debug logging")

	statsdAddress  = flag.String("statsd.address", "localhost:8125", "Address to send statsd metrics to")
	statsdProto    = flag.String("statsd.proto", "udp", "Protocol to use for statsd")
	statsdInterval = flag.Duration("statsd.interval", 30*time.Second, "statsd flush interval")

	logger = log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), "caller", log.Caller(5))
)

func configure() (config *rest.Config, err error) {
	if *kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", *kubeconfig)
	}
	return rest.InClusterConfig()
}

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
	config, err := configure()
	if err != nil {
		fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fatal(err)
	}

	statsdClient := statsd.New("k8s-ci-purger.", logger)
	p := &purger.Purger{
		DryRun:             *dryRun,
		DiscoveryInterface: clientset.Discovery(),
		NamespaceInterface: clientset.CoreV1().Namespaces(),
		ClientPool:         dynamic.NewDynamicClientPool(config),
		SelectorKey:        *sourceSelectorKey,
	}
	if *sourceSelectorVal != "" {
		if err := p.Purge(*sourceSelectorVal, *namespace); err != nil {
			fatal(err)
		}
		os.Exit(0)
	}

	ticker := time.NewTicker(*statsdInterval)
	defer ticker.Stop()
	go statsdClient.SendLoop(ticker.C, *statsdProto, *statsdAddress)

	http.Handle("/", purger.NewGithubHook(p, []byte(githubSecret), statsdClient))
	fatal(http.ListenAndServe(*listenAddr, nil))
}
