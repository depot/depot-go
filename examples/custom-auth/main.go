package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/upload/uploadprovider"
)

func main() {
	// You can use a context with timeout to cancel the build if you would like.
	ctx := context.Background()

	// Set these environment variables...
	token := os.Getenv("DEPOT_TOKEN")
	project := os.Getenv("DEPOT_PROJECT_ID")

	/*
	 *
	 * howdy.tar.gz is a compressed tar archive that contains the Dockerfile and
	 * any other files needed to build the image.
	 *
	 */
	r, err := os.Open("howdy.tar.gz")
	if err != nil {
		log.Printf("unable to open file: %v", err)
		return
	}

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

	uploader := uploadprovider.New()
	// Special buildkit URL for HTTP over gRPC over gRPC.
	contextURL := uploader.Add(r)

	echo := llb.Scratch().File(llb.Copy(llb.Local("."), "/", "/"))

	// TODO: right context?
	def, err := echo.Marshal(connectCtx)
	if err != nil {
		log.Printf("unable to marshal LLB definition: %v", err)
		return
	}

	solverOptions := client.SolveOpt{
		Frontend: "dockerfile.v0", // Interpret the build as a Dockerfile.
		FrontendAttrs: map[string]string{
			"platform": "linux/arm64", // Build for arm64 architecture.
			"context":  contextURL,
		},
		Session: []session.Attachable{
			uploader,
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
	_, buildErr = buildkitClient.Solve(ctx, def, solverOptions, buildStatusCh)
	if buildErr != nil {
		return
	}
}
