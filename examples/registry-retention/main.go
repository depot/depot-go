package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"connectrpc.com/connect"
	"github.com/depot/depot-go/api"
	registryv1beta1 "github.com/depot/depot-go/proto/depot/registry/v1beta1"
	"google.golang.org/protobuf/proto"
)

func main() {
	if err := run(); err != nil {
		// Don't print raw errors that could contain credentials or organization IDs.
		fmt.Fprintln(os.Stderr, "Retention operation failed. Set DEPOT_TOKEN to an organization token and use: registry-retention <get|keep-count|keep-days|delete> <repository|**> [positive-integer]")
		os.Exit(1)
	}
}

func run() error {
	token := os.Getenv("DEPOT_TOKEN")
	if token == "" || len(os.Args) < 3 {
		return fmt.Errorf("missing token or arguments")
	}
	action, repository := os.Args[1], os.Args[2]
	client := api.NewRegistryClient()
	ctx := context.Background()
	switch action {
	case "get":
		resp, err := client.GetRetentionPolicy(ctx, api.WithAuthentication(connect.NewRequest(&registryv1beta1.GetRetentionPolicyRequest{Repository: repository}), token))
		if err != nil {
			return err
		}
		if resp.Msg.Policy == nil {
			fmt.Println("No explicit policy")
		} else if resp.Msg.Policy.KeepCount != nil {
			fmt.Printf("Keep count: %d\n", *resp.Msg.Policy.KeepCount)
		} else if resp.Msg.Policy.KeepDays != nil {
			fmt.Printf("Keep days: %d\n", *resp.Msg.Policy.KeepDays)
		}
	case "delete":
		_, err := client.DeleteRetentionPolicy(ctx, api.WithAuthentication(connect.NewRequest(&registryv1beta1.DeleteRetentionPolicyRequest{Repository: repository}), token))
		if err != nil {
			return err
		}
		fmt.Println("Explicit policy removed")
	case "keep-count", "keep-days":
		if len(os.Args) != 4 {
			return fmt.Errorf("missing retention value")
		}
		keep, err := strconv.ParseInt(os.Args[3], 10, 32)
		if err != nil || keep <= 0 {
			return fmt.Errorf("invalid retention value")
		}
		req := &registryv1beta1.UpsertRetentionPolicyRequest{Repository: repository, Enabled: true}
		if action == "keep-count" {
			req.KeepCount = proto.Int32(int32(keep))
		} else {
			req.KeepDays = proto.Int32(int32(keep))
		}
		if _, err := client.UpsertRetentionPolicy(ctx, api.WithAuthentication(connect.NewRequest(req), token)); err != nil {
			return err
		}
		fmt.Println("Retention policy saved")
	default:
		return fmt.Errorf("unknown action")
	}
	return nil
}
