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
	"github.com/docker/cli/cli/config"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
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
	imageTag := "goller/depot-example:latest"

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
			// Use credentials sorted by `docker login` command.
			authprovider.NewDockerAuthProvider(config.LoadDefaultConfigFile(os.Stderr), nil),
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
