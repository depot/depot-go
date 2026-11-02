package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	corev1 "github.com/depot/depot-go/proto/depot/core/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestExampleOperations(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		method   string
		input    proto.Message
		response proto.Message
		want     string
	}{
		{"list", []string{"list", "depot-project"}, "ListTrustPolicies", &corev1.ListTrustPoliciesRequest{ProjectId: "depot-project"}, &corev1.ListTrustPoliciesResponse{TrustPolicies: []*corev1.TrustPolicy{policy(true)}, NextPageToken: proto.String("next")}, `{"trustPolicies":[{"trustPolicyId":"policy-1","gitlab":{"namespaceId":"123","projectId":"456","protectedRefsOnly":true}}],"nextPageToken":"next"}`},
		{"next page", []string{"list", "depot-project", "next"}, "ListTrustPolicies", &corev1.ListTrustPoliciesRequest{ProjectId: "depot-project", PageToken: proto.String("next")}, &corev1.ListTrustPoliciesResponse{}, `{"trustPolicies":[]}`},
		{"add", []string{"add", "depot-project", "123", "456", "true"}, "AddTrustPolicy", addRequest(true), &corev1.AddTrustPolicyResponse{TrustPolicy: policy(true)}, `{"trustPolicyId":"policy-1","gitlab":{"namespaceId":"123","projectId":"456","protectedRefsOnly":true}}`},
		{"duplicate preserves restrictions", []string{"add", "depot-project", "123", "456", "false"}, "AddTrustPolicy", addRequest(false), &corev1.AddTrustPolicyResponse{TrustPolicy: policy(true)}, `{"trustPolicyId":"policy-1","gitlab":{"namespaceId":"123","projectId":"456","protectedRefsOnly":true}}`},
		{"update false", []string{"update", "depot-project", "policy-1", "false"}, "UpdateTrustPolicy", &corev1.UpdateTrustPolicyRequest{ProjectId: "depot-project", TrustPolicyId: "policy-1", ProtectedRefsOnly: proto.Bool(false)}, &corev1.UpdateTrustPolicyResponse{TrustPolicy: policy(false)}, `{"trustPolicyId":"policy-1","gitlab":{"namespaceId":"123","projectId":"456","protectedRefsOnly":false}}`},
		{"remove", []string{"remove", "depot-project", "policy-1"}, "RemoveTrustPolicy", &corev1.RemoveTrustPolicyRequest{ProjectId: "depot-project", TrustPolicyId: "policy-1"}, &corev1.RemoveTrustPolicyResponse{}, "Trust policy removed\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/depot.core.v1.ProjectService/"+tc.method || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("incorrect operation or authentication")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				got := tc.input.ProtoReflect().New().Interface()
				if err := proto.Unmarshal(body, got); err != nil {
					t.Error(err)
					return
				}
				if !proto.Equal(got, tc.input) {
					t.Errorf("input = %v, want %v", got, tc.input)
				}
				data, err := proto.Marshal(tc.response)
				if err != nil {
					t.Error(err)
					return
				}
				w.Header().Set("Content-Type", "application/proto")
				_, _ = w.Write(data)
			}))
			defer server.Close()
			t.Setenv("DEPOT_API_URL", server.URL)
			var output bytes.Buffer
			if err := run(context.Background(), tc.args, "test-token", &output); err != nil {
				t.Fatal(err)
			}
			if tc.method == "RemoveTrustPolicy" {
				if output.String() != tc.want {
					t.Fatalf("output = %q", output.String())
				}
			} else {
				var got, want any
				if err := json.Unmarshal(output.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("output = %s", output.String())
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("calls = %d", calls.Load())
			}
		})
	}
}
func policy(restricted bool) *corev1.TrustPolicy {
	return &corev1.TrustPolicy{TrustPolicyId: "policy-1", Provider: &corev1.TrustPolicy_Gitlab{Gitlab: &corev1.TrustPolicy_GitLab{NamespaceId: "123", ProjectId: "456", ProtectedRefsOnly: restricted}}}
}
func addRequest(restricted bool) *corev1.AddTrustPolicyRequest {
	return &corev1.AddTrustPolicyRequest{ProjectId: "depot-project", Provider: &corev1.AddTrustPolicyRequest_Gitlab{Gitlab: policy(restricted).GetGitlab()}}
}
func TestExampleRejectsInvalidInputBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	t.Setenv("DEPOT_API_URL", server.URL)
	for _, args := range [][]string{
		{}, {"list"}, {"list", " "}, {"list", "project", "page", "extra"},
		{"add", "project", "123", "456"}, {"add", "project", "acme", "app", "true"}, {"add", "project", "123", "456", "yes"},
		{"update", "project", "policy", "FALSE"}, {"update", "project", "policy"},
		{"remove", "project"}, {"remove", "project", "policy", "extra"}, {"unknown", "project"},
	} {
		var output bytes.Buffer
		if err := run(context.Background(), args, "test-token", &output); err == nil {
			t.Fatalf("accepted %v", args)
		}
		if output.Len() != 0 {
			t.Fatal("unexpected output")
		}
	}
	if err := run(context.Background(), []string{"list", "project"}, "", io.Discard); err == nil {
		t.Fatal("accepted missing token")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input reached API")
	}
}
func TestExampleAPIRejectionAndMissingPolicy(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status == http.StatusForbidden {
					_, _ = io.WriteString(w, `{"code":"permission_denied","message":"private request details"}`)
				} else {
					data, err := protojson.Marshal(&corev1.UpdateTrustPolicyResponse{})
					if err != nil {
						t.Error(err)
						return
					}
					_, _ = w.Write(data)
				}
			}))
			defer server.Close()
			t.Setenv("DEPOT_API_URL", server.URL)
			var output bytes.Buffer
			err := run(context.Background(), []string{"update", "project", "policy", "false"}, "test-token", &output)
			if err == nil || output.Len() != 0 || calls.Load() != 1 {
				t.Fatalf("expected one rejected operation with no output; err=%v calls=%d", err, calls.Load())
			}
		})
	}
}
