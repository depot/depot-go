# Depot Go SDK [![CI](https://github.com/depot/depot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/depot/depot-go/actions/workflows/ci.yml)

See [examples](./examples) for usage.

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
