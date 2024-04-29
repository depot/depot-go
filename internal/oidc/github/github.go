package github

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/depot/depot-go/internal/oidc/common"
)

type GitHubOIDCProvider struct {
}

func NewGitHubOIDCProvider() *GitHubOIDCProvider {
	return &GitHubOIDCProvider{}
}

func (p *GitHubOIDCProvider) Name() string {
	return "github"
}

func (p *GitHubOIDCProvider) RetrieveToken(ctx context.Context) (string, error) {
	requestToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if requestToken == "" {
		return "", nil
	}

	requestURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	if requestURL == "" {
		return "", nil
	}

	requestURL = requestURL + "&audience=" + common.Audience

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "bearer "+requestToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var payload struct {
		Value string `json:"value"`
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&payload); err != nil {
		return "", err
	}
	return payload.Value, nil
}
