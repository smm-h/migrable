# migrable -- Design Specification

Declarative config file migrations for TOML.

A standalone Go binary that applies TOML migration files to TOML config files. Comment-preserving, atomic, reversible. Powered by [go-toml-edit](https://github.com/smm-h/go-toml-edit).

---

## Table of Contents

- [Motivation](#motivation)
- [Project Identity](#project-identity)
- [Architecture](#architecture)
- [migrable.toml](#migrabletoml)
- [Migration File Format](#migration-file-format)
- [Migration File Structure](#migration-file-structure)
- [Op Vocabulary](#op-vocabulary)
- [Op-to-API Mapping](#op-to-api-mapping)
- [Match Modes](#match-modes)
- [Type System](#type-system)
- [Dot-Path Addressing](#dot-path-addressing)
- [Down Ops (Reversibility)](#down-ops-reversibility)
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
| howmuchleft | JavaScript/npm | `~/.config/howmuchleft.toml` (per-user) | 12 fields added, 2 removed across 8 versions with zero migration mechanism. Users silently lose customizations. |
| claudewheel | Python+npm | `~/.claudelauncher/` (per-user multi-file) | Already has inline imperative migrations (`_run_versioned_migrations`). Would benefit from externalizing to declarative format. |
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

Everything is TOML. Migration files, project config, target config files, and conformance tests are all TOML.

- **`migrable.toml`** -- project config declaring target files and migrations directory.
- **Migration files** (`.toml` in `migrations/`) -- declare ops describing what to change.
- **Target files** (`.toml`) -- user config files being migrated.
- **go-toml-edit** -- handles all TOML parsing, editing, and diffing, preserving comments and formatting.
- **Op engine** -- interprets each op and translates it into go-toml-edit API calls.

---

## migrable.toml

Per-project config file. Tells migrable where to find TOML config files and migration files. Lives in the project root or `.migrable/`.

### Single-file project

```toml
migrations_dir = "migrations"

[files]
config = "$HOME/.config/howmuchleft.toml"
```

### Multi-file project

```toml
migrations_dir = "migrations"
version_file = "config"

[files]
config = "$HOME/.config/myapp/config.toml"
themes = "$HOME/.config/myapp/themes.toml"
keybindings = "$HOME/.config/myapp/keybindings.toml"
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `migrations_dir` | Yes | Path to the migrations directory, relative to `migrable.toml` |
| `version_file` | Multi-file only | Which file key holds `_schema_version`. Required when `[files]` has more than one entry. Error if missing. |
| `[files]` | Yes | Map of file keys to filesystem paths. Keys are identifiers used in op paths for multi-file targeting. |

### Shell variable expansion

Paths in `[files]` support shell variables: `$HOME`, `$XDG_CONFIG_HOME`, etc. Expanded at runtime. Unset variables cause a clear error (no empty-string substitution).

### Multi-file path resolution

In multi-file projects, the first segment of an op's dot-path selects the target file by its key in `[files]`. In single-file projects, no file prefix is needed -- ops target the sole file directly.

Example with the multi-file config above:

- `config.ui.theme` -- key `theme` in table `ui` in the `config` file
- `themes.dark.background` -- key `background` in table `dark` in the `themes` file
- `keybindings.editor.save` -- key `save` in table `editor` in the `keybindings` file

### Reserved keys

Keys starting with `_` in target config files are reserved by migrable. Currently only `_schema_version` is used.

---

## Migration File Format

### Location

Migration files live in the directory specified by `migrations_dir` in `migrable.toml`.

### Naming convention

Files are named by semver: `1.2.0.toml`, `2.0.0.toml`. Not numbered sequences. The version in the filename is the version of the project that ships this migration.

### Staging with next/

During development, migration files live in `migrations/next/` with descriptive names:

- `add-email-verified.toml`
- `rename-color-mode.toml`

At release time, `migrable merge <version>` combines all `next/*.toml` into a single `<version>.toml` and empties `next/`.

### Merge algorithm

1. Read all `*.toml` from `next/`, sorted alphabetically.
2. For each section (`structure`, `data`), concatenate array-of-tables entries in file order.
3. Concatenate `description` fields with `"; "` separator.
4. Write the result as `migrations/<version>.toml`.
5. Delete all files from `next/`.
6. If `next/` is empty, warn and create no migration file.

Alphabetical sort determines op order within the merged file. Name staging files accordingly (e.g., `01-add-field.toml`, `02-rename.toml`) when order matters.

### Version tracking

The current schema version is stored as a `_schema_version` key in a TOML config file, set to the semver string of the highest applied migration. Defaults to `"0.0.0"` if absent. In single-file projects, `_schema_version` is stored in the sole file. In multi-file projects, it is stored in the file designated by `version_file` in `migrable.toml`.

### Accumulation

Migration files are never removed. Version N ships all prior migrations. A user upgrading from v1.0 to v3.0 gets all intermediate migrations applied in order (semver comparison). Migrations with versions less than or equal to the current `_schema_version` are skipped.

### Pre-release versions

No migration files for betas or release candidates. Pre-release migrations stay in `next/` until the actual release.

### Fresh installs

If the target config file does not exist, migrable treats it as an empty TOML document. All migrations apply from scratch, building up the config. The file is created atomically on first write.

Structure ops create the foundations (fields, tables). Data ops operate on existing structure -- `append` and `transform` require their target path to already exist (created by a prior structure op in the same or earlier migration). Ops that tolerate missing paths:

- `add_field` creates the field and intermediate tables. No-op if the field already exists (preserves existing value).
- `set_value` creates the key and intermediate tables.
- `rename_field` is a no-op if the source path is missing.
- `remove_field`, `drop_collection` are no-ops if the path is missing.
- `merge_defaults` creates all missing keys recursively.

### Migrations with no ops

A migration file with a `description` but no `[[structure]]` or `[[data]]` entries is valid. The version is still bumped. This can be used for documentation-only version markers.

---

## Migration File Structure

Migration files have explicit sections with a fixed execution order: **structure -> data**. Both sections are optional.

```toml
description = "Add user verification fields"

[[structure]]
op = "add_field"
path = "email_verified"
type = "boolean"
default = false
down = { op = "remove_field", path = "email_verified" }

[[structure]]
op = "add_field"
path = "verified_at"
type = "datetime"
down = { op = "remove_field", path = "verified_at" }

[[data]]
op = "set_value_where"
path = "users"
match_mode = "subset"
where = { role = "admin" }
set = { email_verified = true }

[data.down]
op = "set_value_where"
path = "users"
match_mode = "subset"
where = { role = "admin" }
set = { email_verified = false }
```

### Down ops: inline, header, and array forms

Simple down ops fit on one line as inline tables:

```toml
down = { op = "remove_field", path = "email_verified" }
```

Complex down ops with nested fields use the `[section.down]` header form. Each `[data.down]` or `[structure.down]` attaches to the most recent `[[data]]` or `[[structure]]` entry (standard TOML array-of-tables behavior):

```toml
[[data]]
op = "set_value_where"
path = "users"
match_mode = "subset"
where = { role = "admin" }
set = { email_verified = true }

[data.down]
op = "set_value_where"
path = "users"
match_mode = "subset"
where = { role = "admin" }
set = { email_verified = false }
```

When reversal requires multiple ops (e.g., undoing `merge_defaults`), `down` can be an array of inline tables. During rollback, array entries execute in reverse order:

```toml
[[structure]]
op = "merge_defaults"
path = "ui"
value = { theme = "dark", font_size = 14 }
down = [
  { op = "remove_field", path = "ui.theme" },
  { op = "remove_field", path = "ui.font_size" },
]
```

### Irreversible ops

When an op is not meaningfully reversible (e.g., a lossy `transform`), declare `down = "irreversible"` instead of providing a fake reversal:

```toml
[[data]]
op = "transform"
path = "password"
expr = "value.upperAscii()"
down = "irreversible"
```

`migrable validate` accepts `"irreversible"`. `migrable migrate --rollback` errors if any op in the target migration is marked irreversible.

`migrable validate` checks that every op has a `down` field (single op, array of ops, or `"irreversible"`).

### Absent defaults

TOML has no null. Omitting the `default` field in `add_field` means the field is declared in the migration but no value is written. The field's key will not exist in the config until data arrives through a subsequent op or user action.

---

## Op Vocabulary

14 ops across 3 categories.

### Structure ops

Change what fields and tables exist.

| Op | Fields | Description | Behavior when path exists | Behavior when path missing |
|----|--------|-------------|--------------------------|---------------------------|
| `add_field` | `path`, `type`, `default` (optional), `down` | Add a new field | No-op (preserves existing value) | Creates field and intermediate tables |
| `remove_field` | `path`, `down` | Remove a field | Removes the field | No-op |
| `rename_field` | `from`, `to`, `down` | Rename a field (or table) | Renames the key. Error if `to` already exists. | No-op if source missing |
| `move_field` | `from`, `to`, `down` | Move a field between tables | Reads value, writes to new path, deletes old. Error if `to` already exists. | No-op if source missing |
| `add_collection` | `path`, `fields` (optional), `down` | Create a new table or array of tables | Error | Creates the table |
| `drop_collection` | `path`, `down` | Remove a table or array of tables | Removes it | No-op |

`rename_field` works on any key, including table-valued keys. Renaming `server` to `network` renames the `[server]` table header.

### Data ops

Change values within existing structure.

| Op | Fields | Description | Behavior when path missing |
|----|--------|-------------|---------------------------|
| `set_value` | `path`, `value`, `down` | Unconditional overwrite | Creates key and intermediate tables |
| `set_value_where` | `path`, `where`, `match_mode`, `set`, `down` | Conditional set on matching items | Error if path is not an array |
| `remove_where` | `path`, `where`, `match_mode`, `down` | Remove matching items | Error if path is not an array |
| `append` | `path`, `value`, `down` | Add element to array | Error if array missing |
| `transform` | `path`, `expr` (CEL), `down` | Compute new value from old | Error if path missing |
| `merge_defaults` | `path`, `value`, `down` | Deep merge missing keys | Creates missing keys |
| `merge_defaults_by_key` | `path`, `match_field`, `defaults` (array), `down` | Merge missing attributes into array items matched by key | Error if path is not an array |

**`merge_defaults` specifics**: Tables merge recursively (adding missing keys, never overwriting existing). Arrays and scalars are atomic -- if a value exists, keep it; if missing, set the default. Array manipulation should use `set_value_where`, `append`, or `remove_where`.

**`merge_defaults_by_key` specifics**: For each item in the target array, find a matching item in `defaults` by the `match_field` key. For matched items, add missing attributes (flat merge per item, never overwrite existing). Items in `defaults` that have no match in the target are NOT added (respects user removals). Items in the target with no match in `defaults` are left unchanged.

### Raw ops

Escape hatch for operations not covered by the op vocabulary.

| Op | Fields | Description |
|----|--------|-------------|
| `raw` | `content`, `path` (optional), `down` | Insert TOML content into the document |

**`raw` semantics**: `content` is a string containing valid TOML key-value pairs. If `path` is provided, the content is parsed and its keys are set within the table at that path. If `path` is omitted, keys are set at the document root. Existing keys at the target are overwritten. This is an escape hatch for operations that the other 13 ops cannot express.

---

## Op-to-API Mapping

Each op maps to [go-toml-edit](https://github.com/smm-h/go-toml-edit) API calls.

| Op | go-toml-edit API |
|----|-----------------|
| `add_field` | `doc.Get(path)` to check existence; if absent, `doc.SetCreate(path, value)` |
| `remove_field` | `doc.Delete(path)` |
| `rename_field` | `doc.Rename(path, newKey)` |
| `move_field` | `doc.Get(from)` to read value, `doc.SetCreate(to, value)`, `doc.Delete(from)` |
| `add_collection` | `doc.NewTable(path)` + optional `doc.SetCreate` for each field |
| `drop_collection` | `doc.Delete(path)` |
| `set_value` | `doc.SetCreate(path, value)` |
| `set_value_where` | `doc.Items(path)` to iterate + `doc.Set` on matches |
| `remove_where` | `doc.Items(path)` to iterate + `doc.Delete` on matches |
| `append` | `doc.Get(path)` to read array, append element, `doc.Set(path, updated)` to write back |
| `transform` | `doc.Get(path)` to read, CEL evaluate, `doc.Set(path, result)` |
| `merge_defaults` | `doc.MergeDefaults(path, value)` |
| `merge_defaults_by_key` | `doc.Items(path)` to iterate, match by key, set missing attributes per item |
| `raw` | Parse `content` as TOML, iterate key-value pairs, `doc.SetCreate` each at `path` |

---

## Match Modes

Used by `set_value_where` and `remove_where` to identify target items in arrays of tables.

| Mode | Match field type | Behavior | Default |
|------|-----------------|----------|---------|
| `subset` | table | Item contains all match key-value pairs (can have extra keys) | Yes (default if `match_mode` omitted) |
| `exact` | table | Item has exactly these keys and values, nothing more | |
| `all` | (none) | Matches every item in the array | |
| `index` | integer | Matches by array position (zero-based, supports negative indices) | |
| `has_key` | string | Item has this key, regardless of value | |
| `not_has_key` | string | Item lacks this key | |
| `regex` | table | String values matched as Go `regexp` patterns (full match). Non-string fields are skipped. All fields in the `where` table must match. | |

Each mode is explicitly declared via the `match_mode` field. `subset` is the default when omitted. Out-of-range `index` values are a no-op (no match).

---

## Type System

TOML-native types used in `add_field` declarations.

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
| `array` | `[1, 2, 3]` | Ordered collection of homogeneous values |
| `table` | `[section]` / `key = {}` | Key-value mapping |

Every `add_field` op must declare a `type`. This ensures migration files are self-documenting. `migrable validate` checks that default values match their declared type.

### Inline tables vs standard tables

TOML has two representations for tables: standard `[table]` headers and inline `key = {}` tables. go-toml-edit preserves whichever form exists in the source file. When creating new tables (via `add_collection` or intermediate table creation), migrable uses standard `[table]` headers. When adding fields to an existing inline table, go-toml-edit handles extending it in-place.

---

## Dot-Path Addressing

All paths use dot-separated keys to address nested TOML values. The path syntax matches [go-toml-edit](https://github.com/smm-h/go-toml-edit)'s path semantics.

### Syntax

| Notation | Meaning | Example |
|----------|---------|---------|
| `a.b.c` | Nested table access | `[a]` -> `[a.b]` -> key `c` |
| `a[N]` | Array index (zero-based) | Third element of array `a`: `a[2]` |
| `a[-1]` | Negative array index | Last element of array `a` |
| `a\.b` | Escaped dot (literal dot in key name) | Key named `a.b` |
| `"quoted key"` | Quoted segment | Key with spaces or special characters |

Paths are resolved left to right. Write operations (`SetCreate`) create intermediate tables as needed. go-toml-edit resolves dotted keys (`a.b.c = 1`) and nested table headers (`[a.b]` + `c = 1`) identically -- the path addresses the semantic structure, not the syntactic form.

### Examples

- `server.host` -- key `host` inside table `server`
- `servers[0].ip` -- key `ip` in the first entry of array of tables `servers`
- `servers[-1].port` -- key `port` in the last entry of array of tables `servers`
- `config.ui\.settings.theme` -- key `theme` inside key `ui.settings` (literal dot) inside table `config`

---

## Down Ops (Reversibility)

Every op **must** have a `down` field. There is no auto-inference. The `down` field can be:

- A single op as an inline table or `[section.down]` header.
- An array of ops as inline tables (for multi-step reversal). Executed in reverse order during rollback.
- The string `"irreversible"` for ops that cannot be meaningfully reversed.

### Reversal pairs

| Forward op | Down op |
|-----------|---------|
| `add_field` | `remove_field` |
| `remove_field` | `add_field` (with type and default) |
| `rename_field` | `rename_field` (from/to swapped) |
| `move_field` | `move_field` (from/to swapped) |
| `add_collection` | `drop_collection` |
| `drop_collection` | `add_collection` (with fields) |
| `set_value` | `set_value` (with old value) |
| `set_value_where` | `set_value_where` (reverse the set values) |
| `remove_where` | `append` (with the removed items) |
| `append` | `remove_where` (matching the appended item) |
| `transform` | `transform` (reverse expression) |
| `merge_defaults` | Array of `remove_field` ops for each key that was added |
| `merge_defaults_by_key` | Array of `set_value_where` or `remove_where` ops to undo per-item attribute additions |
| `raw` | `raw` (reverse content) |

### Rollback limitations

Rollback restores structure but may lose user-customized values. For example, rolling back `remove_field` restores the field with its declared default, not the user's previous value. This is an inherent limitation of declarative migrations without snapshotting.

### Rollback execution order

`migrable migrate --rollback` reverses the last applied migration by executing all down ops in **reverse section order**: data -> structure. Within each section, ops execute in reverse order. Array-valued `down` fields execute their entries in reverse order. After rollback, `_schema_version` is set to the version of the migration applied before the rolled-back one, or `"0.0.0"` if the first migration was rolled back.

If any op in the target migration has `down = "irreversible"`, rollback aborts with an error before executing any down ops.

---

## CLI

```
migrable migrate                    Run pending migrations
  --dry-run                        Preview changes (diff output)
  --rollback                       Reverse the last applied migration (uses down ops)
  --config-dir <path>              Override location of migrable.toml (default: . then .migrable/)

migrable status                    Show current version, pending migrations, and file paths

migrable merge <version>           Combine next/*.toml into <version>.toml, empty next/

migrable validate                  Validate migration files (syntax, type checks, down op presence)

migrable init                      Scaffold migrable.toml and migrations/ directory

migrable --version                 Show version
migrable --help                    Show help
```

### Dry run output

`--dry-run` uses `tomledit.Diff(before, after)` to show changes without writing. Output lists each change with its category, path, and values:

| Change type | Output |
|-------------|--------|
| Added | Path, new value |
| Removed | Path, old value |
| Modified | Path, old value, new value |

---

## go-toml-edit

migrable uses [go-toml-edit](https://github.com/smm-h/go-toml-edit) (`github.com/smm-h/go-toml-edit`) as its core dependency for all TOML manipulation.

### What it provides

- **Comment-preserving editing**: Modifies TOML values without destroying comments, whitespace, or formatting. Only modified nodes are re-rendered; everything else uses original bytes.
- **Dot-path addressing**: Get, set, delete, and rename values using dot-path strings with array index support.
- **Iteration**: `Items()` for iterating over arrays and arrays of tables, enabling conditional ops like `set_value_where`.
- **Deep merge**: `MergeDefaults()` for recursive table merging with atomic scalars and arrays.
- **Table creation**: `NewTable()` and `NewArrayTable()` for creating TOML structural elements.
- **Diffing**: `Diff(before, after)` compares two documents and returns structured changes (Added, Removed, Modified) with paths and values.

### Atomicity

All ops for a single file are applied in memory before writing -- all-or-nothing per file. If any op fails, the entire in-memory document is discarded and the file on disk is untouched. The error report identifies which op failed and why.

TOML files are written atomically: write to a temporary file in the same directory, then rename. This prevents partial writes from corrupting config files. The entire file is written only if at least one op produced a change. A change is determined by data comparison, not byte comparison -- setting a value to its current value is not a change.

### Multi-file atomicity

In multi-file projects, each migration is committed independently. After a migration succeeds, all affected files are written transactionally: temporary files are written first, then renamed to their final paths. If any write fails, all temporary files are cleaned up and no files are modified for that migration. This prevents inconsistent state across files within a single migration. Previously committed migrations remain on disk.

---

## Engine Behavior

The migration engine follows this sequence for each invocation:

1. **Read `migrable.toml`** to discover target TOML files and the migrations directory.
2. **For each target file**, parse with `tomledit.Parse`. If the file does not exist, start with an empty document.
3. **Read `_schema_version`** from the version file (defaults to `"0.0.0"` if absent).
4. **Discover migration files** in the migrations directory. Parse version from filename. Sort by semver.
5. **Filter** to versions greater than the current schema version.
6. **For each migration in order**:
   - Parse the TOML migration file.
   - Apply `[[structure]]` ops sequentially, each translating to go-toml-edit API calls.
   - Apply `[[data]]` ops sequentially.
   - If any op fails, discard all in-memory changes for this migration, report the error (which migration, which op, why), and stop. Previously committed migrations remain on disk.
   - On success, update `_schema_version` in the version file to this migration's version.
   - Write transactionally: write all temp files for affected files, then rename all. If any write fails, clean up all temps and stop.
7. **Report results**: applied count, current version, any warnings.

---

## CEL Integration

Transform expressions use CEL (Common Expression Language) via `cel-go` (Google's official Go implementation).

### Why CEL

- **Non-Turing-complete**: Guaranteed termination. No infinite loops.
- **Sandboxed**: No file access, no network access, no side effects.
- **Expressive**: String operations, conditionals, list/map operations (`.map`, `.filter`, `.exists`, `.all`).

### Usage in migrable

CEL is used exclusively for `transform` ops to compute new TOML values from existing ones. The engine reads the current value via `doc.Get(path)`, evaluates the CEL expression with the value bound as `value`, and writes the result back via `doc.Set(path, result)`.

### Type marshaling

| TOML type | CEL type |
|-----------|----------|
| `string` | `string` |
| `integer` | `int` |
| `float` | `double` |
| `boolean` | `bool` |
| `datetime` | `google.protobuf.Timestamp` |
| `local_datetime`, `local_date`, `local_time` | `string` (ISO 8601 representation) |
| `array` | `list` |
| `table` | `map` |

### Example expressions

| CEL expression | Effect |
|---------------|--------|
| `value == 'dark' ? 'night' : 'day'` | Remap string values |
| `value + '_suffix'` | Append to string |
| `value * 100` | Scale numeric value |
| `size(value) > 0 ? value : 'default'` | Conditional default |

TOML literal strings (`'''...'''`) are recommended for CEL expressions containing regex or backslashes:

```toml
[[data]]
op = "transform"
path = "email"
expr = '''value.matches('\w+@example\.com') ? value : value + '.legacy' '''
```

---

## Conformance Test Suite

TOML test files with input config, migration ops, and expected output. Published alongside migrable. Usable by any language that wants to implement a compatible migration engine (implementors already need a TOML parser since they are building a TOML migration engine).

### Test file format

```toml
description = "add_field creates nested path"

input = '''
[ui]
font_size = 14
'''

expected = '''
[ui]
font_size = 14
color = "blue"
'''

[[migration.structure]]
op = "add_field"
path = "ui.color"
type = "string"
default = "blue"
down = { op = "remove_field", path = "ui.color" }
```

### Comparison semantics

Tests compare data equivalence, not byte-level output. The test runner parses both `expected` and the migration result as TOML, then compares the resulting data structures. This allows any valid TOML formatting in the `expected` field.

For round-trip and formatting preservation tests (migrable-specific), a separate `expected_bytes` field can be used for exact byte comparison.

### Coverage

- Op correctness for all 14 ops
- Idempotency: `add_field` on existing field is a no-op; double-run produces identical state
- Edge cases: missing paths, type conflicts, empty configs
- Match mode behavior for all 7 modes (including out-of-range index, regex on non-string fields)
- `merge_defaults` recursion (nested tables, atomic arrays, atomic scalars)
- `merge_defaults_by_key` (matched items, unmatched items, empty arrays)
- `move_field` across tables (value preservation, source deletion)
- Transform expressions (CEL evaluation, type marshaling, various expression types)
- Fresh install scenarios (ops applied to configs that never had the old shape)
- Down op execution (rollback correctness, version tracking after rollback)
- Multi-file targeting and cross-file transactional writes
- Version tracking (`_schema_version` bumps correctly, skips already-applied)

---

## rlsbl Integration

migrable is a consumer tool for rlsbl, not a child project. rlsbl delegates to migrable; migrable has no knowledge of rlsbl internals.

### Integration points

| rlsbl command | migrable command | Purpose |
|--------------|-----------------|---------|
| `rlsbl migrate` | `migrable migrate` | Shell out to run pending migrations |
| `rlsbl release` | `migrable merge <version>` | Combine `next/` into versioned migration during release flow |

rlsbl passes `--config-dir .rlsbl/` to migrable when invoking it, so that migrable reads `.rlsbl/migrable.toml`. migrable itself does not search `.rlsbl/` by default.

### Dependency handling

migrable is an optional dependency. If not installed, `rlsbl migrate` shows a clear error: "migrable not found. Install from..."

### Planned rlsbl changes (separate effort)

These changes to rlsbl support the integration but are not part of migrable itself:

- Add `tomlkit` dependency (replace TOML regex in `tagging.py` and `pypi.py`)
- Add `filelock >= 3.20.3` dependency (replace Unix-only `fcntl` in `lock.py`, add Windows support)
- Remove `config` subcommand tree (clean break, pre-1.0)
- Add `migrate` command (delegates to migrable)
- Absorb `config show` into `status` command

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
| Op engine (interpret ops, call go-toml-edit API) | 400-600 | 14 ops, type handling, match modes |
| CEL integration | 100-200 | cel-go glue, value marshaling between TOML and CEL types |
| CLI | 200-300 | cobra commands, flags, help text |
| migrable.toml parser | 80-120 | TOML parsing via go-toml-edit, variable expansion |
| next/ merge tool | 100-150 | TOML file combination logic |
| Conformance test suite | 500-700 | Comprehensive TOML test cases |
| Tests (Go) | 900-1400 | Table-driven tests for all ops |
| **Total** | **~2300-3500 LOC** | |

---

## Future Work

- **JSONC backend**: Support for migrating JSONC config files. Would require a surgical comment-preserving JSONC editor or equivalent library.
- **YAML backend**: Support for YAML config files with comment preservation.
- **Schema diffing**: Auto-generate migration files from before/after TOML schema comparison.
- **GUI**: Web UI for browsing migration history and visualizing config changes.
