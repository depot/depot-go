package oidc

import (
	"context"

	"github.com/depot/depot-go/internal/oidc/actionspublic"
	"github.com/depot/depot-go/internal/oidc/buildkite"
	"github.com/depot/depot-go/internal/oidc/circleci"
	"github.com/depot/depot-go/internal/oidc/github"
)

type OIDCProvider interface {
	Name() string
	RetrieveToken(ctx context.Context) (string, error)
}

var Providers = []OIDCProvider{
	github.NewGitHubOIDCProvider(),
	circleci.NewCircleCIOIDCProvider(),
	buildkite.NewBuildkiteOIDCProvider(),
	actionspublic.NewActionsPublicProvider(),
}
