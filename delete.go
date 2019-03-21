package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/google/go-github/v24/github"
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
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/itskoko/k8s-webhook-handler/git"
)

type DeleteHandler struct {
	logger log.Logger
	DryRun bool
	discovery.DiscoveryInterface
	dynamic.Interface
	v1.NamespaceInterface
	*kubernetes.Clientset
	SelectorKey string
}

func NewDeleteHandler(logger log.Logger, kconfig *rest.Config, selectorKey string, dryRun bool) (*DeleteHandler, error) {
	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		return nil, err
	}

	intf, err := dynamic.NewForConfig(kconfig)
	if err != nil {
		return nil, err
	}

	return &DeleteHandler{
		logger:             logger,
		DryRun:             dryRun,
		Interface:          intf,
		Clientset:          clientset,
		DiscoveryInterface: clientset.Discovery(),
		NamespaceInterface: clientset.CoreV1().Namespaces(),
		SelectorKey:        selectorKey,
	}, nil
}

func (p *DeleteHandler) NewSelector(val string) (labels.Selector, error) {
	req, err := labels.NewRequirement(p.SelectorKey, selection.Equals, []string{val})
	if err != nil {
		// Should never happen
		return nil, err
	}
	return labels.NewSelector().Add(*req), nil
}

func (p *DeleteHandler) APIResources() ([]*metav1.APIResourceList, error) {
	preferredResources, err := p.DiscoveryInterface.ServerPreferredResources()
	if err != nil {
		return nil, err
	}
	return discovery.FilteredBy(
		discovery.ResourcePredicateFunc(func(groupVersion string, r *metav1.APIResource) bool {
			return discovery.SupportsAllVerbs{Verbs: []string{"list", "create"}}.Match(groupVersion, r)
		}),
		preferredResources,
	), nil
}

type resourceHandlerFn func(resource runtime.Unstructured, client dynamic.ResourceInterface) error

func (p *DeleteHandler) HandleResources(namespace string, selector labels.Selector, handler resourceHandlerFn) ([]metav1.Object, error) {
	resourceLists, err := p.APIResources()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	unhandled := []metav1.Object{}
	for _, resourceList := range resourceLists {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, resource := range resourceList.APIResources {
			if !resource.Namespaced {
				// Skip non-namespaced resources
				continue
			}

			uh, err := p.findAndHandleResource(&gv, &resource, selector, namespace, handler)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			unhandled = append(unhandled, uh...)
		}
	}
	return unhandled, nil
}

func (p *DeleteHandler) FindResources(namespace string) ([]runtime.Unstructured, error) {
	req, err := labels.NewRequirement(p.SelectorKey, selection.Exists, nil)
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*req)

	resources := []runtime.Unstructured{}
	unhandled, err := p.HandleResources(namespace, selector, func(resource runtime.Unstructured, client dynamic.ResourceInterface) error {
		resources = append(resources, resource)
		return nil
	})
	level.Debug(logger).Log("msg", "Unhandled objects", "objects", unhandled)
	return resources, err
}

func (p *DeleteHandler) FindResourcesAll() ([]runtime.Unstructured, error) {
	resources := []runtime.Unstructured{}
	namespaces, err := p.Clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, namespace := range namespaces.Items {
		res, err := p.FindResources(namespace.ObjectMeta.Name)
		if err != nil {
			return nil, err
		}
		resources = append(resources, res...)
	}
	return resources, nil
}

