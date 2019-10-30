package handler

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/google/go-github/v24/github"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type Loader interface {
	Load(ctx context.Context, repo, path, ref string) (runtime.Object, error)
}

type GithubLoader struct {
	*github.Client
}

func NewGithubLoader(token, baseURL, uploadURL string) (*GithubLoader, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	if baseURL != "" {
		bu, err := url.Parse(baseURL)
		if err != nil {
			return nil, err
		}
		client.BaseURL = bu
	}
	if uploadURL != "" {
		uu, err := url.Parse(uploadURL)
		if err != nil {
			return nil, err
		}
		client.UploadURL = uu
	}
	return &GithubLoader{client}, nil

}

// Apply downloads a manifest from repo specified by owner and name at given
// ref. Ref and path can be a SHA, branch, or tag.
func (l *GithubLoader) Load(ctx context.Context, repo, path, ref string) (runtime.Object, error) {
	var (
		parts = strings.SplitN(repo, "/", 2)
		owner = parts[0]
		name  = parts[1]
	)
	var options *github.RepositoryContentGetOptions
	if ref != "" {
		options = &github.RepositoryContentGetOptions{
			Ref: ref,
		}
	}

	file, err := l.Client.Repositories.DownloadContents(ctx, owner, name, path, options)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get file %s from %s/%s at %s: %s", path, owner, name, ref, err)
	}
	defer file.Close()
	return Decode(file)
}

// Decode reads a reader and parses the stream as runtime.Object.
func Decode(r io.Reader) (runtime.Object, error) {
	content, err := ioutil.ReadAll(r)
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
	return obj, nil
}
