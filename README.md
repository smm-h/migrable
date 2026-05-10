# migrable

Declarative config file migrations for TOML.

## Features

- 14 migration ops covering structure, data, and raw edits
- Comment-preserving TOML editing via [go-toml-edit](https://github.com/smm-h/go-toml-edit)
- Rollback via down ops
- Multi-file transactional writes
- CEL expressions for transforms
- Dry-run preview
- Conformance test suite

## Install

```
go install github.com/smm-h/migrable@latest
```

Or download binaries from [GitHub Releases](https://github.com/smm-h/migrable/releases).

npm and PyPI wrappers coming soon.

## Quick start

1. Scaffold a new project:

```
migrable init
```

This creates `migrable.toml` and a `migrations/` directory.

2. Edit `migrable.toml` to point to your config file:

```toml
migrations_dir = "migrations"

[files]
config = "config.toml"
```

3. Create a migration file `migrations/0001_add-debug.toml`:

```toml
description = "add debug flag to server section"

[[structure]]
op = "add_field"
path = "server.debug"
type = "boolean"
default = false
down = { op = "remove_field", path = "server.debug" }
```

4. Apply pending migrations:

```
migrable migrate
```

5. Preview changes without writing:

```
migrable migrate --dry-run
```

6. Reverse the last migration:

```
migrable migrate --rollback
```

## Library usage

Go projects can import the engine directly:

```go
import (
    "github.com/smm-h/migrable/config"
    "github.com/smm-h/migrable/engine"
)

cfg, _ := config.Load("")
result, _ := engine.Migrate(cfg, false)
```

## CLI reference

| Command | Description |
|---|---|
| `migrable migrate` | Run pending migrations |
| `migrable migrate --dry-run` | Preview changes |
| `migrable migrate --rollback` | Reverse last migration |
| `migrable status` | Show version and pending migrations |
| `migrable validate` | Check migration files |
| `migrable merge <version>` | Combine staging files |
| `migrable init` | Scaffold new project |

## License

MIT
