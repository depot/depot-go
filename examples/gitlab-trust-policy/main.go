package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/depot/depot-go/api"
	corev1 "github.com/depot/depot-go/proto/depot/core/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const usage = `Set DEPOT_TOKEN to an organization API token. Usage:
  gitlab-trust-policy list <depot-project-id> [page-token]
  gitlab-trust-policy add <depot-project-id> <namespace-id> <gitlab-project-id> <true|false>
  gitlab-trust-policy update <depot-project-id> <policy-id> <true|false>
  gitlab-trust-policy remove <depot-project-id> <policy-id>`

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv("DEPOT_TOKEN"), os.Stdout); err != nil {
		// Raw API errors may contain private request details.
		fmt.Fprintln(os.Stderr, "Trust policy operation failed. Check the token, project, policy, and GitLab IDs.\n"+usage)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, token string, out io.Writer) error {
	if token == "" || len(args) < 2 {
		return fmt.Errorf("missing token or arguments")
	}
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("empty argument")
		}
	}
	action, projectID := args[0], args[1]
	validBool := func(s string) bool { return s == "true" || s == "false" }
	numeric := func(s string) bool {
		return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) == -1
	}
	switch action {
	case "list":
		if len(args) != 2 && len(args) != 3 {
			return fmt.Errorf("invalid list arguments")
		}
	case "add":
		if len(args) != 5 || !numeric(args[2]) || !numeric(args[3]) || !validBool(args[4]) {
			return fmt.Errorf("invalid add arguments")
		}
	case "update":
		if len(args) != 4 || !validBool(args[3]) {
			return fmt.Errorf("invalid update arguments")
		}
	case "remove":
		if len(args) != 3 {
			return fmt.Errorf("invalid remove arguments")
		}
	default:
		return fmt.Errorf("unknown action")
	}
	client := api.NewProjectClient()
	var message proto.Message
	switch action {
	case "list":
		req := &corev1.ListTrustPoliciesRequest{ProjectId: projectID}
		if len(args) == 3 {
			req.PageToken = proto.String(args[2])
		}
		resp, err := client.ListTrustPolicies(ctx, api.WithAuthentication(connect.NewRequest(req), token))
		if err != nil {
			return err
		}
		message = resp.Msg
	case "add":
		resp, err := client.AddTrustPolicy(ctx, api.WithAuthentication(connect.NewRequest(&corev1.AddTrustPolicyRequest{
			ProjectId: projectID,
			Provider:  &corev1.AddTrustPolicyRequest_Gitlab{Gitlab: &corev1.TrustPolicy_GitLab{NamespaceId: args[2], ProjectId: args[3], ProtectedRefsOnly: args[4] == "true"}},
		}), token))
		if err != nil {
			return err
		}
		if resp.Msg.TrustPolicy == nil {
			return fmt.Errorf("missing policy")
		}
		// Duplicate adds preserve existing restrictions; show the saved policy.
		message = resp.Msg.TrustPolicy
	case "update":
		resp, err := client.UpdateTrustPolicy(ctx, api.WithAuthentication(connect.NewRequest(&corev1.UpdateTrustPolicyRequest{
			ProjectId: projectID, TrustPolicyId: args[2], ProtectedRefsOnly: proto.Bool(args[3] == "true"),
		}), token))
		if err != nil {
			return err
		}
		if resp.Msg.TrustPolicy == nil {
			return fmt.Errorf("missing policy")
		}
		message = resp.Msg.TrustPolicy
	case "remove":
		_, err := client.RemoveTrustPolicy(ctx, api.WithAuthentication(connect.NewRequest(&corev1.RemoveTrustPolicyRequest{ProjectId: projectID, TrustPolicyId: args[2]}), token))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, "Trust policy removed")
		return err
	}
	data, err := (protojson.MarshalOptions{Indent: "  ", EmitUnpopulated: true}).Marshal(message)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}
