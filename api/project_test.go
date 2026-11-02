package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/depot/depot-go/api"
	corev1 "github.com/depot/depot-go/proto/depot/core/v1"
	"github.com/depot/depot-go/proto/depot/core/v1/corev1connect"
	"google.golang.org/protobuf/proto"
)

type projectServer struct {
	corev1connect.UnimplementedProjectServiceHandler
	add    func(*corev1.AddTrustPolicyRequest) (*corev1.AddTrustPolicyResponse, error)
	update func(*corev1.UpdateTrustPolicyRequest) (*corev1.UpdateTrustPolicyResponse, error)
	list   func(*corev1.ListTrustPoliciesRequest) (*corev1.ListTrustPoliciesResponse, error)
	remove func(*corev1.RemoveTrustPolicyRequest) (*corev1.RemoveTrustPolicyResponse, error)
}

func (s *projectServer) AddTrustPolicy(_ context.Context, r *connect.Request[corev1.AddTrustPolicyRequest]) (*connect.Response[corev1.AddTrustPolicyResponse], error) {
	m, e := s.add(r.Msg)
	return connect.NewResponse(m), e
}
func (s *projectServer) UpdateTrustPolicy(_ context.Context, r *connect.Request[corev1.UpdateTrustPolicyRequest]) (*connect.Response[corev1.UpdateTrustPolicyResponse], error) {
	m, e := s.update(r.Msg)
	return connect.NewResponse(m), e
}
func (s *projectServer) ListTrustPolicies(_ context.Context, r *connect.Request[corev1.ListTrustPoliciesRequest]) (*connect.Response[corev1.ListTrustPoliciesResponse], error) {
	m, e := s.list(r.Msg)
	return connect.NewResponse(m), e
}
func (s *projectServer) RemoveTrustPolicy(_ context.Context, r *connect.Request[corev1.RemoveTrustPolicyRequest]) (*connect.Response[corev1.RemoveTrustPolicyResponse], error) {
	m, e := s.remove(r.Msg)
	return connect.NewResponse(m), e
}

func projectEndpoint(t *testing.T, s corev1connect.ProjectServiceHandler) *atomic.Int32 {
	t.Helper()
	calls := &atomic.Int32{}
	_, handler := corev1connect.NewProjectServiceHandler(s)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer organization-token" {
			t.Error("missing organization authentication")
		}
		if r.Header.Get("X-Depot-Org") != "" {
			t.Error("unexpected tenant selector")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	t.Setenv("DEPOT_API_URL", server.URL)
	return calls
}
func gitlabPolicy(restricted bool) *corev1.TrustPolicy {
	return &corev1.TrustPolicy{TrustPolicyId: "policy-1", Provider: &corev1.TrustPolicy_Gitlab{Gitlab: &corev1.TrustPolicy_GitLab{NamespaceId: "123", ProjectId: "456", ProtectedRefsOnly: restricted}}}
}

func TestProjectAddTrustPolicyProviders(t *testing.T) {
	for _, provider := range []string{"gitlab", "github", "circleci", "buildkite"} {
		t.Run(provider, func(t *testing.T) {
			req := &corev1.AddTrustPolicyRequest{ProjectId: "depot-project"}
			saved := &corev1.TrustPolicy{TrustPolicyId: "policy-1"}
			switch provider {
			case "gitlab":
				p := &corev1.TrustPolicy_GitLab{NamespaceId: "123", ProjectId: "456", ProtectedRefsOnly: true}
				req.Provider = &corev1.AddTrustPolicyRequest_Gitlab{Gitlab: p}
				saved.Provider = &corev1.TrustPolicy_Gitlab{Gitlab: p}
			case "github":
				p := &corev1.TrustPolicy_GitHub{RepositoryOwner: "acme", Repository: "app", AllowPublicPullRequests: true}
				req.Provider = &corev1.AddTrustPolicyRequest_Github{Github: p}
				saved.Provider = &corev1.TrustPolicy_Github{Github: p}
			case "circleci":
				p := &corev1.TrustPolicy_CircleCI{OrganizationUuid: "org-uuid", ProjectUuid: "project-uuid"}
				req.Provider = &corev1.AddTrustPolicyRequest_Circleci{Circleci: p}
				saved.Provider = &corev1.TrustPolicy_Circleci{Circleci: p}
			case "buildkite":
				p := &corev1.TrustPolicy_Buildkite{OrganizationSlug: "acme", PipelineSlug: "app"}
				req.Provider = &corev1.AddTrustPolicyRequest_Buildkite{Buildkite: p}
				saved.Provider = &corev1.TrustPolicy_Buildkite_{Buildkite: p}
			}
			calls := projectEndpoint(t, &projectServer{add: func(got *corev1.AddTrustPolicyRequest) (*corev1.AddTrustPolicyResponse, error) {
				if !proto.Equal(got, req) {
					t.Errorf("request = %v, want %v", got, req)
				}
				return &corev1.AddTrustPolicyResponse{TrustPolicy: saved}, nil
			}})
			resp, err := api.NewProjectClient().AddTrustPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(req), "organization-token"))
			if err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(resp.Msg.TrustPolicy, saved) {
				t.Fatalf("saved policy = %v", resp.Msg.TrustPolicy)
			}
			if calls.Load() != 1 {
				t.Fatalf("calls = %d", calls.Load())
			}
		})
	}
}

