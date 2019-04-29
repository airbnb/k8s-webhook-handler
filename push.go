package handler

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/google/go-github/v24/github"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

const annotationPrefix = "k8s-webhook-handler.io/"

type PushHandler struct {
	logger   log.Logger
	ghClient *github.Client
	dynamic.Interface
	*kubernetes.Clientset
	secret       []byte
	ResourcePath string
	Namespace    string
	meta.RESTMapper
}

func NewPushHandler(logger log.Logger, kconfig *rest.Config, ghClient *github.Client) (*PushHandler, error) {
	intf, err := dynamic.NewForConfig(kconfig)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		return nil, err
	}

	groupResources, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return nil, err
	}

	return &PushHandler{
		logger:       logger,
		Interface:    intf,
		ghClient:     ghClient,
		RESTMapper:   restmapper.NewDiscoveryRESTMapper(groupResources),
		ResourcePath: ".ci/workflow.yaml",
	}, nil
}

func (h *PushHandler) Handle(ctx context.Context, event *github.PushEvent) (*handlerResponse, error) {
	logger := log.With(h.logger, "repo", *event.Repo.Owner.Login+"/"+*event.Repo.Name)
	file, err := h.ghClient.Repositories.DownloadContents(
		ctx,
		*event.Repo.Owner.Login,
		*event.Repo.Name,
		h.ResourcePath,
		&github.RepositoryContentGetOptions{
			Ref: *event.HeadCommit.ID,
		})
	if err != nil {
		return nil, fmt.Errorf("Couldn't get file %s from %s/%s at %s: %s", h.ResourcePath, *event.Repo.Owner.Login, *event.Repo.Name, *event.HeadCommit.ID, err)
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Couldn't read file: %s", err)
	}

	jcontent, err := yaml.ToJSON(content)
	if err != nil {
		return nil, fmt.Errorf("Couldn't translate yaml to json: %s", err)
	}
	obj, _, err := unstructured.UnstructuredJSONScheme.Decode(jcontent, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("Couldn't decode manifest: %s", err)
	}
	acr := meta.NewAccessor()
	acr.SetAnnotations(obj, map[string]string{
		annotationPrefix + "ref":       *event.Ref,
		annotationPrefix + "before":    *event.Before,
		annotationPrefix + "revision":  *event.HeadCommit.ID,
		annotationPrefix + "repo_name": *event.Repo.FullName,
		annotationPrefix + "repo_url":  *event.Repo.GitURL,
		annotationPrefix + "repo_ssh":  *event.Repo.SSHURL,
	})
	level.Info(logger).Log("msg", "Downloaded manifest succesfully", "obj", obj, "content", content)

	if err := h.apply(obj); err != nil {
		return &handlerResponse{message: "Couldn't update resource"}, err
	}
	return nil, nil
}

func (h *PushHandler) apply(obj runtime.Object) error {
	switch obj := obj.(type) {
	case *unstructured.Unstructured:
		gvk := obj.GroupVersionKind()
		gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}

		mapping, err := h.RESTMapper.RESTMapping(gk, gvk.Version)
		if err != nil {
			return err
		}
		if _, err := h.Interface.Resource(mapping.Resource).Namespace(h.Namespace).Create(obj, metav1.CreateOptions{}); err != nil {
			return err
		}
		level.Debug(h.logger).Log("Updated object", "obj", fmt.Sprintf("%#v", obj))
	case *unstructured.UnstructuredList:
		return obj.EachListItem(func(o runtime.Object) error { return h.apply(o) })
	}
	return nil
}
