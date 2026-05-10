# migrable

Declarative, backend-agnostic schema and data migrations.

A standalone Go binary that applies the same migration files to JSONC config files and Postgres databases. One vocabulary of ops, multiple backends.

---

## Table of Contents

- [Motivation](#motivation)
- [Project Identity](#project-identity)
- [Architecture](#architecture)
- [Migration File Format](#migration-file-format)
- [Migration File Structure](#migration-file-structure)
- [Op Vocabulary](#op-vocabulary)
- [Match Modes](#match-modes)
- [Type System](#type-system)
- [Dot-Path Addressing](#dot-path-addressing)
- [Down Ops (Reversibility)](#down-ops-reversibility)
- [Config-Schema.json](#config-schemajson)
- [CLI](#cli)
- [JSONC Surgical Editor](#jsonc-surgical-editor)
- [Postgres Backend](#postgres-backend)
- [Engine Behavior](#engine-behavior)
- [CEL Integration](#cel-integration)
- [Conformance Test Suite](#conformance-test-suite)
- [rlsbl Integration](#rlsbl-integration)
- [Dependencies](#dependencies)
- [Effort Estimate](#effort-estimate)
- [Future Work](#future-work)

---

## Motivation

The project was born from designing a config migration engine for [rlsbl](https://github.com/smm-h/rlsbl) (a Python release orchestration CLI). Key realizations:

1. **Config migration is a general problem.** Every tool that stores user configuration eventually needs to evolve that schema. This is not specific to rlsbl.
2. **Downstream projects span multiple languages.** JavaScript, Python, Go -- a single Go binary serves all of them without language-specific dependencies.
3. **The same op vocabulary maps to both config files and databases.** "Add a field" means the same thing whether the target is a JSONC file or a Postgres table. The intent is identical; only the translation differs.
4. **A standalone tool avoids coupling.** Migration infrastructure should not live inside any one project's release tooling.

### Consumer projects that motivated the design

| Project | Language | Config location | Current state |
|---------|----------|----------------|---------------|
| howmuchleft | JavaScript/npm | `~/.config/howmuchleft.json` (per-user JSONC) | 12 fields added, 2 removed across 8 versions with zero migration mechanism. Users silently lose customizations. |
| claudewheel | Python+npm | `~/.claudelauncher/` (per-user multi-file JSON) | Already has inline imperative migrations (`_run_versioned_migrations`). Would benefit from externalizing to declarative format. |
| safegit | Go | `.git/safegit/config.json` (per-repo) | Config has never changed but has `schemaVersion` plumbing ready. |

---

## Project Identity

| Property | Value |
|----------|-------|
| Name | migrable |
| Language | Go |
| Type | Standalone CLI binary (not a Go library) |
| Distribution | goreleaser cross-platform binaries (linux/darwin/windows, amd64/arm64), GitHub Releases |
| Registry wrappers | Thin npm and PyPI wrapper packages (eventual) |
| Name availability | Confirmed available on both npm and PyPI registries. Zero GitHub repos with this name. |

---

## Architecture

### Backend system

The same migration files work across all backends. A migration file describes **intent** (add a field, rename a field, set a value conditionally). The tool translates each op to the target backend's native operations.

### v1 backends

- **JSONC**: Surgical comment-preserving editor (~200-400 LOC Go). Reads JSONC, modifies values at specific byte positions without re-serialization, preserves all comments, whitespace, and formatting.
- **Postgres**: Generates and executes SQL. Uses transactional DDL. Connects via DSN from environment.

### Future backends

- SQLite (v2)
- TOML
- MongoDB

---

## Migration File Format

### Location

Migration files live in a configurable migrations directory. The path is declared in `config-schema.json`. Defaults: `.migrable/migrations/` or `.rlsbl/migrations/`.

### Naming convention

Files are named by semver: `1.2.0.json`, `2.0.0.json`. Not numbered sequences. The version in the filename is the version of the project that ships this migration.

### Staging with next/

During development, migration files live in `migrations/next/` with descriptive names:

- `add-email-verified.json`
- `rename-color-mode.json`

At release time, `migrable merge <version>` combines all `next/*.json` into a single `<version>.json` and empties `next/`.

### Merge algorithm

1. Read all `*.json` from `next/`, sorted alphabetically.
2. For each section (`structure`, `data`, `indexes`, `constraints`), concatenate ops arrays in file order.
3. Concatenate `description` fields with `"; "` separator.
4. Write the result as `migrations/<version>.json`.
5. Delete all files from `next/`.
6. If `next/` is empty, no migration file is created.

### Version tracking

| Backend | Mechanism | Default |
|---------|-----------|---------|
| JSONC | `_schema_version` key in the config file, set to semver string of highest applied migration | `"0.0.0"` if absent |
| Postgres | `_migrations` table: `(version VARCHAR PRIMARY KEY, applied_at TIMESTAMPTZ DEFAULT now())` | Auto-created if missing |

### Accumulation

Migration files are never removed. Version N ships all prior migrations. A user upgrading from v1.0 to v3.0 gets all intermediate migrations applied in order (semver comparison).

### Pre-release versions

No migration files for betas or release candidates. Pre-release migrations stay in `next/` until the actual release.

### Fresh installs

Every op must tolerate a config that never had the old shape:

- `set_default` on a non-existent key creates it.
- `rename_field` on a non-existent key is a no-op (per its down declaration behavior).

---

## Migration File Structure

Migration files have explicit sections with a fixed execution order: **structure -> data -> indexes -> constraints**. All sections are optional.

```json
{
  "description": "Add user verification",
  "structure": [
    {
      "op": "add_field",
      "path": "users.email_verified",
      "type": "boolean",
      "default": false,
      "down": {"op": "remove_field", "path": "users.email_verified"}
    },
    {
      "op": "add_field",
      "path": "users.verified_at",
      "type": "timestamp",
      "default": null,
      "down": {"op": "remove_field", "path": "users.verified_at"}
    }
  ],
  "data": [
    {
      "op": "set_value_where",
      "path": "users.email_verified",
      "match_mode": "subset",
      "where": {"role": "admin"},
      "value": true,
      "down": {
        "op": "set_value_where",
        "path": "users.email_verified",
        "match_mode": "subset",
        "where": {"role": "admin"},
        "value": false
      }
    }
  ],
  "indexes": [
    {
      "op": "add_index",
      "path": "users.email",
      "unique": true,
      "down": {"op": "drop_index", "name": "idx_users_email"}
    }
  ],
  "constraints": [
    {
      "op": "add_constraint",
      "path": "users.email",
      "type": "not_null",
      "down": {"op": "drop_constraint", "path": "users.email", "name": "users_email_not_null"}
    }
  ]
}
```

A JSONC-only migration might only have `structure` and `data`. An index-only migration might only have `indexes`.

---

## Op Vocabulary

18 ops across 5 categories.

### Structure ops

Change what fields and collections exist.

| Op | Fields | Description | JSONC translation | Postgres translation | Behavior when path missing |
|----|--------|-------------|-------------------|---------------------|---------------------------|
| `add_field` | `path`, `type`, `default`, `down` | Add a new field | Set key with value | `ALTER TABLE ADD COLUMN` | Creates field (and intermediate objects) |
| `remove_field` | `path`, `down` | Remove a field | Delete key | `ALTER TABLE DROP COLUMN` | No-op |
| `rename_field` | `from`, `to`, `down` | Rename a field | Move key | `ALTER TABLE RENAME COLUMN` | No-op if source missing |
| `add_collection` | `path`, `fields` (optional), `down` | Create a new collection/table/object | Create nested object | `CREATE TABLE` | Error if exists |
| `drop_collection` | `path`, `down` | Remove a collection/table/object | Delete key | `DROP TABLE` | No-op |

### Data ops

Change values within existing structure.

| Op | Fields | Description | JSONC translation | Postgres translation | Behavior when path missing |
|----|--------|-------------|-------------------|---------------------|---------------------------|
| `set_value` | `path`, `value`, `down` | Unconditional overwrite | Set at path | `UPDATE SET` | Creates (and intermediate objects) for JSONC; error for Postgres if column missing |
| `set_value_where` | `path`, `where`, `match_mode`, `set`, `down` | Conditional set on matching items | Find in list + set fields | `UPDATE SET WHERE` | No-op if no match |
| `remove_where` | `path`, `where`, `match_mode`, `down` | Remove matching items | Remove from list | `DELETE WHERE` | No-op if no match |
| `append` | `path`, `value`, `down` | Add to collection | Push to array | `INSERT INTO` | Error if collection missing |
| `transform` | `path`, `expr` (CEL), `down` | Compute new value from old | Evaluate CEL + set result | CEL-to-SQL `UPDATE` | Error if path missing |
| `merge_defaults` | `path`, `value`, `down` | Deep merge missing keys | Recursive dict merge (atomic lists) | Multiple `ADD COLUMN` or `UPSERT` | Creates missing keys |

**`merge_defaults` specifics**: Dicts merge recursively (adding missing keys, never overwriting existing). Lists and scalars are atomic -- if a value exists, keep it; if missing, set the default. List manipulation should use `set_value_where`, `append`, or `remove_where`.

### Optimization ops

Performance changes. No data or structure change.

| Op | Fields | Description | JSONC translation | Postgres translation |
|----|--------|-------------|-------------------|---------------------|
| `add_index` | `path` (or `table`+`columns`), `unique`, `concurrently` (optional), `where` (optional), `down` | Create index | Skip (no-op) | `CREATE INDEX` |
| `drop_index` | `name`, `down` | Remove index | Skip (no-op) | `DROP INDEX` |

### Constraint ops

Enforce rules.

| Op | Fields | Description | JSONC translation | Postgres translation |
|----|--------|-------------|-------------------|---------------------|
| `add_constraint` | `path`, `name`, `type` (`unique`/`check`/`fk`/`pk`/`not_null`), `details`, `down` | Add constraint | Skip (no-op) | `ALTER TABLE ADD CONSTRAINT` |
| `drop_constraint` | `path`, `name`, `down` | Remove constraint | Skip (no-op) | `ALTER TABLE DROP CONSTRAINT` |
| `add_enum` | `name`, `values[]`, `down` | Define value set | Skip (no-op) | `CREATE TYPE AS ENUM` |
| `alter_enum` | `name`, `add_value`, `before`/`after` (optional), `down` | Extend value set | Skip (no-op) | `ALTER TYPE ADD VALUE` |

### Raw ops

Escape hatch for backend-specific operations.

| Op | Fields | Description | JSONC translation | Postgres translation |
|----|--------|-------------|-------------------|---------------------|
| `raw` | `content`, `down` | Backend-specific raw operation | Raw text splice | Raw SQL execution |

---

## Match Modes

Used by `set_value_where` and `remove_where` to identify target items.

| Mode | Match field type | Behavior | Default |
|------|-----------------|----------|---------|
| `subset` | dict | Item contains all match key-value pairs (can have extra keys) | Yes (default if `match_mode` omitted) |
| `exact` | dict | Item has exactly these keys and values, nothing more | |
| `all` | (none) | Matches every item in the list/table | |
| `index` | integer | Matches by array position | |
| `has_key` | string | Item has this key, regardless of value | |
| `not_has_key` | string | Item lacks this key | |
| `regex` | dict | String values matched as regex patterns | |

Each mode is explicitly declared via the `match_mode` field. `subset` is the default when omitted.

---

## Type System

Universal types that map to every backend.

| Type | JSONC value | Postgres type | Description |
|------|-------------|---------------|-------------|
| `string` | `"string"` | `VARCHAR(n)` or `TEXT` | Short text. Postgres uses `VARCHAR` if max length specified. |
| `text` | `"string"` | `TEXT` | Long text |
| `integer` | number | `INTEGER` or `BIGINT` | Whole numbers |
| `float` | number | `DOUBLE PRECISION` | Decimal numbers |
| `boolean` | `true`/`false` | `BOOLEAN` | True/false |
| `timestamp` | `"ISO 8601 string"` | `TIMESTAMPTZ` | Date+time with timezone |
| `uuid` | `"uuid string"` | `UUID` | Universally unique identifier |
| `array` | `[]` | Array type or junction table | Ordered collection |
| `object` | `{}` | `JSONB` or separate table | Key-value structure |
| `enum` | `"string"` (validated) | `CREATE TYPE ENUM` | Restricted value set |
| `json` | any | `JSONB` | Arbitrary JSON data |

Every `add_field` op must declare a `type`, even for JSONC backends. This ensures migration files are backend-agnostic and self-documenting.

---

## Dot-Path Addressing

All backends use dot-path addressing for field references.

| Backend | Path `users.email_verified` | Path `ui.color.theme` |
|---------|----------------------------|----------------------|
| JSONC | `obj["users"]["email_verified"]` | `obj["ui"]["color"]["theme"]` |
| Postgres | Table `users`, column `email_verified` | Not applicable (Postgres is two levels deep) |
| MongoDB (future) | Collection `users`, field `email_verified` (native dot notation) | Collection `ui`, nested field `color.theme` |

### Escaping

Use backslash for literal dots in key names:

- `config.ui\.settings.theme` addresses key `"ui.settings"` inside `"config"`.

---

## Down Ops (Reversibility)

Every op **must** have a `"down"` field declaring how to reverse it. There is no auto-inference. The `down` field contains a complete op object.

### Reversal pairs

| Forward op | Down op |
|-----------|---------|
| `add_field` | `remove_field` |
| `remove_field` | `add_field` (with type, default) |
| `rename_field` | `rename_field` (from/to swapped) |
| `set_value` | `set_value` (with old value) |
| `transform` | `transform` (reverse expression) |
| `raw` | `raw` (reverse content) |
| `add_index` | `drop_index` |
| `add_constraint` | `drop_constraint` |
| `add_enum` | `raw` (drop type) or reverse op |

### Rollback execution order

`migrable migrate --rollback` reverses the last applied migration by executing all down ops in **reverse section order**: constraints -> indexes -> data -> structure. Within each section, ops execute in reverse order.

---

## Config-Schema.json

Declared per-project. Tells migrable where to find config files and how to connect to databases. The format is always a map.

```json
{
  "files": {
    "config": "$HOME/.config/howmuchleft.json"
  },
  "connections": {
    "main_db": {
      "dsn_env": "DATABASE_URL",
      "env_file": ".env"
    }
  },
  "migrations_dir": ".rlsbl/migrations"
}
```

### Shell variable expansion

Paths support shell variables: `$HOME`, `$XDG_CONFIG_HOME`, etc. Expanded at runtime.

### Connection config

| Field | Purpose |
|-------|---------|
| `dsn_env` | Name of the environment variable containing the connection string |
| `env_file` | Path to `.env` file to load (gitignored). Loaded before checking env vars. |

**Precedence**: CLI `--dsn` flag > environment variable > `.env` file.

No secrets in committed files. `config-schema.json` is committed; `.env` is not.

### Multi-file support

Multiple config files can be declared in the `files` map. Each op targets a specific file via the first segment of its dot-path. If there is only one file, the file key is implicit.

---

## CLI

```
migrable migrate                    Run pending migrations
  --dry-run                        Preview: show ops for JSONC, show SQL for Postgres
  --status                         Show current version + pending migrations
  --rollback                       Reverse the last applied migration (uses down ops)
  --config-dir <path>              Override schema config location (default: .migrable/ or .rlsbl/)
  --dsn <connection-string>        Override database connection string
  --backend <name>                 Run only for this backend (jsonc, postgres)

migrable merge <version>           Combine next/*.json into <version>.json, empty next/

migrable validate                  Validate migration files (syntax, type checks, down op presence)

migrable init                      Scaffold config-schema.json and migrations/ directory

migrable --version                 Show version
migrable --help                    Show help
```

### Dry run output

| Backend | Output |
|---------|--------|
| JSONC | Shows each op and its effect: path, old value, new value |
| Postgres | Outputs the SQL that would be executed (can be piped to `psql`) |

---

## JSONC Surgical Editor

Built-in Go component, approximately 200-400 lines. The approach avoids parse-modify-serialize (which would destroy comments) by working directly on raw bytes.

### Algorithm

1. **Parse JSONC to understand structure.** Map each dot-path to its byte range in the raw text.
2. **For each op, find the target byte range** in the parsed structure.
3. **Splice in the new value** as formatted JSON text at the target position.
4. **Accumulate byte offset deltas** from prior splices so subsequent ops target correct positions.
5. **Everything else stays untouched.** Comments, whitespace, trailing commas, and formatting are preserved exactly.

### Atomicity

JSONC files are written atomically: write to a temporary file in the same directory, then rename. This prevents partial writes from corrupting config files. The entire file is written only if at least one op produced a change. All ops for a single file are applied in memory before writing -- all-or-nothing per file.

---

## Postgres Backend

### SQL generation

Each op translates to one or more SQL statements. The Postgres backend generates SQL from the op vocabulary using the type system for column types.

### Transactional safety

- Each migration file is wrapped in a single transaction (Postgres supports transactional DDL).
- Uses advisory locks (`pg_advisory_lock`) to prevent concurrent migrations from different processes.
- If any op fails, the entire migration is rolled back.

### Migrations table

The `_migrations` table is auto-created if missing:

```sql
CREATE TABLE IF NOT EXISTS _migrations (
    version VARCHAR PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT now()
);
```

### Dry run

Outputs the generated SQL without executing it. The output can be piped directly to `psql` for manual review or execution.

---

## CEL Integration

Transform expressions use CEL (Common Expression Language) via `cel-go` (Google's official Go implementation).

### Why CEL

- **Non-Turing-complete**: Guaranteed termination. No infinite loops.
- **Sandboxed**: No file access, no network access, no side effects.
- **Expressive**: String operations, conditionals, list/map operations (`.map`, `.filter`, `.exists`, `.all`).

### CEL-to-SQL translation

For the Postgres backend, CEL expressions are translated to SQL equivalents where possible. Unsupported CEL constructs trigger a clear error at validation time, not at migration time.

| CEL expression | SQL equivalent |
|---------------|----------------|
| `value == 'dark' ? 'night' : 'day'` | `CASE WHEN value = 'dark' THEN 'night' ELSE 'day' END` |
| `email.split('@')[0]` | `split_part(email, '@', 1)` |
| `name.upperAscii()` | `upper(name)` |
| `size(name)` | `length(name)` |
| `items.filter(x, x > 5)` | Not supported (error) |

---

## Engine Behavior

The migration engine follows this sequence for each invocation:

1. **Read `config-schema.json`** to discover backends (files and connections).
2. **For each backend**, read the current schema version:
   - JSONC: `_schema_version` key in the config file.
   - Postgres: `_migrations` table.
3. **Discover migration files** in the migrations directory. Parse version from filename. Sort by semver.
4. **Filter** to versions greater than the current schema version.
5. **For each migration in order**:
   - Parse sections: `structure`, `data`, `indexes`, `constraints`.
   - Apply each section in fixed order. Within each section, ops execute sequentially.
6. **Update schema version** to the highest applied migration.
7. **Write results**:
   - JSONC: Write atomically (tmp + rename) only if changed. All-or-nothing per file.
   - Postgres: Commit transaction. If any op fails, rollback the entire migration.
8. **Report results**: applied count, current version, any warnings.

---

## Conformance Test Suite

JSON test files with input config, migration ops, and expected output. Published alongside migrable. Usable by any language that wants to implement a compatible migration engine.

### Test file format

```json
{
  "description": "add_field creates nested path",
  "input": {"ui": {}},
  "migration": {
    "structure": [
      {
        "op": "add_field",
        "path": "ui.color",
        "type": "string",
        "default": "blue",
        "down": {"op": "remove_field", "path": "ui.color"}
      }
    ]
  },
  "expected": {"ui": {"color": "blue"}}
}
```

### Coverage

- Op correctness for all 18 ops
- Edge cases: missing paths, type conflicts, empty configs, malformed input
- Match mode behavior for all 7 modes
- `merge_defaults` recursion (nested dicts, atomic lists, atomic scalars)
- Transform expressions (CEL evaluation, various expression types)
- Fresh install scenarios (ops applied to configs that never had the old shape)
- Down op execution (rollback correctness)

---

## rlsbl Integration

migrable is a consumer tool for rlsbl, not a child project. rlsbl delegates to migrable; migrable has no knowledge of rlsbl internals.

### Integration points

| rlsbl command | migrable command | Purpose |
|--------------|-----------------|---------|
| `rlsbl migrate` | `migrable migrate` | Shell out to run pending migrations |
| `rlsbl release` | `migrable merge <version>` | Combine `next/` into versioned migration during release flow |

### Dependency handling

migrable is an optional dependency. If not installed, `rlsbl migrate` shows a clear error: "migrable not found. Install from..."

### Planned rlsbl changes (separate effort)

These changes to rlsbl support the integration but are not part of migrable itself:

- Add `tomlkit` dependency (replace ~110 LOC of TOML regex in `tagging.py` and `pypi.py`)
- Add `filelock >= 3.20.3` dependency (replace Unix-only `fcntl` in `lock.py`, add Windows support)
- Remove `config` subcommand tree (clean break, pre-1.0)
- Add `migrate` command (delegates to migrable)
- Absorb `config show` into `status` command
- Document all currently-hidden flags (`--quiet`, `--no-commit`, `--skip-shared`)
- Add `--json` flag to `status`
- Add Windows support (CI matrix, cross-platform locking)

---

## Dependencies

Go modules required for v1.

| Dependency | Purpose |
|------------|---------|
| `github.com/google/cel-go` | CEL expression evaluation for `transform` ops |
| `github.com/Masterminds/semver/v3` | Semver parsing and comparison for migration ordering |
| `github.com/lib/pq` or `github.com/jackc/pgx/v5` | Postgres driver |
| `github.com/spf13/cobra` (or similar) | CLI framework |
| `github.com/joho/godotenv` | `.env` file loading |

No TOML library needed (JSONC only in v1). No external JSON library needed (the surgical editor is custom-built).

---

## Effort Estimate

| Component | Estimated LOC | Notes |
|-----------|--------------|-------|
| JSONC surgical editor | 200-400 | Parse + splice approach |
| Op engine (apply ops to in-memory data) | 400-600 | 18 ops, type handling, match modes |
| JSONC backend (file I/O + editor integration) | 200-300 | Atomic writes, version tracking |
| Postgres backend (SQL generation + execution) | 400-600 | DDL/DML generation, transactions, advisory locks |
| CEL integration + CEL-to-SQL translator | 200-400 | cel-go glue, SQL generation for subset of CEL |
| CLI | 200-300 | cobra commands, flags, help text |
| Config-schema parser + .env loading | 100-200 | JSON parsing, variable expansion |
| next/ merge tool | 100-150 | File combination logic |
| Conformance test suite | 500-800 | Comprehensive JSON test cases |
| Tests (Go) | 1500-2500 | Table-driven tests for all ops, both backends |
| **Total** | **~4000-6000 LOC source + ~2000-3300 LOC tests** | |

---

## Future Work

- **SQLite backend** (v2): Pure Go driver (`modernc.org/sqlite`). Similar to Postgres but with SQLite-specific DDL limitations (no `ALTER COLUMN`, limited `ALTER TABLE`).
- **TOML backend**: Surgical comment-preserving editor for TOML files (same approach as the JSONC editor).
- **MongoDB backend**: Native document operations.
- **Schema diffing**: Auto-generate migration files from before/after schema comparison (similar to Prisma or Atlas).
- **GUI**: Web UI for browsing migration history and visualizing schema changes.
