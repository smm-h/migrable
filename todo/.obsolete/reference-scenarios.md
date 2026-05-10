# Reference Scenarios from rlsbl ConfigMigrator Tests

Extracted from `rlsbl/tests/test_config_migrator.py` and `rlsbl/tests/test_schema_loader.py`.
These scenarios serve as conformance test material for migrable's declarative engine.

---

## 1. Merge Strategy: deep_recursive

Recursively merges missing keys at all nesting levels. Existing values are never overwritten.

### 1.1 Adds missing top-level key

| | Value |
|---|---|
| **Input** | `{"a": 1}` |
| **Defaults** | `{"a": 99, "b": 2}` |
| **Output** | `{"a": 1, "b": 2}` |
| **Changed** | `true` |
| **Edge case** | Existing keys preserved even if defaults differ |

### 1.2 Does not overwrite existing values

| | Value |
|---|---|
| **Input** | `{"a": 1, "b": 2}` |
| **Defaults** | `{"a": 99, "b": 100}` |
| **Output** | `{"a": 1, "b": 2}` |
| **Changed** | `false` |
| **Edge case** | No-op when all keys already exist |

### 1.3 Recurses into nested dicts

| | Value |
|---|---|
| **Input** | `{"outer": {"existing": "yes"}}` |
| **Defaults** | `{"outer": {"existing": "no", "new_key": "added"}}` |
| **Output** | `{"outer": {"existing": "yes", "new_key": "added"}}` |
| **Changed** | `true` |
| **Edge case** | Nested objects merged recursively, not replaced |

### 1.4 Deeply nested (3+ levels)

| | Value |
|---|---|
| **Input** | `{"a": {"b": {"c": 1}}}` |
| **Defaults** | `{"a": {"b": {"c": 99, "d": 2}, "e": 3}}` |
| **Output** | `{"a": {"b": {"c": 1, "d": 2}, "e": 3}}` |
| **Changed** | `true` |
| **Edge case** | Works at arbitrary depth; multiple branches merged in one pass |

### 1.5 No change returns false

| | Value |
|---|---|
| **Input** | `{"a": {"b": 1}}` |
| **Defaults** | `{"a": {"b": 99}}` |
| **Output** | `{"a": {"b": 1}}` |
| **Changed** | `false` |
| **Edge case** | Change detection is accurate even with nested structures |

### 1.6 Deep copies defaults (no shared references)

| | Value |
|---|---|
| **Input** | `{}` |
| **Defaults** | `{"key": {"inner": [1, 2, 3]}}` |
| **Output** | `{"key": {"inner": [1, 2, 3]}}` |
| **Edge case** | Mutating output must not affect the defaults object |

### 1.7 Full run with deep_recursive strategy

| | Value |
|---|---|
| **Existing file** | `{"ui": {"color": "blue"}}` |
| **Schema defaults** | `{"ui": {"color": "red", "font_size": 14}, "version": 1}` |
| **File after run** | `{"ui": {"color": "blue", "font_size": 14}, "version": 1}` |
| **File written** | `true` |
| **Edge case** | Integration test: reading from disk, merging, writing back |

---

## 2. Merge Strategy: flat_dict

Only adds missing top-level keys. Never recurses into nested objects.

### 2.1 Adds missing keys

| | Value |
|---|---|
| **Input** | `{"a": 1}` |
| **Defaults** | `{"a": 99, "b": 2, "c": 3}` |
| **Output** | `{"a": 1, "b": 2, "c": 3}` |
| **Changed** | `true` |

### 2.2 Does not overwrite existing

| | Value |
|---|---|
| **Input** | `{"a": 1, "b": 2}` |
| **Defaults** | `{"a": 99, "b": 100}` |
| **Output** | `{"a": 1, "b": 2}` |
| **Changed** | `false` |

### 2.3 Does NOT recurse into nested dicts

| | Value |
|---|---|
| **Input** | `{"nested": {"x": 1}}` |
| **Defaults** | `{"nested": {"x": 99, "y": 2}}` |
| **Output** | `{"nested": {"x": 1}}` |
| **Changed** | `false` |
| **Edge case** | Key "nested" exists, so its value is preserved wholesale -- no sub-merge |

