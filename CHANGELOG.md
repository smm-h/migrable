# Changelog

## Unreleased

## 0.1.0

Initial release. Declarative, comment-preserving TOML config file migrations.

### Added

- Migration engine with 14 ops across 3 categories: structure (add/remove/rename/move field, add/drop collection), data (set value, set/remove where, append, transform, merge defaults, merge defaults by key), and raw
- 7 match modes for conditional ops: subset, exact, all, index, has_key, not_has_key, regex
- CEL expression evaluation for transform ops with string extensions, sandboxed and guaranteed to terminate
- Comment-preserving TOML editing via go-toml-edit
- Atomic file writes (temp file + rename) with multi-file transactional commits
- Rollback support via explicit down ops, with irreversible op detection
- Migration staging with next/ directory and merge workflow
- Shell variable expansion in config file paths (e.g. $HOME, $XDG_CONFIG_HOME)
- Semver-based migration ordering and _schema_version tracking
- CLI commands: migrate (--dry-run, --rollback), status, validate, merge, init
- Conformance test suite with 34 TOML test files covering all ops, match modes, and edge cases
- Cross-platform builds via goreleaser (linux/darwin/windows, amd64/arm64)
