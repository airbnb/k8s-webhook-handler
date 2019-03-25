package main

import (
	"flag"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	handler "github.com/itskoko/k8s-webhook-handler"
)

var (
	sourceSelectorKey = flag.String("sk", "ci-source-repo", "Label key that identifies source repo")
	kubeconfig        = flag.String("kubeconfig", "", "If set, use this kubeconfig to connect to kubernetes")
	dryRun            = flag.Bool("dry", false, "Enable dry-run, print resources instead of deleting them")

	logger = log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), "caller", log.Caller(5))
)

func main() {
	flag.Parse()
	kconfig, err := handler.NewKubernetesConfig(*kubeconfig)
	if err != nil {
		level.Error(logger).Log("msg", "Couldn't create kubernetes config", "err", err)
		os.Exit(1)
	}
	dh, err := handler.NewDeleteHandler(logger, kconfig, *sourceSelectorKey, *dryRun)
	if err != nil {
		level.Error(logger).Log("msg", "Couldn't create handler", "err", err)
		os.Exit(1)
	}
	if err := dh.PurgeBranchless(); err != nil {
		level.Error(logger).Log("msg", "Couldn't delete branchless resources", "err", err)
		os.Exit(1)
	}
}
