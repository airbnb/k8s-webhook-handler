package handler

import (
	"context"
	"net/url"
	"os"

	"github.com/google/go-github/v24/github"
	"golang.org/x/oauth2"
)

func NewGitHubClient(token, baseURL, uploadURL string) (*github.Client, error) {
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
	return client, nil
}