func TestProjectUpdateRestrictionPresence(t *testing.T) {
	for _, provider := range []string{"gitlab", "github"} {
		for _, tc := range []struct {
			name  string
			value *bool
		}{{"omitted", nil}, {"true", proto.Bool(true)}, {"false", proto.Bool(false)}} {
			t.Run(provider+"/"+tc.name, func(t *testing.T) {
				req := &corev1.UpdateTrustPolicyRequest{ProjectId: "depot-project", TrustPolicyId: "policy-1"}
				if provider == "gitlab" {
					req.ProtectedRefsOnly = tc.value
				} else {
					req.AllowPublicPullRequests = tc.value
				}
				enabled := tc.value == nil || *tc.value
				saved := gitlabPolicy(enabled)
				if provider == "github" {
					saved.Provider = &corev1.TrustPolicy_Github{Github: &corev1.TrustPolicy_GitHub{RepositoryOwner: "acme", Repository: "app", AllowPublicPullRequests: enabled}}
				}
				projectEndpoint(t, &projectServer{update: func(got *corev1.UpdateTrustPolicyRequest) (*corev1.UpdateTrustPolicyResponse, error) {
					// proto.Equal distinguishes nil from a pointer to false after a real wire round trip.
					if !proto.Equal(got, req) {
						t.Errorf("request = %v, want %v", got, req)
					}
					return &corev1.UpdateTrustPolicyResponse{TrustPolicy: saved}, nil
				}})
				resp, err := api.NewProjectClient().UpdateTrustPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(req), "organization-token"))
				if err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(resp.Msg.TrustPolicy, saved) {
					t.Fatalf("policy = %v", resp.Msg.TrustPolicy)
				}
			})
		}
	}
}

func TestProjectListPaginationAndRemove(t *testing.T) {
	var pages []*corev1.ListTrustPoliciesRequest
	calls := projectEndpoint(t, &projectServer{
		list: func(req *corev1.ListTrustPoliciesRequest) (*corev1.ListTrustPoliciesResponse, error) {
			pages = append(pages, req)
			if req.PageToken == nil {
				return &corev1.ListTrustPoliciesResponse{TrustPolicies: []*corev1.TrustPolicy{gitlabPolicy(true)}, NextPageToken: proto.String("page-2")}, nil
			}
			return &corev1.ListTrustPoliciesResponse{TrustPolicies: []*corev1.TrustPolicy{gitlabPolicy(false)}}, nil
		},
		remove: func(req *corev1.RemoveTrustPolicyRequest) (*corev1.RemoveTrustPolicyResponse, error) {
			if req.ProjectId != "depot-project" || req.TrustPolicyId != "policy-1" {
				t.Errorf("incorrect remove scope: %v", req)
			}
			return &corev1.RemoveTrustPolicyResponse{}, nil
		},
	})
	client := api.NewProjectClient()
	first, err := client.ListTrustPolicies(context.Background(), api.WithAuthentication(connect.NewRequest(&corev1.ListTrustPoliciesRequest{ProjectId: "depot-project", PageSize: proto.Int32(1)}), "organization-token"))
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Msg.TrustPolicies) != 1 || !first.Msg.TrustPolicies[0].GetGitlab().GetProtectedRefsOnly() || first.Msg.GetNextPageToken() != "page-2" {
		t.Fatalf("first page = %v", first.Msg)
	}
	second, err := client.ListTrustPolicies(context.Background(), api.WithAuthentication(connect.NewRequest(&corev1.ListTrustPoliciesRequest{ProjectId: "depot-project", PageSize: proto.Int32(1), PageToken: first.Msg.NextPageToken}), "organization-token"))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Msg.TrustPolicies) != 1 || second.Msg.TrustPolicies[0].GetGitlab() == nil || second.Msg.TrustPolicies[0].GetGitlab().ProtectedRefsOnly || second.Msg.NextPageToken != nil {
		t.Fatalf("second page = %v", second.Msg)
	}
	want := []*corev1.ListTrustPoliciesRequest{{ProjectId: "depot-project", PageSize: proto.Int32(1)}, {ProjectId: "depot-project", PageSize: proto.Int32(1), PageToken: proto.String("page-2")}}
	for i, p := range pages {
		if !proto.Equal(p, want[i]) {
			t.Errorf("page request = %v", p)
		}
	}
	_, err = client.RemoveTrustPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(&corev1.RemoveTrustPolicyRequest{ProjectId: "depot-project", TrustPolicyId: "policy-1"}), "organization-token"))
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestProjectUpdateFailureDoesNotDeleteOrRecreate(t *testing.T) {
	for _, code := range []connect.Code{connect.CodeUnimplemented, connect.CodePermissionDenied, connect.CodeInvalidArgument} {
		t.Run(code.String(), func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				_, handler := corev1connect.NewProjectServiceHandler(&projectServer{update: func(*corev1.UpdateTrustPolicyRequest) (*corev1.UpdateTrustPolicyResponse, error) {
					return nil, connect.NewError(code, errors.New("update rejected"))
				}})
				handler.ServeHTTP(w, r)
			}))
			defer server.Close()
			t.Setenv("DEPOT_API_URL", server.URL)
			_, err := api.NewProjectClient().UpdateTrustPolicy(context.Background(), api.WithAuthentication(connect.NewRequest(&corev1.UpdateTrustPolicyRequest{ProjectId: "depot-project", TrustPolicyId: "policy-1", ProtectedRefsOnly: proto.Bool(false)}), "organization-token"))
			if connect.CodeOf(err) != code {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(paths, []string{corev1connect.ProjectServiceUpdateTrustPolicyProcedure}) {
				t.Fatalf("unexpected operations: %v", paths)
			}
		})
	}
}