func (p *DeleteHandler) PurgeBranchless() error {
	req, err := labels.NewRequirement(p.SelectorKey, selection.Exists, nil)
	if err != nil {
		return err
	}
	selector := labels.NewSelector().Add(*req)

	namespaces, err := p.Clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	level.Debug(logger).Log("msg", "Found namespaces", "selector", selector.String(), "namespaces", fmt.Sprintf("%v", namespaces.Items))
	for _, namespace := range namespaces.Items {
		logger := log.With(logger, "namespace", namespace.ObjectMeta.Name, "selector", selector.String())
		if _, ok := namespace.GetAnnotations()[p.SelectorKey]; !ok {
			level.Debug(logger).Log("msg", "namespace not tagged, skipping")
			continue
		}
		namespaceInUse := false
		_, err := p.HandleResources(namespace.ObjectMeta.Name, selector, func(resource runtime.Unstructured, client dynamic.ResourceInterface) error {
			metadata, err := meta.Accessor(resource)
			if err != nil {
				return err
			}
			ls := labels.Set(metadata.GetLabels())
			name := metadata.GetName()
			repo := fmt.Sprintf("git@github.com:%s.git", labelValueToRepo(ls.Get(p.SelectorKey)))
			logger := log.With(logger, "name", name, "self-link", metadata.GetSelfLink(), "repo", repo)

			exists, err := git.BranchExists(repo, namespace.ObjectMeta.Name)
			if err != nil {
				return errors.WithStack(err)
			}
			if exists {
				level.Debug(logger).Log("msg", "branch still exists")
				namespaceInUse = true
			} else {
				level.Debug(logger).Log("msg", "branch not found, deleting resource")
				if err := p.deleteResource(resource, client); err != nil {
					return errors.WithStack(err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !namespaceInUse {
			p.deleteNamespace(namespace.ObjectMeta.Name)
		}
	}
	return nil
}

func repoToLabelValue(repo string) string {
	return strings.Replace(repo, "/", ".", -1) // label values may not contain /
}

func labelValueToRepo(lv string) string {
	return strings.Replace(lv, ".", "/", -1)
}

func (p *DeleteHandler) Handle(_ context.Context, ev *github.DeleteEvent) (*handlerResponse, error) {
	var (
		repo   = *ev.Repo.FullName
		branch = *ev.Ref
	)
	if *ev.RefType != "branch" {
		level.Info(p.logger).Log("msg", "Ignoring delete event for refType", "refType", *ev.RefType)
		return &handlerResponse{http.StatusOK, "Nothing to do"}, nil
	}

	// Map repo to label selector value and branch to namespace
	selectorVal := repoToLabelValue(repo)
	namespace := branch
	selector, err := p.NewSelector(selectorVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	retained, err := p.HandleResources(namespace, selector, p.deleteResource)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resourcesLabeled := 0
	for _, metadata := range retained {
		ls := labels.Set(metadata.GetLabels())
		if ls.Has(p.SelectorKey) {
			resourcesLabeled++
			level.Debug(logger).Log("msg", "Found resource labeled for other repo", "repo", ls.Get(p.SelectorKey))
		}
	}
	if resourcesLabeled > 0 {
		msg := "Found resources labeled for other repos, keeping namespace"
		level.Info(logger).Log("msg", msg, "resourcesLabeled", resourcesLabeled)
		return &handlerResponse{http.StatusOK, msg}, nil
	}
	if err := p.deleteNamespace(namespace); err != nil {
		return &handlerResponse{http.StatusInternalServerError, "Couldn't delete namespace"}, err
	}
	return &handlerResponse{http.StatusOK, "Namespace deleted succesfully"}, nil
}

func (p *DeleteHandler) deleteNamespace(namespace string) error {
	level.Debug(logger).Log("msg", "Deleting empty namespace", "namespace", namespace)
	if p.DryRun {
		return nil
	}
	// Namespaces need to be deleted in the background.
	propagationPolicy := metav1.DeletePropagationOrphan
	return p.NamespaceInterface.Delete(namespace, &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
}

func (p *DeleteHandler) findAndHandleResource(gv *schema.GroupVersion, resource *metav1.APIResource, selector labels.Selector, namespace string, handlerFn resourceHandlerFn) ([]metav1.Object, error) {
	logger := log.With(logger, "namespace", namespace, "selector", selector)
	level.Debug(logger).Log("msg", "Getting resources")
	rclient := p.Interface.Resource(gv.WithResource(resource.Name)).Namespace(namespace)
	/*
		if err != nil {
			return nil, fmt.Errorf("Couldn't create client for GroupVersionKind: %s", err)
		}*/
	//rclient := client.Resource(resource, namespace)
	list, err := rclient.List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Couldn't List resources: %s", err)
	}
	resources, err := meta.ExtractList(list)
	if err != nil {
		return nil, fmt.Errorf("Couldn't extract list: %s", err)
	}
	unhandled := []metav1.Object{}
	for _, resource := range resources {
		unstructured, ok := resource.(runtime.Unstructured)
		if !ok {
			return nil, fmt.Errorf("Unexpected type for %v", resource)
		}
		metadata, err := meta.Accessor(unstructured)
		if err != nil {
			return nil, fmt.Errorf("Couldn't get metadata for %v: %s", unstructured, err)
		}
		if selector.Matches(labels.Set(metadata.GetLabels())) {
			if err := handlerFn(unstructured, rclient); err != nil {
				return nil, fmt.Errorf("Handler failed: %s", err)
			}
		} else {
			unhandled = append(unhandled, metadata)
		}
	}
	return unhandled, nil
}

// deleteResource implements resourceHandlerFn
func (p *DeleteHandler) deleteResource(resource runtime.Unstructured, client dynamic.ResourceInterface) error {
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
