package auth

import (
	"context"
	"errors"
	"os"

	"github.com/depot/depot-go/internal/config"
	"github.com/depot/depot-go/internal/oidc"
	"github.com/depot/depot-go/logger"
)

var ErrNoTokenFound = errors.New("no token found")

func ResolveToken(ctx context.Context, token string) (string, error) {

	if token == "" {
		token = resolveTokenFromEnv()
	}

	if token == "" {
		token = resolveTokenFromConfig()
	}

	if token == "" {
		token = resolveTokenFromOIDC(ctx)
	}

	if token == "" {
		return "", ErrNoTokenFound
	}

	return token, nil
}

func resolveTokenFromEnv() string {
	return os.Getenv("DEPOT_TOKEN")
}

func resolveTokenFromConfig() string {
	return config.GetApiToken()
}

func resolveTokenFromOIDC(ctx context.Context) string {
	for _, provider := range oidc.Providers {
		logger.DebugContext(ctx, "Trying OIDC provide", "provider", provider.Name())

		token, err := provider.RetrieveToken(ctx)

		if err != nil {
			logger.DebugContext(ctx, "OIDC provider failed", "provider", provider.Name(), "error", err)
		}

		if token != "" {
			return token
		}
	}

	return ""
}
