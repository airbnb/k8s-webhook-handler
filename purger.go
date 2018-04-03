package purger

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// interface so we can mock ClientPool in tests
type clientForGroupVersionKinder interface {
	ClientForGroupVersionKind(kind schema.GroupVersionKind) (dynamic.Interface, error)
}

type Purger struct {
	DryRun bool
	discovery.DiscoveryInterface
	v1.NamespaceInterface
	ClientPool  clientForGroupVersionKinder
	SelectorKey string
}

func configure(kubeconfig string) (config *rest.Config, err error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func New(kubeconfig, selectorKey string, dryRun bool) (*Purger, error) {
	config, err := configure(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Purger{
		DryRun:             dryRun,
		DiscoveryInterface: clientset.Discovery(),
		NamespaceInterface: clientset.CoreV1().Namespaces(),
		ClientPool:         dynamic.NewDynamicClientPool(config),
		SelectorKey:        selectorKey,
	}, nil
}

func (p *Purger) NewSelector(val string) (labels.Selector, error) {
	req, err := labels.NewRequirement(p.SelectorKey, selection.Equals, []string{val})
	if err != nil {
		// Should never happen
		return nil, err
	}
	return labels.NewSelector().Add(*req), nil
}

func (p *Purger) Purge(repo, branch string) error {
	// Map repo to label selector value and branch to namespace
	selectorVal := strings.Replace(repo, "/", ".", -1) // label values may not contain /
	namespace := branch

	preferredResources, err := p.DiscoveryInterface.ServerPreferredResources()
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
		return errors.WithStack(err)
	}
	selector, err := p.NewSelector(selectorVal)
	if err != nil {
		return errors.WithStack(err)
	}
	resourcesLabeled := 0
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

			retained, err := p.deleteResourceList(&gv, &resource, selector, namespace)
			if err != nil {
				return errors.WithStack(err)
			}
			for _, metadata := range retained {
				ls := labels.Set(metadata.GetLabels())
				if ls.Has(p.SelectorKey) {
					resourcesLabeled++
					level.Debug(logger).Log("msg", "Found resource labeled for other repo", "repo", ls.Get(p.SelectorKey))
				}
			}
		}
	}
	if resourcesLabeled > 0 {
		level.Info(logger).Log("msg", "Found resources labeled for other repos, keeping namespace", "resourcesLabeled", resourcesLabeled)
		return nil
	}
	level.Debug(logger).Log("msg", "Deleting empty namespace")
	if p.DryRun {
		return nil
	}
	// Namespaces need to be deleted in the background.
	propagationPolicy := metav1.DeletePropagationBackground
	return p.NamespaceInterface.Delete(namespace, &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
}

func (p *Purger) deleteResourceList(gv *schema.GroupVersion, resource *metav1.APIResource, selector labels.Selector, namespace string) ([]metav1.Object, error) {
	logger := log.With(logger, "namespace", namespace, "selector", selector)
	level.Debug(logger).Log("msg", "Getting resources to delete")
	// Based on https://github.com/heptio/ark/blob/1210cb36e10c2cd5a27633fc71a920d6eff37052/pkg/client/dynamic.go#L49:
	// > client-go doesn't actually use the kind when getting the dynamic client from the client pool;
	// > it only needs the group and version.
	client, err := p.ClientPool.ClientForGroupVersionKind(gv.WithKind(""))
	if err != nil {
		return nil, fmt.Errorf("Couldn't create client for GroupVersionKind: %s", err)
	}
	rclient := client.Resource(resource, namespace)
	list, err := rclient.List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Couldn't List resources: %s", err)
	}
	resources, err := meta.ExtractList(list)
	if err != nil {
		return nil, fmt.Errorf("Couldn't extract list: %s", err)
	}
	retained := []metav1.Object{}
	for _, resource := range resources {
		// metav1.ListOptions{LabelSelector: selector})
		unstructured, ok := resource.(runtime.Unstructured)
		if !ok {
			return nil, fmt.Errorf("Unexpected type for %v", resource)
		}
		metadata, err := meta.Accessor(unstructured)
		if err != nil {
			return nil, fmt.Errorf("Couldn't get metadata for %v: %s", unstructured, err)
		}
		if selector.Matches(labels.Set(metadata.GetLabels())) {
			if err := p.deleteResource(unstructured, rclient); err != nil {
				return nil, fmt.Errorf("Couldn't delete resource: %s", err)
			}
		} else {
			retained = append(retained, metadata)
		}
	}
	return retained, nil
}

func (p *Purger) deleteResource(resource runtime.Unstructured, client dynamic.ResourceInterface) error {
	metadata, err := meta.Accessor(resource)
	if err != nil {
		return err
	}
	name := metadata.GetName()
	logger := log.With(logger, "name", name, "self-link", metadata.GetSelfLink())
	logger.Log("msg", "Deleting")
	if p.DryRun {
		return nil
	}
	propagationPolicy := metav1.DeletePropagationForeground
	return client.Delete(name, &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
}