### 2.4 Empty target gets all defaults

| | Value |
|---|---|
| **Input** | `{}` |
| **Defaults** | `{"a": 1, "b": 2}` |
| **Output** | `{"a": 1, "b": 2}` |
| **Changed** | `true` |

### 2.5 Full run: merges missing keys into existing file

| | Value |
|---|---|
| **Existing file** | `{"theme": "light"}` |
| **Schema defaults** | `{"theme": "dark", "debug": false}` |
| **File after run** | `{"theme": "light", "debug": false}` |
| **File written** | `true` |

---

## 3. Merge Strategy: list_by_key

Matches items in a list by a key field. For each matching item, adds missing attributes (flat merge per item). Items in defaults that are not in the target are NOT added back (respects user removals).

### 3.1 Adds missing attributes to matched items

| | Value |
|---|---|
| **Input** | `[{"key": "foo", "color": "red"}]` |
| **Defaults** | `[{"key": "foo", "color": "blue", "size": 10}]` |
| **Match field** | `"key"` |
| **Output** | `[{"key": "foo", "color": "red", "size": 10}]` |
| **Changed** | `true` |

### 3.2 Does not overwrite existing attributes

| | Value |
|---|---|
| **Input** | `[{"key": "foo", "color": "red", "size": 5}]` |
| **Defaults** | `[{"key": "foo", "color": "blue", "size": 10}]` |
| **Match field** | `"key"` |
| **Output** | `[{"key": "foo", "color": "red", "size": 5}]` |
| **Changed** | `false` |

### 3.3 Does NOT add back removed items

| | Value |
|---|---|
| **Input** | `[{"key": "foo", "color": "red"}]` |
| **Defaults** | `[{"key": "foo", "color": "blue", "size": 10}, {"key": "bar", "color": "green", "size": 20}]` |
| **Match field** | `"key"` |
| **Output** | `[{"key": "foo", "color": "red", "size": 10}]` |
| **Changed** | `true` |
| **Edge case** | "bar" exists in defaults but not in target -- it stays absent |

### 3.4 Multiple items matched

| | Value |
|---|---|
| **Input** | `[{"id": "a", "val": 1}, {"id": "b", "val": 2}]` |
| **Defaults** | `[{"id": "a", "val": 99, "new": "x"}, {"id": "b", "val": 99, "new": "y"}]` |
| **Match field** | `"id"` |
| **Output** | `[{"id": "a", "val": 1, "new": "x"}, {"id": "b", "val": 2, "new": "y"}]` |
| **Changed** | `true` |

### 3.5 Empty target -- nothing to match

| | Value |
|---|---|
| **Input** | `[]` |
| **Defaults** | `[{"key": "foo", "size": 10}]` |
| **Match field** | `"key"` |
| **Output** | `[]` |
| **Changed** | `false` |
| **Edge case** | Empty list is treated as "user removed everything" |

### 3.6 Full run with list_by_key strategy

| | Value |
|---|---|
| **Existing file (items.json)** | `[{"key": "a", "val": 1}, {"key": "b", "val": 2}]` |
| **Schema defaults** | `[{"key": "a", "val": 99, "extra": "x"}, {"key": "b", "val": 99, "extra": "y"}, {"key": "c", "val": 3, "extra": "z"}]` |
| **Match field** | `"key"` |
| **File after run** | `[{"key": "a", "val": 1, "extra": "x"}, {"key": "b", "val": 2, "extra": "y"}]` |
| **File written** | `true` |
| **Edge case** | "c" not in target, so not added. Only existing items enriched. |

---

## 4. Version Tracking and Migration Ordering

### 4.1 Skips already-applied migrations

| | Value |
|---|---|
| **Config state** | `{"_schema_version": 1}` |
| **Migrations** | `[{version: 1, apply: sets "mutated": true}]` |
| **Result** | Config unchanged, migration not called |
| **Changed files** | `(empty set)` |
| **Edge case** | Version equality means "already applied" |

### 4.2 Applies pending migration and bumps version

