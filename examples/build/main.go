package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/progress/progressui"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

func main() {
	token := os.Getenv("DEPOT_TOKEN")
	project := os.Getenv("DEPOT_PROJECT_ID")

	// You can use a context with timeout to cancel the build if you would like.
	ctx := context.Background()

	// 1. Register a new build.
	req := &cliv1.CreateBuildRequest{
		ProjectId: project,
		Options: []*cliv1.BuildOptions{
			{
				Command: cliv1.Command_COMMAND_BUILD,
				Tags:    []string{"depot/example:latest"},
			},
		},
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
	buildkit, buildErr = machine.Acquire(ctx, build.ID, build.Token, "amd64")
	if buildErr != nil {
		return
	}
	defer buildkit.Release()

	// 3. Check buildkitd readiness. When the buildkitd starts, it may take
	// quite a while to be ready to accept connections when it loads a large boltdb.
	connectCtx, cancelConnect := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelConnect()

	var buildkitClient *client.Client
	buildkitClient, buildErr = buildkit.Connect(connectCtx)
	if buildErr != nil {
		return
	}

	// 4. Use the buildkit client to build the image.
	buildErr = buildImage(ctx, buildkitClient)
	if buildErr != nil {
		return
	}
}

func buildImage(ctx context.Context, buildkitClient *client.Client) error {
	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)

	ops := llb.Image("alpine:latest")
	def, err := ops.Marshal(ctx, llb.LinuxAmd64)
	if err != nil {
		return err
	}

	var res *client.SolveResponse

	eg.Go(func() error {
		opts := client.SolveOpt{
			FrontendAttrs: map[string]string{
				"platform": "linux/amd64",
			},
			Internal: true, // Prevent recording the build steps and traces in buildkit as it is _very_ slow.
		}
		res, err = buildkitClient.Solve(ctx, def, opts, ch)
		return err
	})

	eg.Go(func() error {
		display, err := progressui.NewDisplay(os.Stdout, progressui.AutoMode)
		if err != nil {
			return err
		}

		_, err = display.UpdateFrom(context.TODO(), ch)
		return err
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	for k, encoded := range res.ExporterResponse {
		v, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return err
		}
		log.Printf("exporter response: %v %v\n", k, string(v))
	}
	return nil
}

func RunImage(ctx context.Context, imageName string, args []string, c *client.Client, platform ocispecs.Platform, filename string, fileContent []byte) error {
	buildCallback := func(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
		image := llb.Image(imageName).
			Platform(platform).
			File(
				llb.Mkfile(filename, 0664, fileContent),
				llb.WithCustomName("[internal] lint"),
			)

		def, err := image.Marshal(ctx, llb.Platform(platform))
		if err != nil {
			return nil, err
		}
		imgRef, err := c.Solve(ctx, gateway.SolveRequest{
			Definition: def.ToPB(),
		})
		if err != nil {
			return nil, err
		}

		containerCtx, containerCancel := context.WithCancel(ctx)
		defer containerCancel()
		bkContainer, err := c.NewContainer(containerCtx, gateway.NewContainerRequest{
			Mounts: []gateway.Mount{
				{
					Dest:      "/",
					MountType: pb.MountType_BIND,
					Ref:       imgRef.Ref,
				},
			},
			Platform: &pb.Platform{Architecture: platform.Architecture, OS: platform.OS},
		})
		if err != nil {
			return nil, err
		}

		proc, err := bkContainer.Start(ctx, gateway.StartRequest{
			Args:   args,
			Stdout: os.Stdout,
		})
		if err != nil {
			_ = bkContainer.Release(ctx)
			return nil, err
		}
		_ = proc.Wait()

		return nil, bkContainer.Release(ctx)
	}
	_, err := c.Build(ctx, client.SolveOpt{}, "buildx", buildCallback, nil)

	return err
}
