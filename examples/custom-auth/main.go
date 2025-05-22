package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	// You can use a context with timeout to cancel the build if you would like.
	ctx := context.Background()

	// Set these environment variables...
	token := os.Getenv("DEPOT_TOKEN")
	project := os.Getenv("DEPOT_PROJECT_ID")

	// ... and set these variables.
	dockerfilePath := "./Dockerfile"
	workingDir := "."
	imageTag := "AWS_ACCOUNT_ID_HERE.dkr.ecr.us-east-1.amazonaws.com/REPO_HERE:TAG_HERE"

	// 1. Register a new build.  This returns back an id and a temporary build token.
	req := &cliv1.CreateBuildRequest{
		ProjectId: project,
	}
	build, err := build.NewBuild(ctx, req, token)
	if err != nil {
		log.Fatal(err)
	}

	// Set the buildErr to any error that represents the build failing.
	var buildErr error
	defer build.Finish(buildErr)

	// 2. Acquire a buildkit machine.
	var buildkit *machine.Machine
	buildkit, buildErr = machine.Acquire(ctx, build.ID, build.Token, "arm64" /* or "amd64" */)
	if buildErr != nil {
		return
	}
	defer buildkit.Release()

	connectCtx, cancelConnect := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelConnect()

	var buildkitClient *client.Client
	buildkitClient, buildErr = buildkit.Connect(connectCtx)
	if buildErr != nil {
		return
	}

	solverOptions := client.SolveOpt{
		Frontend: "dockerfile.v0", // Interpret the build as a Dockerfile.
		FrontendAttrs: map[string]string{
			"filename": filepath.Base(dockerfilePath),
			"platform": "linux/arm64", // Build for arm64 architecture.
		},
		LocalDirs: map[string]string{
			"dockerfile": filepath.Dir(dockerfilePath),
			"context":    workingDir,
		},
		Exports: []client.ExportEntry{
			{
				Type: "image",
				Attrs: map[string]string{
					"oci-mediatypes": "true",
					"push":           "true",   // Push the image to the registry...
					"name":           imageTag, // ... with this tag.
				},
			},
		},
		Session: []session.Attachable{
			&EnvAuth{},
		},
	}

	// 3. Print all build status updates as JSON to stdout.
	buildStatusCh := make(chan *client.SolveStatus, 10)
	go func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		for status := range buildStatusCh {
			_ = enc.Encode(status)
		}
	}()

	// 4. Build and push the image.
	_, buildErr = buildkitClient.Solve(ctx, nil, solverOptions, buildStatusCh)
	if buildErr != nil {
		return
	}
}

// EnvAuth is a custom auth provider that uses environment variables to provide registry credentials.
// Uses REGISTRY_USERNAME and REGISTRY_TOKEN environment variables.
type EnvAuth struct{}

// In BuildKit an Attachable is a client-side gRPC server that the build server can connect to.
// BuildKit tunnels gRPC over gRPC, so the client-side can be dialed by the server-side.
var _ session.Attachable = (*EnvAuth)(nil)

// Register hosts an AuthServer on the client-side for the build server.
func (ap *EnvAuth) Register(server *grpc.Server) {
	auth.RegisterAuthServer(server, ap)
}

// AuthServer is not documented in BuildKit, but these are functions called by the build server.
var _ auth.AuthServer = (*EnvAuth)(nil)

// For AWS ECR username is `AWS` and for password run `aws ecr get-login-password --region YOUR_REGION`.
func (ap *EnvAuth) Credentials(ctx context.Context, req *auth.CredentialsRequest) (*auth.CredentialsResponse, error) {
	// If base image is at docker return empty creds to use public download.
	if req.Host == "registry-1.docker.io" {
		return &auth.CredentialsResponse{}, nil
	}

	username := os.Getenv("REGISTRY_USERNAME")
	registryPassword := os.Getenv("REGISTRY_PASSWORD")

	return &auth.CredentialsResponse{
		Username: username,
		Secret:   registryPassword,
	}, nil
}

// GetTokenAuthority needs to return an Unimplemented or a nil public key in order for the Credentials function to be called.
func (ap *EnvAuth) GetTokenAuthority(ctx context.Context, req *auth.GetTokenAuthorityRequest) (*auth.GetTokenAuthorityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Info not implemented")
}

func (ap *EnvAuth) VerifyTokenAuthority(ctx context.Context, req *auth.VerifyTokenAuthorityRequest) (*auth.VerifyTokenAuthorityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Info not implemented")
}

func (ap *EnvAuth) FetchToken(ctx context.Context, req *auth.FetchTokenRequest) (rr *auth.FetchTokenResponse, err error) {
	return nil, status.Errorf(codes.Unimplemented, "method Info not implemented")
}