| | Value |
|---|---|
| **Config state** | `{"_schema_version": 0}` |
| **Migrations** | `[{version: 1, apply: sets "new_field": "added"}]` |
| **Result** | `{"_schema_version": 1, "new_field": "added"}` |
| **Changed files** | `{"config.json"}` |

### 4.3 Multiple migrations applied in version order

| | Value |
|---|---|
| **Config state** | `{}` (no _schema_version, treated as 0) |
| **Migrations** | `[{version: 2, apply: sets "step2": true}, {version: 1, apply: sets "step1": true}]` |
| **Execution order** | version 1 first, then version 2 (sorted regardless of list order) |
| **Result** | `{"_schema_version": 2, "step1": true, "step2": true}` |
| **Edge case** | Declaration order does not matter; version number determines execution order |

### 4.4 Migration runs after defaults merge

| | Value |
|---|---|
| **Existing file** | `{"flag": true}` |
| **Defaults** | `{"flag": true}` |
| **Migrations** | `[{version: 1, apply: sets "flag": false}]` |
| **File after run** | `{"flag": false, "_schema_version": 1}` |
| **Edge case** | Merge happens first, then migrations run on the merged state |

### 4.5 Custom schema_version_key

| | Value |
|---|---|
| **Config state** | `{}` |
| **Schema setting** | `"schema_version_key": "__version__"` |
| **Migrations** | `[{version: 1, apply: sets "migrated": true}]` |
| **File after run** | `{"__version__": 1, "migrated": true}` |
| **Edge case** | `_schema_version` must NOT appear; only the custom key is used |

### 4.6 Default schema_version_key is "_schema_version"

When the schema omits `schema_version_key`, it defaults to `"_schema_version"`.

---

## 5. Multi-File Migrations

### 5.1 Migration mutates multiple files

| | Value |
|---|---|
| **Files before** | `config.json: {"mode": "old"}`, `state.json: {"count": 0}` |
| **Migration** | `{version: 1, apply: sets config.json.mode="new", state.json.count=1}` |
| **Files after** | `config.json: {"mode": "new", "_schema_version": 1}`, `state.json: {"count": 1}` |
| **Changed files** | Both `config.json` and `state.json` |
| **Edge case** | Schema version is stored in the first dict-type file; secondary file has no version key |

### 5.2 Detects changes in secondary files

| | Value |
|---|---|
| **Files before** | `config.json: {}`, `other.json: {}` |
| **Migration** | `{version: 1, apply: sets other.json.touched=true}` |
| **Changed files** | Both -- config.json because schema_version bumped, other.json because content changed |
| **Edge case** | Change detection works per-file via snapshot comparison |

---

## 6. Missing File Handling

### 6.1 Creates file from defaults when missing

| | Value |
|---|---|
| **File on disk** | Does not exist |
| **Schema defaults** | `{"theme": "dark", "version": 1}` |
| **File after run** | `{"theme": "dark", "version": 1}` |
| **File written** | `true` |
| **Edge case** | Missing file is not an error; defaults are written as initial content |

### 6.2 Returns None for missing JSON file (load_json)

When the engine reads a file that does not exist, it returns `null`/`None` and proceeds to use defaults.

### 6.3 Returns None for malformed JSON file

When the engine reads a file containing invalid JSON, it returns `null`/`None` (treated same as missing).

---

## 7. Idempotency

### 7.1 Double run is a no-op

| | Value |
|---|---|
| **Schema defaults** | `{"a": 1, "b": 2}` |
| **Migrations** | `[{version: 1, apply: sets "c": 3}]` |
| **First run result** | File written: `true`, content: `{"a": 1, "b": 2, "c": 3, "_schema_version": 1}` |
| **Second run result** | File written: `false` |
| **Edge case** | Running twice must produce identical state; second run detects no changes |

---

## 8. Change Detection

### 8.1 No write when nothing changed

| | Value |
|---|---|
| **Existing file** | `{"theme": "dark", "debug": false}` |
| **Schema defaults** | `{"theme": "dark", "debug": false}` |
| **Migrations** | `[]` |
| **File written** | `false` |
| **Edge case** | File is not rewritten if content is identical after merge |

