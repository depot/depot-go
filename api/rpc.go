package api

import (
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/depot/depot-go/proto/depot/cli/v1/cliv1connect"
	"github.com/depot/depot-go/proto/depot/registry/v1beta1/registryv1beta1connect"
)

func NewBuildClient() cliv1connect.BuildServiceClient {
	baseURL := os.Getenv("DEPOT_API_URL")
	if baseURL == "" {
		baseURL = "https://api.depot.dev"
	}
	return cliv1connect.NewBuildServiceClient(http.DefaultClient, baseURL, WithUserAgent())
}

// NewRegistryClient returns a client for the public Registry API, including image
// retention policies. Authenticate each request with WithAuthentication and an
// organization token. DEPOT_API_URL overrides the default API endpoint.
func NewRegistryClient() registryv1beta1connect.RegistryServiceClient {
	baseURL := os.Getenv("DEPOT_API_URL")
	if baseURL == "" {
		baseURL = "https://api.depot.dev"
	}
	return registryv1beta1connect.NewRegistryServiceClient(http.DefaultClient, baseURL, WithUserAgent())
}

func WithAuthentication[T any](req *connect.Request[T], token string) *connect.Request[T] {
	req.Header().Add("Authorization", "Bearer "+token)
	return req
}
