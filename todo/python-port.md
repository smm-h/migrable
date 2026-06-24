# Python port of migrable's migration engine

## Problem

migrable is Go-only. Its Python "package" is a thin CLI wrapper that downloads the Go binary. There is no Python library API.

strictcli (which has both Python and Go implementations) wants to integrate migrable's config migration engine natively. strictcli's config field system already supports schema versioning infrastructure (`_schema_version` framework field, `ConfigMigrator` interface design), but the actual migration engine integration is blocked on migrable having a Python implementation.

## What's needed

A proper Python port of migrable's core engine — not a wrapper around the Go binary. The port should cover:
- Migration file discovery and ordering (semver-named TOML files)
- Schema version tracking (`_schema_version` field in config files)
- Core structural ops: add_field, remove_field, rename_field, move_field, set_value
- Atomic file writes (temp file + rename)
- Rollback support (down ops)
- A shared conformance test suite between Go and Python engines

## Consumer

strictcli's `todo/migrable-engine-integration.md` describes the integration design on the consumer side.
