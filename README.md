# Depot Go SDK [![CI](https://github.com/depot/depot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/depot/depot-go/actions/workflows/ci.yml)

See [examples](./examples) for usage.

## Project OIDC trust policies

`api.NewProjectClient()` exposes `ListTrustPolicies`, `AddTrustPolicy`,
`UpdateTrustPolicy`, and `RemoveTrustPolicy` on the public Project API. Pass an
**organization API token** with `api.WithAuthentication` on each request.
Project tokens and read-only organization tokens cannot authorize these operations.

```go
client := api.NewProjectClient()
resp, err := client.AddTrustPolicy(ctx, api.WithAuthentication(connect.NewRequest(&corev1.AddTrustPolicyRequest{
    ProjectId: "your-depot-project-id",
    Provider: &corev1.AddTrustPolicyRequest_Gitlab{Gitlab: &corev1.TrustPolicy_GitLab{
        NamespaceId:       "12345",
        ProjectId:         "67890",
        ProtectedRefsOnly: true,
    }},
}), token))
if err != nil {
    return err
}
if resp.Msg.TrustPolicy == nil {
    return fmt.Errorf("no trust policy returned")
}
// Inspect the saved settings: adding an existing policy preserves its restrictions.
policy := resp.Msg.TrustPolicy
_, err = client.UpdateTrustPolicy(ctx, api.WithAuthentication(connect.NewRequest(&corev1.UpdateTrustPolicyRequest{
    ProjectId:         "your-depot-project-id",
    TrustPolicyId:     policy.TrustPolicyId,
    ProtectedRefsOnly: proto.Bool(false),
}), token))
if err != nil {
    return err
}
```

Import `corev1` from `github.com/depot/depot-go/proto/depot/core/v1`, `api` from
`github.com/depot/depot-go/api`, `connect` from `connectrpc.com/connect`, and
`proto` from `google.golang.org/protobuf/proto`.

GitLab namespace and project IDs are numeric strings; the GitLab project ID is
separate from the Depot project ID. `ProtectedRefsOnly: true` limits GitLab
authentication to protected refs. New policies default to `false`. In an update,
`nil` preserves the saved restriction; `proto.Bool(false)` explicitly disables it.
GitHub's `AllowPublicPullRequests` update field has the same pointer semantics.
Use each restriction only with its corresponding provider. CircleCI and Buildkite
policies are also supported.

`ListTrustPolicies` returns one page. When `NextPageToken` is present, pass it as
`PageToken` with the same project and filter to continue. `RemoveTrustPolicy`
revokes the specified policy using its `ProjectId` and `TrustPolicyId`.

The [GitLab example](./examples/gitlab-trust-policy/main.go) performs one explicit
operation per invocation and prints the saved policy, including false restrictions.
From this checkout, set `DEPOT_TOKEN` and run:

```bash
go run ./examples/gitlab-trust-policy list <depot-project-id>
go run ./examples/gitlab-trust-policy add <depot-project-id> <namespace-id> <gitlab-project-id> true
go run ./examples/gitlab-trust-policy update <depot-project-id> <policy-id> false
go run ./examples/gitlab-trust-policy remove <depot-project-id> <policy-id>
```

Pass the returned `nextPageToken` as the last argument to `list` for the next page.
The client defaults to `https://api.depot.dev`; `DEPOT_API_URL` overrides it.
Only point authenticated clients at endpoints you trust.

Regenerate the Project bindings from the pinned public schema with:

```bash
buf generate --template buf.gen.project.yaml
```

## Registry retention policies

`api.NewRegistryClient()` exposes the public `depot.registry.v1beta1.RegistryService`,
including `GetRetentionPolicy`, `UpsertRetentionPolicy`, and `DeleteRetentionPolicy`.
Pass an **organization API token** with `api.WithAuthentication` on each request.
The token determines the organization; don't supply an organization selector.
Project tokens and Registry pull/push tokens don't authorize these operations.

```go
client := api.NewRegistryClient()
req := connect.NewRequest(&registryv1beta1.UpsertRetentionPolicyRequest{
    Repository: "team/app",
    Enabled:    true,
    KeepCount:  proto.Int32(10),
})
_, err := client.UpsertRetentionPolicy(ctx, api.WithAuthentication(req, token))
if err != nil {
    return err
}
```

Import `registryv1beta1` from
`github.com/depot/depot-go/proto/depot/registry/v1beta1`, `connect` from
`connectrpc.com/connect`, and `proto` from `google.golang.org/protobuf/proto`.
The [complete example](./examples/registry-retention/main.go) includes get,
upsert, and delete operations. From this checkout, set `DEPOT_TOKEN` and run one
operation at a time:

```bash
go run ./examples/registry-retention get team/app
go run ./examples/registry-retention keep-count team/app 10
go run ./examples/registry-retention keep-days '**' 30
go run ./examples/registry-retention delete team/app
```

Repository names are paths such as `team/app`, without the registry host or
organization prefix. `**` addresses the organization-default policy. Set exactly
one positive integer strategy, `KeepCount` or `KeepDays`; their pointers distinguish
an absent strategy from zero, which the API rejects. `Enabled` isn't a disable
switch: use `DeleteRetentionPolicy` to remove an explicit policy.

`GetRetentionPolicy` returns a nil `Policy` if this scope has no explicit policy;
it doesn't resolve inheritance. Deleting a repository policy removes only that
explicit policy, so an inherited policy may still apply. Deleting `**` removes
the explicit organization default. Neither operation deletes the repository or
its images, and neither restores images already removed by retention.

The client defaults to `https://api.depot.dev`; `DEPOT_API_URL` overrides the
endpoint. Only point authenticated clients at endpoints you trust.

To regenerate the Registry bindings from the pinned public schema:

```bash
buf generate --template buf.gen.registry.yaml
```

## Contributing

PR contributions are welcome! The CLI codebase is evolving rapidly, but we are happy to work with you on your contribution.

## License

MIT License, see `LICENSE`

## Finalized invoices

`api.NewBillingClient()` exposes `ListInvoices`, `GetInvoice`, and
`ListInvoiceLineItems`. Authenticate each request with `api.WithAuthentication`
and an organization token. An organization owner's user token is also supported;
set `X-Depot-Org` on each request to select the organization.

The API includes finalized invoices for the linked Stripe customer across current,
previous, and canceled subscriptions, plus standalone invoices (whose
`SubscriptionId` is empty). Other Stripe customers, drafts, and upcoming estimates
are excluded. Organizations sharing a Stripe customer share its invoice history.

```go
client := api.NewBillingClient()
page, err := client.ListInvoices(ctx, api.WithAuthentication(connect.NewRequest(&corev1.ListInvoicesRequest{
    PageSize: 100,
}), token))
if err != nil {
    return err
}
for _, invoice := range page.Msg.Invoices {
    // Amounts are signed int64 minor units; do not convert through float64.
    fmt.Println(invoice.Id, invoice.Currency, invoice.Total)
}
// Pass page.Msg.NextPageToken in the next request until it is empty.
// Even an empty page can have a continuation token when drafts were filtered.
```

`GetInvoice` takes an `InvoiceId`; `ListInvoiceLineItems` takes `InvoiceId`,
`PageSize`, and `PageToken`, and returns its own `NextPageToken`. Page size is at
most 100. Tokens are opaque and bound to the organization/customer and, for lines,
the invoice. Hosted/PDF invoice links are confidential; avoid logging them.

Billing bindings are generated from published Buf commit
`2ae2cc96583940688aa4b20766844296` with `buf.gen.billing.yaml`.
