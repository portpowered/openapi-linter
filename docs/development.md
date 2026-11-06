# Development

Run from this standalone repository with Go 1.24.2 or newer:

```sh
GOWORK=off go test -race ./... -timeout 120s
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

Keep application policy in consumer packs/checks. Public packages own parsing, traversal, reference policy, registration, and generic checks. Stock checks must register through the same factory API as third-party checks.

Test located diagnostics, strict configuration, deterministic output, empty inputs, and operational errors. Cross-file behavior needs local-reference fixtures; external HTTP access is disabled. Portability and packaged command examples are required release checks. Before tagging, verify the supported CI matrix, then fetch the immutable version in a separate consumer without workspace replacements.

The module and pack schema use semantic versioning. Compatible additions can introduce new checks/options; removals or incompatible public API/schema changes require an appropriate version transition and consumer migration guidance. Never rewrite published tags.

Reference parsing and successful root/target path validation share the lifetime of one `Program.Run`. Treat its input files as a stable snapshot while checks execute. Root confinement is checked before reusing parsed references, including for a different root; failed validations are never retained. Every new run starts fresh and observes source or symlink changes. No analysis results are persisted.
