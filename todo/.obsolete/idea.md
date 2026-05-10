# migrable

Declarative config file migrations for TOML.

A standalone Go binary that applies migration files to TOML config files. Comment-preserving, atomic, reversible. Powered by [go-toml-edit](https://github.com/smm-h/go-toml-edit).

---

## Table of Contents

- [Motivation](#motivation)
- [Project Identity](#project-identity)
- [Architecture](#architecture)
- [Migration File Format](#migration-file-format)
- [Migration File Structure](#migration-file-structure)
- [Op Vocabulary](#op-vocabulary)
- [Op-to-API Mapping](#op-to-api-mapping)
- [Match Modes](#match-modes)
- [Type System](#type-system)
- [Dot-Path Addressing](#dot-path-addressing)
- [Down Ops (Reversibility)](#down-ops-reversibility)
- [Config-Schema.json](#config-schemajson)
- [CLI](#cli)
- [go-toml-edit](#go-toml-edit)
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
3. **TOML is the right target format.** It is widely adopted for configuration files, has a strong type system (datetimes, integers, floats are distinct), and supports comments natively. A tool that migrates TOML configs covers the most common case.
4. **A standalone tool avoids coupling.** Migration infrastructure should not live inside any one project's release tooling.

### Consumer projects that motivated the design

| Project | Language | Config location | Current state |
|---------|----------|----------------|---------------|
| howmuchleft | JavaScript/npm | `~/.config/howmuchleft.toml` (per-user TOML) | 12 fields added, 2 removed across 8 versions with zero migration mechanism. Users silently lose customizations. |
| claudewheel | Python+npm | `~/.claudelauncher/` (per-user multi-file config) | Already has inline imperative migrations (`_run_versioned_migrations`). Would benefit from externalizing to declarative format. |
| safegit | Go | `.git/safegit/config.toml` (per-repo) | Config has never changed but has `schemaVersion` plumbing ready. |

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

migrable reads JSON migration files that describe **intent** (add a field, rename a field, set a value conditionally) and applies them to TOML config files using [go-toml-edit](https://github.com/smm-h/go-toml-edit), a comment-preserving TOML editing library.

The architecture is straightforward:

- **Migration files** (JSON) declare ops -- structured operations describing what to change.
- **Config files** (TOML) are the targets being migrated.
- **go-toml-edit** handles all TOML parsing, editing, and diffing, preserving comments and formatting.
- **The op engine** interprets each op and translates it into go-toml-edit API calls.

Migration files are JSON (not TOML) because they describe structured operations with nested objects (down ops, match clauses) where JSON is more natural. The tool migrates TOML files, but its own migration definitions use JSON. `config-schema.json` is also JSON. This avoids bootstrapping issues.

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
2. For each section (`structure`, `data`), concatenate ops arrays in file order.
3. Concatenate `description` fields with `"; "` separator.
4. Write the result as `migrations/<version>.json`.
5. Delete all files from `next/`.
6. If `next/` is empty, no migration file is created.

### Version tracking

The current schema version is stored as a `_schema_version` key in the TOML config file itself, set to the semver string of the highest applied migration. Defaults to `"0.0.0"` if absent.

### Accumulation

Migration files are never removed. Version N ships all prior migrations. A user upgrading from v1.0 to v3.0 gets all intermediate migrations applied in order (semver comparison).

### Pre-release versions

No migration files for betas or release candidates. Pre-release migrations stay in `next/` until the actual release.

### Fresh installs

Every op must tolerate a config that never had the old shape:

- `set_value` on a non-existent key creates it (and intermediate tables).
- `rename_field` on a non-existent key is a no-op (per its down declaration behavior).

---

## Migration File Structure

Migration files have explicit sections with a fixed execution order: **structure -> data**. Both sections are optional.

```json
{
  "description": "Add user verification fields",
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
      "type": "datetime",
      "default": null,
      "down": {"op": "remove_field", "path": "users.verified_at"}
    }
  ],
  "data": [
    {
      "op": "set_value_where",
      "path": "users",
      "match_mode": "subset",
      "where": {"role": "admin"},
      "set": {"email_verified": true},
      "down": {
        "op": "set_value_where",
        "path": "users",
        "match_mode": "subset",
        "where": {"role": "admin"},
        "set": {"email_verified": false}
      }
    }
  ]
}
```

---

## Op Vocabulary

12 ops across 3 categories.

### Structure ops

Change what fields and tables exist.

| Op | Fields | Description | Behavior when path missing |
|----|--------|-------------|---------------------------|
| `add_field` | `path`, `type`, `default`, `down` | Add a new field | Creates field (and intermediate tables) |
| `remove_field` | `path`, `down` | Remove a field | No-op |
| `rename_field` | `from`, `to`, `down` | Rename a field | No-op if source missing |
| `add_collection` | `path`, `fields` (optional), `down` | Create a new table/array of tables | Error if exists |
| `drop_collection` | `path`, `down` | Remove a table/array of tables | No-op |

### Data ops

Change values within existing structure.

| Op | Fields | Description | Behavior when path missing |
|----|--------|-------------|---------------------------|
| `set_value` | `path`, `value`, `down` | Unconditional overwrite | Creates (and intermediate tables) |
| `set_value_where` | `path`, `where`, `match_mode`, `set`, `down` | Conditional set on matching items | No-op if no match |
| `remove_where` | `path`, `where`, `match_mode`, `down` | Remove matching items | No-op if no match |
| `append` | `path`, `value`, `down` | Add to array | Error if array missing |
| `transform` | `path`, `expr` (CEL), `down` | Compute new value from old | Error if path missing |
| `merge_defaults` | `path`, `value`, `down` | Deep merge missing keys | Creates missing keys |

**`merge_defaults` specifics**: Tables merge recursively (adding missing keys, never overwriting existing). Arrays and scalars are atomic -- if a value exists, keep it; if missing, set the default. Array manipulation should use `set_value_where`, `append`, or `remove_where`.

### Raw ops

Escape hatch for operations not covered by the op vocabulary.

| Op | Fields | Description |
|----|--------|-------------|
| `raw` | `content`, `down` | Raw TOML text splice into the document |

---

## Op-to-API Mapping

Each op maps to [go-toml-edit](https://github.com/smm-h/go-toml-edit) API calls.

| Op | go-toml-edit API |
|----|-----------------|
| `add_field` | `doc.SetCreate(path, value)` |
| `remove_field` | `doc.Delete(path)` |
| `rename_field` | `doc.Rename(from, newKey)` (extract last segment from `to` path) |
| `add_collection` | `doc.NewTable(path)` + optional `doc.SetCreate` for each field |
| `drop_collection` | `doc.Delete(path)` |
| `set_value` | `doc.Set(path, value)` |
| `set_value_where` | `doc.Items(path)` to iterate + `doc.Set` on matches |
| `remove_where` | `doc.Items(path)` to iterate + `doc.Delete` on matches |
| `append` | `doc.Get(path)` to read array, append element, `doc.Set(path, updated)` to write back |
| `transform` | `doc.Get(path)` to read, CEL evaluate, `doc.Set(path, result)` |
| `merge_defaults` | `doc.MergeDefaults(path, value)` |
| `raw` | Manual parse + splice (escape hatch) |

---

## Match Modes

Used by `set_value_where` and `remove_where` to identify target items.

| Mode | Match field type | Behavior | Default |
|------|-----------------|----------|---------|
| `subset` | table | Item contains all match key-value pairs (can have extra keys) | Yes (default if `match_mode` omitted) |
| `exact` | table | Item has exactly these keys and values, nothing more | |
| `all` | (none) | Matches every item in the array of tables | |
| `index` | integer | Matches by array position | |
| `has_key` | string | Item has this key, regardless of value | |
| `not_has_key` | string | Item lacks this key | |
| `regex` | table | String values matched as regex patterns | |

Each mode is explicitly declared via the `match_mode` field. `subset` is the default when omitted.

---

## Type System

TOML-native types.

| Type | TOML representation | Description |
|------|---------------------|-------------|
| `string` | `"hello"` | Basic string value |
| `integer` | `42` | Whole numbers (64-bit signed) |
| `float` | `3.14` | IEEE 754 double precision |
| `boolean` | `true` / `false` | True or false |
| `datetime` | `1979-05-27T07:32:00Z` | Offset date-time (RFC 3339) |
| `local_datetime` | `1979-05-27T07:32:00` | Local date-time (no timezone) |
| `local_date` | `1979-05-27` | Local date |
| `local_time` | `07:32:00` | Local time |
| `array` | `[1, 2, 3]` | Ordered collection of values |
| `table` | `[section]` / `key = {}` | Key-value mapping |

Every `add_field` op must declare a `type`. This ensures migration files are self-documenting and validates that default values match their declared type.

---

## Dot-Path Addressing

All paths use dot-separated keys to address nested TOML values.

### Syntax

| Notation | Meaning | Example |
|----------|---------|---------|
| `a.b.c` | Nested table access | `[a]` -> `[a.b]` -> key `c` |
| `a[N]` | Array index (zero-based) | Third element of array `a` |
| `a[-1]` | Negative array index | Last element of array `a` |
| `a\.b` | Escaped dot (literal dot in key name) | Key named `a.b` |
| `"quoted key"` | Quoted segment | Key with spaces or special characters |

The path syntax matches go-toml-edit's path semantics. Paths are resolved left to right, creating intermediate tables as needed for write operations.

### Examples

- `server.host` -- key `host` inside table `server`
- `servers[0].ip` -- key `ip` in the first element of array of tables `servers`
- `servers[-1].port` -- key `port` in the last element of array of tables `servers`
- `config.ui\.settings.theme` -- key `theme` inside key `ui.settings` (literal dot) inside table `config`

---

## Down Ops (Reversibility)

Every op **must** have a `"down"` field declaring how to reverse it. There is no auto-inference. The `down` field contains a complete op object.

### Reversal pairs

| Forward op | Down op |
|-----------|---------|
| `add_field` | `remove_field` |
| `remove_field` | `add_field` (with type, default) |
| `rename_field` | `rename_field` (from/to swapped) |
| `add_collection` | `drop_collection` |
| `drop_collection` | `add_collection` (with fields) |
| `set_value` | `set_value` (with old value) |
| `set_value_where` | `set_value_where` (reverse the set values) |
| `remove_where` | `append` (with removed items) |
| `append` | `remove_where` (matching the appended item) |
| `transform` | `transform` (reverse expression) |
| `merge_defaults` | `remove_field` for each added key, or reverse op |
| `raw` | `raw` (reverse content) |

### Rollback execution order

`migrable migrate --rollback` reverses the last applied migration by executing all down ops in **reverse section order**: data -> structure. Within each section, ops execute in reverse order.

---

## Config-Schema.json

Declared per-project. Tells migrable where to find TOML config files.

```json
{
  "files": {
    "config": "$HOME/.config/howmuchleft.toml"
  },
  "migrations_dir": ".rlsbl/migrations"
}
```

### Shell variable expansion

Paths support shell variables: `$HOME`, `$XDG_CONFIG_HOME`, etc. Expanded at runtime.

### Multi-file support

Multiple config files can be declared in the `files` map. Each op targets a specific file via the first segment of its dot-path. If there is only one file, the file key is implicit.

---

## CLI

```
migrable migrate                    Run pending migrations
  --dry-run                        Preview changes (diff output)
  --status                         Show current version + pending migrations
  --rollback                       Reverse the last applied migration (uses down ops)
  --config-dir <path>              Override schema config location (default: .migrable/ or .rlsbl/)

migrable merge <version>           Combine next/*.json into <version>.json, empty next/

migrable validate                  Validate migration files (syntax, type checks, down op presence)

migrable init                      Scaffold config-schema.json and migrations/ directory

migrable --version                 Show version
migrable --help                    Show help
```

### Dry run output

`--dry-run` uses `tomledit.Diff(before, after)` to show changes without writing them. The diff output lists each change with its category, path, and values:

| Change type | Output |
|-------------|--------|
| Added | Path, new value |
| Removed | Path, old value |
| Modified | Path, old value, new value |

---

## go-toml-edit

migrable uses [go-toml-edit](https://github.com/smm-h/go-toml-edit) (`github.com/smm-h/go-toml-edit`) as its core dependency for all TOML manipulation.

### What it provides

- **Comment-preserving editing**: Modifies TOML values without destroying comments, whitespace, or formatting. No parse-modify-serialize round-trip.
- **Dot-path addressing**: Get, set, delete, and rename values using dot-path strings with array index support.
- **Iteration**: `Items()` for iterating over arrays of tables, enabling conditional ops like `set_value_where`.
- **Deep merge**: `MergeDefaults()` for recursive table merging with atomic scalars and arrays.
- **Table creation**: `NewTable()` for creating TOML tables and arrays of tables.
- **Diffing**: `Diff(before, after)` compares two documents and returns structured changes (Added, Removed, Modified) with paths and values.

### Atomicity

TOML files are written atomically: write to a temporary file in the same directory, then rename. This prevents partial writes from corrupting config files. The entire file is written only if at least one op produced a change. All ops for a single file are applied in memory before writing -- all-or-nothing per file.

---

## Engine Behavior

The migration engine follows this sequence for each invocation:

1. **Read `config-schema.json`** to discover TOML config files and the migrations directory.
2. **For each config file**, read the current schema version from the `_schema_version` key (defaults to `"0.0.0"` if absent).
3. **Discover migration files** in the migrations directory. Parse version from filename. Sort by semver.
4. **Filter** to versions greater than the current schema version.
5. **For each migration in order**:
   - Parse sections: `structure`, `data`.
   - Apply each section in fixed order. Within each section, ops execute sequentially.
   - Each op translates to go-toml-edit API calls (see [Op-to-API Mapping](#op-to-api-mapping)).
6. **Update `_schema_version`** to the highest applied migration.
7. **Write atomically** (tmp + rename) only if changed. All-or-nothing per file.
8. **Report results**: applied count, current version, any warnings.

---

## CEL Integration

Transform expressions use CEL (Common Expression Language) via `cel-go` (Google's official Go implementation).

### Why CEL

- **Non-Turing-complete**: Guaranteed termination. No infinite loops.
- **Sandboxed**: No file access, no network access, no side effects.
- **Expressive**: String operations, conditionals, list/map operations (`.map`, `.filter`, `.exists`, `.all`).

### Usage in migrable

CEL is used exclusively for `transform` ops to compute new TOML values from existing ones. The engine reads the current value via `doc.Get(path)`, evaluates the CEL expression with the value bound as `value`, and writes the result back via `doc.Set(path, result)`.

### Example expressions

| CEL expression | Effect |
|---------------|--------|
| `value == 'dark' ? 'night' : 'day'` | Remap string values |
| `value + '_suffix'` | Append to string |
| `value * 100` | Scale numeric value |
| `size(value) > 0 ? value : 'default'` | Conditional default |

---

## Conformance Test Suite

JSON test files with input TOML (as parsed objects), migration ops, and expected output. Published alongside migrable. Usable by any language that wants to implement a compatible migration engine.

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

- Op correctness for all 12 ops
- Edge cases: missing paths, type conflicts, empty configs, malformed input
- Match mode behavior for all 7 modes
- `merge_defaults` recursion (nested tables, atomic arrays, atomic scalars)
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
| `github.com/smm-h/go-toml-edit` | Comment-preserving TOML editing, diffing, and merging |
| `github.com/google/cel-go` | CEL expression evaluation for `transform` ops |
| `github.com/Masterminds/semver/v3` | Semver parsing and comparison for migration ordering |
| `github.com/spf13/cobra` (or similar) | CLI framework |

---

## Effort Estimate

go-toml-edit handles TOML parsing, editing, diffing, and merging. The main work is the op engine, CEL integration, CLI, and tooling.

| Component | Estimated LOC | Notes |
|-----------|--------------|-------|
| Op engine (interpret ops, call go-toml-edit API) | 300-500 | 12 ops, type handling, match modes |
| CEL integration | 100-200 | cel-go glue, value marshaling between TOML and CEL |
| CLI | 200-300 | cobra commands, flags, help text |
| Config-schema parser | 80-120 | JSON parsing, variable expansion |
| next/ merge tool | 100-150 | File combination logic |
| Conformance test suite | 400-600 | Comprehensive JSON test cases |
| Tests (Go) | 800-1200 | Table-driven tests for all ops |
| **Total** | **~2000-3100 LOC source + ~1200-1800 LOC tests** | |

---

## Future Work

- **JSONC backend** (v2): Add support for migrating JSONC config files. Would require a surgical comment-preserving JSONC editor (~200-400 LOC) or an equivalent library.
- **YAML backend**: Support for YAML config files with comment preservation.
- **Schema diffing**: Auto-generate migration files from before/after TOML schema comparison.
- **GUI**: Web UI for browsing migration history and visualizing config changes.
