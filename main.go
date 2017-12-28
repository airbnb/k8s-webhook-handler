package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"gopkg.in/go-playground/webhooks.v3/github"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	listenAddr        = flag.String("l", ":8080", "Address to listen on for webhook requests")
	sourceSelectorKey = flag.String("sk", "ci-source-repo", "Label key that identifies source repo")
	sourceSelectorVal = flag.String("sv", "", "If set, delete all resources matching this selector and exit")
	namespace         = flag.String("ns", "stage", "Namespace to use when -source-selector is given")
	kubeconfig        = flag.String("kubeconfig", "", "If set, use this kubeconfig to connect to kubernetes")

	debug  = flag.Bool("debug", false, "Enable debug logging")
	logger = log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), "caller", log.DefaultCaller)
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

	p := &purger{
		Clientset:   clientset,
		ClientPool:  dynamic.NewDynamicClientPool(config),
		selectorKey: *sourceSelectorKey,
	}
	if *sourceSelectorVal != "" {
		if err := p.purge(*sourceSelectorVal, *namespace); err != nil {
			fatal(err)
		}
		os.Exit(0)
	}
	hook := github.New(&github.Config{Secret: os.Getenv("GITHUB_SECRET")})
	hook.RegisterEvents(p.handleDelete, github.DeleteEvent)
}

type purger struct {
	*kubernetes.Clientset
	dynamic.ClientPool
	selectorKey string
}

func (p *purger) purge(repo, branch string) error {
	selector := p.selectorKey + " = " + repo
	level.Debug(logger).Log("msg", "Using selector", "selector", selector)

	preferredResources, err := p.Discovery().ServerPreferredResources()
	if err != nil {
		return err
	}
	resourceLists := discovery.FilteredBy(
		discovery.ResourcePredicateFunc(func(groupVersion string, r *metav1.APIResource) bool {
			return discovery.SupportsAllVerbs{Verbs: []string{"list", "create"}}.Match(groupVersion, r)
		}),
		preferredResources,
	)
	if err != nil {
		return err
	}
	for _, resourceList := range resourceLists {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return errors.WithStack(err)
		}
		for _, resource := range resourceList.APIResources {
			if !resource.Namespaced {
				// Skip non-namespaced resources
				continue
			}

			if err := p.deleteResourceList(&gv, &resource, selector, branch); err != nil {
				return errors.WithStack(err)
			}
		}
	}
	return nil
}

func (p *purger) deleteResourceList(gv *schema.GroupVersion, resource *metav1.APIResource, selector, namespace string) error {
	logger := log.With(logger, "namespace", namespace, "selector", selector)
	level.Debug(logger).Log("msg", "Getting resources to delete")
	// Based on https://github.com/heptio/ark/blob/1210cb36e10c2cd5a27633fc71a920d6eff37052/pkg/client/dynamic.go#L49:
	// > client-go doesn't actually use the kind when getting the dynamic client from the client pool;
	// > it only needs the group and version.
	client, err := p.ClientPool.ClientForGroupVersionKind(gv.WithKind(""))
	if err != nil {
		return fmt.Errorf("Couldn't create client for GroupVersionKind: %s", err)
	}
	rclient := client.Resource(resource, namespace)
	list, err := rclient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("Couldn't List resources: %s", err)
	}
	resources, err := meta.ExtractList(list)
	if err != nil {
		return fmt.Errorf("Couldn't extract list: %s", err)
	}
	for _, resource := range resources {
		unstructured, ok := resource.(runtime.Unstructured)
		if !ok {
			return fmt.Errorf("Unexpected type for %v", resource)
		}
		if err := p.deleteResource(unstructured, rclient); err != nil {
			return fmt.Errorf("Couldn't delete resource: %s", err)
		}
	}
	return nil
}

func (p *purger) deleteResource(resource runtime.Unstructured, client dynamic.ResourceInterface) error {
	metadata, err := meta.Accessor(resource)
	if err != nil {
		return err
	}
	name := metadata.GetName()
	logger := log.With(logger, "name", name, "self-link", metadata.GetSelfLink())
	logger.Log("msg", "Deleting")
	return client.Delete(name, &metav1.DeleteOptions{}) // FIXME: Should we specify PropagationPolicy?
}
