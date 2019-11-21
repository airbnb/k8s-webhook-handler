package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/google/go-github/v24/github"
	handler "github.com/airbnb/k8s-webhook-handler"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func main() {
	var (
		evType   = flag.String("-type", "push", "Event type")
		action   = flag.String("-action", "", "Event action")
		revision = flag.String("-revision", "0000000000000000000000000000000000000000", "Revision")
		ref      = flag.String("-ref", "refs/heads/master", "Ref")
		before   = flag.String("-before", "0000000000000000000000000000000000000000", "Before")
		repoURL  = flag.String("-url", "git://github.com/airbnb/k8s-webhook-handler.git", "git URL")
		sshUser  = flag.String("-ssh-user", "git", "SSH user")
	)
	flag.Parse()
	files := flag.Args()
	if len(files) == 0 {
		log.Fatal("Usage: annotate [flags] file-to-annotate [more-files-to-annotate...]")
	}
	u, err := url.Parse(*repoURL)
	if err != nil {
		log.Fatal(err)
	}

	var (
		sshURL   = fmt.Sprintf("%s@%s:%s", *sshUser, u.Host, u.Path)
		fullName = strings.TrimSuffix(u.Path, ".git")
	)

	repo := &github.Repository{
		FullName: &fullName,
		GitURL:   repoURL,
		SSHURL:   &sshURL,
	}
	annotations := (&handler.Event{
		Type:       *evType,
		Action:     *action,
		Revision:   *revision,
		Ref:        *ref,
		Before:     *before,
		Repository: repo,
	}).Annotations()

	for _, file := range flag.Args() {
		fh, err := os.Open(file)
		if err != nil {
			log.Fatalf("Couldn't read file %s: %s", err)
		}
		defer fh.Close()
		obj, err := handler.Decode(fh)
		meta.NewAccessor().SetAnnotations(obj, annotations)
		if err := unstructured.UnstructuredJSONScheme.Encode(obj, os.Stdout); err != nil {
			log.Fatal(err)
		}
	}
}
