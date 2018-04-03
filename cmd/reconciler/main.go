package main

import (
	"flag"
	"log"

	purger "github.com/itskoko/k8s-ci-purger"
)

var (
	sourceSelectorKey = flag.String("sk", "ci-source-repo", "Label key that identifies source repo")
	kubeconfig        = flag.String("kubeconfig", "", "If set, use this kubeconfig to connect to kubernetes")
	dryRun            = flag.Bool("dry", false, "Enable dry-run, print resources instead of deleting them")
)

func main() {
	flag.Parse()

	p, err := purger.New(*kubeconfig, *sourceSelectorKey, *dryRun)
	if err != nil {
		log.Fatal(err)
	}
	if err := p.PurgeBranchless(); err != nil {
		log.Fatal(err)
	}
}
