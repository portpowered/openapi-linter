# OpenAPI Linter

[![Go version](https://img.shields.io/github/go-mod/go-version/portpowered/openapi-linter)](https://github.com/portpowered/openapi-linter/blob/main/go.mod)
[![CI](https://github.com/portpowered/openapi-linter/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/portpowered/openapi-linter/actions/workflows/ci.yml)
[![Coverage](https://portpowered.github.io/openapi-linter/coverage.svg)](https://portpowered.github.io/openapi-linter/coverage.html)
[![Release](https://img.shields.io/github/v/release/portpowered/openapi-linter?display_name=tag)](https://github.com/portpowered/openapi-linter/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/portpowered/openapi-linter.svg)](https://pkg.go.dev/github.com/portpowered/openapi-linter)
[![License](https://img.shields.io/github/license/portpowered/openapi-linter)](https://github.com/portpowered/openapi-linter/blob/main/LICENSE)
[![Documentation](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://portpowered.github.io/openapi-linter/)

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