### 8.2 Change detection per file in multi-file schemas

Each file in the schema is independently tracked. A file is only written if its content changed (either from defaults merge or from migration). The run result is a map of `{filename: bool}`.

---

## 9. Schema Loading (Declarative Schema Definition)

These scenarios describe how a declarative schema file is parsed and resolved into a runtime schema object.

### 9.1 Schema file structure

```json
{
  "schema_version_key": "_schema_version",
  "files": [
    {
      "path": "config.json",
      "defaults_path": "defaults/config.json",
      "merge_strategy": "deep_recursive"
    },
    {
      "path": "segments.json",
      "defaults_path": "defaults/segments.json",
      "merge_strategy": "list_by_key",
      "match_field": "key"
    }
  ]
}
```

The loader resolves `defaults_path` by reading the referenced file and inlining its content as `defaults`.

### 9.2 Defaults can be a dict

```json
{"key1": "value1", "nested": {"a": 1, "b": 2}}
```

### 9.3 Defaults can be a list

```json
[{"id": "a", "val": 1}, {"id": "b", "val": 2}]
```

### 9.4 Missing defaults file is an error

If `defaults_path` points to a nonexistent file, the loader raises `SchemaLoadError("Defaults file not found")`.

### 9.5 Malformed defaults file is an error

If the defaults file contains invalid JSON, the loader raises `SchemaLoadError("Failed to read defaults file")`.

### 9.6 Missing schema file returns null

If the schema definition file does not exist, the loader returns `null` (no migration needed for this project).

### 9.7 No migrations directory means empty migrations list

If the migrations directory does not exist, the schema loads successfully with `migrations: []`.

---

## 10. Migration File Validation

These describe what constitutes a valid migration definition and what errors to raise for invalid ones.

### 10.1 Valid migration structure

Each migration must have:
- `version` (integer) -- must match the numeric filename prefix
- `description` (string)
- `apply` (callable/function)

### 10.2 Missing apply -- error

Error message: `"missing required attributes: apply"`

### 10.3 Missing version -- error

Error message: `"missing required attributes: version"`

### 10.4 Missing description -- error

Error message: `"missing required attributes: description"`

### 10.5 Non-callable apply -- error

Error message: `"'apply' must be callable"`

### 10.6 Non-integer version -- error

Error message: `"'version' must be an int"`

### 10.7 Version/filename mismatch -- error

If filename is `001_foo` but `version = 5`, error:
`"Migration file 001_foo.py has version=5, expected version=1 (must match filename prefix)"`

### 10.8 Migrations sorted by numeric prefix

Regardless of filesystem order, migrations are returned sorted by their numeric prefix (and thus version number). Example: files `010_last`, `003_middle`, `001_first` yield versions `[1, 3, 10]`.

### 10.9 Non-matching files ignored

Files without a numeric prefix (e.g., `README.md`, `helper.py`) and directories (e.g., `__pycache__/`) in the migrations directory are silently ignored.

---

## 11. Integration: Schema Load + Migration Run

### 11.1 End-to-end: load schema, run migrator, verify output

| | Value |
|---|---|
| **Schema definition** | `{schema_version_key: "_sv", files: [{path: "config.json", defaults_path: "defaults/config.json", merge_strategy: "deep_recursive"}]}` |
| **Defaults file** | `{"theme": "dark", "version": 1}` |
| **Migration 001** | Sets `config.json.debug = true` |
| **Config file before** | Does not exist |
| **Config file after** | `{"theme": "dark", "version": 1, "debug": true, "_sv": 1}` |
| **Edge case** | Full pipeline: schema load -> defaults applied -> migration executed -> version bumped |

---

## 12. Atomic Write Behavior

### 12.1 Writes via tmp file + rename

The engine writes to a `.tmp` sibling, then atomically renames. After write, no `.tmp` file remains.

### 12.2 Output ends with trailing newline

All written JSON files end with `\n` (POSIX compliance).

### 12.3 Overwrites existing files

If the target file already exists, it is replaced atomically.
