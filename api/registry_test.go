package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"connectrpc.com/connect"
	"github.com/depot/depot-go/api"
	registryv1beta1 "github.com/depot/depot-go/proto/depot/registry/v1beta1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Exercise the public client against a local Connect endpoint. The endpoint
// returns wire responses; production validation remains the API's responsibility.
func registryEndpoint(t *testing.T, method string, want map[string]any, response string, status int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/depot.registry.v1beta1.RegistryService/"+method {
			t.Errorf("unexpected method: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer organization-token" {
			t.Error("missing organization authentication")
		}
		if r.Header.Get("X-Depot-Org") != "" {
			t.Error("client must not choose a tenant")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		// The generated Go client uses protobuf. Decode through the public messages.
		var msg proto.Message
		switch method {
		case "GetRetentionPolicy":
			msg = &registryv1beta1.GetRetentionPolicyRequest{}
		case "UpsertRetentionPolicy":
			msg = &registryv1beta1.UpsertRetentionPolicyRequest{}
		case "DeleteRetentionPolicy":
			msg = &registryv1beta1.DeleteRetentionPolicyRequest{}
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if err := proto.Unmarshal(body, msg); err != nil {
			t.Error(err)
			return
		}
		encoded, err := protojson.Marshal(msg)
		if err != nil {
			t.Error(err)
			return
		}
		var got map[string]any
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("request = %v, want %v", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(response))
			return
		}
		var reply proto.Message
		switch method {
		case "GetRetentionPolicy":
			reply = &registryv1beta1.GetRetentionPolicyResponse{}
		case "UpsertRetentionPolicy":
			reply = &registryv1beta1.UpsertRetentionPolicyResponse{}
		case "DeleteRetentionPolicy":
			reply = &registryv1beta1.DeleteRetentionPolicyResponse{}
		}
		if err := protojson.Unmarshal([]byte(response), reply); err != nil {
			t.Error(err)
			return
		}
		data, err := proto.Marshal(reply)
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/proto")
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	t.Setenv("DEPOT_API_URL", server.URL)
}

func TestRegistryRetentionStrategies(t *testing.T) {
	for _, tc := range []struct {
		name     string
		request  *registryv1beta1.UpsertRetentionPolicyRequest
		want     map[string]any
		response string
	}{
		{"repository count", &registryv1beta1.UpsertRetentionPolicyRequest{Repository: "team/app", Enabled: true, KeepCount: proto.Int32(10)}, map[string]any{"repository": "team/app", "enabled": true, "keepCount": float64(10)}, `{"policy":{"repository":"team/app","enabled":true,"keepCount":10}}`},
		{"organization days", &registryv1beta1.UpsertRetentionPolicyRequest{Repository: "**", Enabled: true, KeepDays: proto.Int32(30)}, map[string]any{"repository": "**", "enabled": true, "keepDays": float64(30)}, `{"policy":{"repository":"**","enabled":true,"keepDays":30}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registryEndpoint(t, "UpsertRetentionPolicy", tc.want, tc.response, http.StatusOK)
			resp, err := api.NewRegistryClient().UpsertRetentionPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(tc.request), "organization-token"))
			if err != nil {
				t.Fatal(err)
			}
			policy := resp.Msg.Policy
			if policy == nil || policy.Repository != tc.request.Repository || !policy.Enabled || !reflect.DeepEqual(policy.KeepCount, tc.request.KeepCount) || !reflect.DeepEqual(policy.KeepDays, tc.request.KeepDays) {
				t.Fatalf("unexpected policy: %v", policy)
			}
		})
	}
}

func TestRegistryMissingPolicy(t *testing.T) {
	registryEndpoint(t, "GetRetentionPolicy", map[string]any{"repository": "team/app"}, `{}`, http.StatusOK)
	resp, err := api.NewRegistryClient().GetRetentionPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(&registryv1beta1.GetRetentionPolicyRequest{Repository: "team/app"}), "organization-token"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.Policy != nil {
		t.Fatal("missing policy must remain absent")
	}
}

func TestRegistryDeletePolicy(t *testing.T) {
	for _, repository := range []string{"team/app", "**"} {
		t.Run(repository, func(t *testing.T) {
			registryEndpoint(t, "DeleteRetentionPolicy", map[string]any{"repository": repository}, `{}`, http.StatusOK)
			_, err := api.NewRegistryClient().DeleteRetentionPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(&registryv1beta1.DeleteRetentionPolicyRequest{Repository: repository}), "organization-token"))
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRegistryInvalidStrategy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		count, days *int32
		want        map[string]any
	}{
		{"empty", nil, nil, map[string]any{"repository": "team/app"}},
		{"mixed", proto.Int32(10), proto.Int32(30), map[string]any{"repository": "team/app", "keepCount": float64(10), "keepDays": float64(30)}},
		{"zero count", proto.Int32(0), nil, map[string]any{"repository": "team/app", "keepCount": float64(0)}},
		{"zero days", nil, proto.Int32(0), map[string]any{"repository": "team/app", "keepDays": float64(0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registryEndpoint(t, "UpsertRetentionPolicy", tc.want, `{"code":"invalid_argument","message":"Invalid retention strategy"}`, http.StatusBadRequest)
			_, err := api.NewRegistryClient().UpsertRetentionPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(&registryv1beta1.UpsertRetentionPolicyRequest{Repository: "team/app", KeepCount: tc.count, KeepDays: tc.days}), "organization-token"))
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}
}

func TestRegistryPermissionDenied(t *testing.T) {
	registryEndpoint(t, "GetRetentionPolicy", map[string]any{"repository": "team/app"}, `{"code":"permission_denied","message":"Permission denied"}`, http.StatusForbidden)
	_, err := api.NewRegistryClient().GetRetentionPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(&registryv1beta1.GetRetentionPolicyRequest{Repository: "team/app"}), "organization-token"))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}
