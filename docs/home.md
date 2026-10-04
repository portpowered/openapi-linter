# OpenAPI Linter

Check OpenAPI contracts and standalone schemas with configurable rules for structure, naming, examples and API design.

Start with `openapi:recommended`, then compose Google API or Portos policies when they match your service. Each check remains independently selectable. The same public registry powers both the CLI and Go integrations.

| Task | Guide |
| --- | --- |
| Install and lint a specification | [Get started](getting-started.md) |
| Embed checks in a Go application | [Go library](library.md) |
| Find a rule and its parameters | [Rule reference](rules/index.md) |
| Compose packs, scopes and exceptions | [Rule packs](rule-packs.md) |
| Apply Google API design checks | [Google API standards](google-api.md) |
| Apply internal response and error contracts | [Portos contracts](portos-api.md) |

```yaml
version: 1
extends: [google-defaults]
overrides:
  - id: google.version-path
    severity: error
```

Every rule page explains its behavior, parameters, defaults, participating packs and a configuration example checked against the registry. Schema checks describe declared contracts; runtime behavior needs service tests.
