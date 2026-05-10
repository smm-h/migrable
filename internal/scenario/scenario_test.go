package scenario

import "testing"

// --- Normal use cases ---

func TestNormal_AddNewConfigField(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
`).
		Migration("1.0.0", `description = "Add debug field"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = { op = "remove_field", path = "debug" }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0").
		AssertKey("config", "debug", false).
		AssertHasKey("config", "debug")
}

func TestNormal_RemoveDeprecatedField(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
legacy_mode = true
`).
		Migration("1.0.0", `description = "Remove legacy_mode"

[[structure]]
op = "remove_field"
path = "legacy_mode"
down = { op = "add_field", path = "legacy_mode", type = "bool", default = true }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertMissingKey("config", "legacy_mode").
		AssertVersion("config", "1.0.0")
}

func TestNormal_RenameKey(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
log_file = "/var/log/app.log"
`).
		Migration("1.0.0", `description = "Rename log_file to log_path"

[[structure]]
op = "rename_field"
from = "log_file"
to = "log_path"
down = { op = "rename_field", from = "log_path", to = "log_file" }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertMissingKey("config", "log_file").
		AssertKey("config", "log_path", "/var/log/app.log").
		AssertVersion("config", "1.0.0")
}

func TestNormal_AddConfigSection(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "0.0.0"
title = "My App"
`).
		Migration("1.0.0", `description = "Add database section"

[[structure]]
op = "add_collection"
path = "database"
fields = { host = "localhost", port = 5432 }
down = { op = "drop_collection", path = "database" }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertHasKey("config", "database").
		AssertKey("config", "database.host", "localhost").
		AssertKey("config", "database.port", int64(5432)).
		AssertVersion("config", "1.0.0")
}

func TestNormal_MergeNewDefaults(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "0.0.0"

[server]
host = "my-host.example.com"
`).
		Migration("1.0.0", `description = "Merge server defaults"

[[data]]
op = "merge_defaults"
path = "server"
value = { host = "default.example.com", port = 8080, debug = false }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertKey("config", "server.host", "my-host.example.com"). // existing not overwritten
		AssertKey("config", "server.port", int64(8080)).
		AssertKey("config", "server.debug", false).
		AssertVersion("config", "1.0.0")
}

func TestNormal_DryRunPreview(t *testing.T) {
	s := New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
`).
		Migration("1.0.0", `description = "Add debug"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`)

	s.MigrateDryRun().
		AssertSuccess().
		AssertApplied(1).
		AssertUntouched("config")
}

func TestScenario_Validate(t *testing.T) {
	// Migration with a missing down op should produce 1 validation error.
	New(t).
		Migration("1.0.0", `description = "Missing down op"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`).
		Validate().
		AssertSuccess().
		AssertErrors(1).
		AssertWarnings(0)
}

func TestScenario_Merge(t *testing.T) {
	s := New(t).
		Migration("0.0.0", `description = "initial"
`)

	s.StagingFile("001_add-debug", `description = "Add debug flag"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = { op = "remove_field", path = "debug" }
`)

	s.StagingFile("002_add-log-level", `description = "Add log_level"

[[structure]]
op = "add_field"
path = "log_level"
type = "string"
default = "info"
down = { op = "remove_field", path = "log_level" }
`)

	s.Merge("1.0.0").
		AssertSuccess().
		AssertOutputExists().
		AssertPathExists("migrations/1.0.0.toml")
}

func TestScenario_Init(t *testing.T) {
	New(t).
		Init().
		AssertSuccess().
		AssertPathExists("migrable.toml").
		AssertPathExists("migrations")
}

func TestNormal_MigrationStatus(t *testing.T) {
	s := New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "1.0.0"
title = "My App"
`).
		Migration("1.0.0", `description = "v1"

[[structure]]
op = "add_field"
path = "a"
type = "string"
default = "x"
`).
		Migration("2.0.0", `description = "v2"

[[structure]]
op = "add_field"
path = "b"
type = "string"
default = "y"
`).
		Migration("3.0.0", `description = "v3"

[[structure]]
op = "add_field"
path = "c"
type = "string"
default = "z"
`)

	s.Status().
		AssertPending(2) // 2.0.0 and 3.0.0 are pending
}

// TestNormal_ScaffoldNewProject is covered by TestScenario_Init.

// --- Not-so-normal use cases ---

func TestNotNormal_RollbackBadRelease(t *testing.T) {
	s := New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
`).
		Migration("1.0.0", `description = "Add debug"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = { op = "remove_field", path = "debug" }
`).
		Migration("2.0.0", `description = "Add log_level"

[[structure]]
op = "add_field"
path = "log_level"
type = "string"
default = "info"
down = { op = "remove_field", path = "log_level" }
`)

	s.Migrate().
		AssertSuccess().
		AssertApplied(2).
		AssertVersion("config", "2.0.0")

	s.Rollback().
		AssertSuccess().
		AssertApplied(1).
		AssertMissingKey("config", "log_level").
		AssertHasKey("config", "debug").
		AssertVersion("config", "1.0.0")
}

func TestNotNormal_TransformWithCEL(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `timeout_ms = 5
`).
		Migration("1.0.0", `description = "Convert seconds to milliseconds"

[[data]]
op = "transform"
path = "timeout_ms"
expr = "value * 1000"
down = { op = "transform", path = "timeout_ms", expr = "int(value / 1000)" }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertKey("config", "timeout_ms", int64(5000)).
		AssertVersion("config", "1.0.0")
}

func TestNotNormal_MultiFileMigration(t *testing.T) {
	New(t).
		MultiFile(map[string]string{
			"config": "config.toml",
			"themes": "themes.toml",
		}, "config").
		Input("config", `title = "My App"
`).
		Input("themes", `[dark]
background = "#000"
`).
		Migration("1.0.0", `description = "Add fields to both"

[[structure]]
op = "add_field"
path = "config.debug"
type = "bool"
default = false

[[structure]]
op = "add_field"
path = "themes.dark.font_size"
type = "integer"
default = 14
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertKey("config", "debug", false).
		AssertKey("themes", "dark.font_size", int64(14)).
		AssertVersion("config", "1.0.0").
		AssertMissingKey("themes", "_schema_version")
}

func TestNotNormal_ConditionalArrayUpdate(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "0.0.0"

[[servers]]
name = "web"
port = 8080

[[servers]]
name = "db"
port = 5432
`).
		Migration("1.0.0", `description = "Update db port"

[[data]]
op = "set_value_where"
path = "servers"
match_mode = "subset"
where = { name = "db" }
set = { port = 5433 }
down = { op = "set_value_where", path = "servers", match_mode = "subset", where = { name = "db" }, set = { port = 5432 } }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0")
}

func TestNotNormal_RawTOMLInjection(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `existing = true
`).
		Migration("1.0.0", `description = "Inject raw TOML"

[[data]]
op = "raw"
content = """
new_key = 42
new_str = "hello"
"""
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertKey("config", "new_key", int64(42)).
		AssertKey("config", "new_str", "hello").
		AssertKey("config", "existing", true).
		AssertVersion("config", "1.0.0")
}

func TestNotNormal_FreshInstall(t *testing.T) {
	// No Input call -- file does not exist.
	New(t).
		SingleFile("config", "config.toml").
		Migration("1.0.0", `description = "Initial schema"

[[structure]]
op = "add_field"
path = "title"
type = "string"
default = "Untitled"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertFileExists("config").
		AssertKey("config", "title", "Untitled").
		AssertKey("config", "debug", false).
		AssertVersion("config", "1.0.0")
}

func TestNotNormal_CrossFileMove(t *testing.T) {
	New(t).
		MultiFile(map[string]string{
			"config": "config.toml",
			"themes": "themes.toml",
		}, "config").
		Input("config", `title = "App"
theme_color = "#ff0000"
`).
		Input("themes", `font = "monospace"
`).
		Migration("1.0.0", `description = "Move theme_color to themes"

[[structure]]
op = "move_field"
from = "config.theme_color"
to = "themes.accent"
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertMissingKey("config", "theme_color").
		AssertKey("themes", "accent", "#ff0000").
		AssertVersion("config", "1.0.0")
}

func TestNotNormal_RegexConditionalUpdate(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "0.0.0"

[[servers]]
name = "prod-web-01"
port = 8080

[[servers]]
name = "staging-web-01"
port = 8081

[[servers]]
name = "prod-db-01"
port = 5432
`).
		Migration("1.0.0", `description = "Mark prod servers"

[[data]]
op = "set_value_where"
path = "servers"
match_mode = "regex"
where = { name = "prod-.*" }
set = { is_prod = true }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0")
}

func TestNotNormal_MergeDefaultsByKey(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "0.0.0"

[[plugins]]
name = "auth"
enabled = true

[[plugins]]
name = "logging"
level = "info"
`).
		Migration("1.0.0", `description = "Add default attrs to plugins"

[[data]]
op = "merge_defaults_by_key"
path = "plugins"
match_field = "name"
defaults = [
  { name = "auth", timeout = 30, enabled = false },
  { name = "logging", level = "debug", format = "json" },
]
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0")
}

func TestNotNormal_DescriptionOnlyMigration(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
`).
		Migration("1.0.0", `description = "No-op migration, version bump only"
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0").
		AssertKey("config", "title", "My App")
}

// --- Edge cases ---

func TestEdge_AllOpsNoOp(t *testing.T) {
	// add_field on fields that already exist -- no data change except version bump.
	New(t).
		MultiFile(map[string]string{
			"config": "config.toml",
			"themes": "themes.toml",
		}, "config").
		Input("config", `title = "My App"
debug = false
`).
		Input("themes", `[dark]
background = "#000"
`).
		Migration("1.0.0", `description = "Add fields that already exist"

[[structure]]
op = "add_field"
path = "config.debug"
type = "bool"
default = false
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0").
		AssertUntouched("themes")
}

func TestEdge_EscapedDotsInKeys(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
`).
		Migration("1.0.0", `description = "Add key with literal dot"

[[structure]]
op = "add_field"
path = "api\\.v2"
type = "string"
default = "enabled"
down = { op = "remove_field", path = "api\\.v2" }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertHasKey("config", `api\.v2`).
		AssertVersion("config", "1.0.0")
}

func TestEdge_IrreversibleBlocksRollback(t *testing.T) {
	s := New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
`).
		Migration("1.0.0", `description = "Irreversible migration"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = "irreversible"
`)

	s.Migrate().
		AssertSuccess().
		AssertApplied(1)

	s.Rollback().
		AssertExitCode(4).
		AssertError("irreversible").
		AssertUntouched("config")
}

func TestEdge_FreshInstallFullHistory(t *testing.T) {
	// No input, 3 migrations building up config from scratch.
	New(t).
		SingleFile("config", "config.toml").
		Migration("1.0.0", `description = "v1: add title"

[[structure]]
op = "add_field"
path = "title"
type = "string"
default = "Untitled"
`).
		Migration("2.0.0", `description = "v2: add debug"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`).
		Migration("3.0.0", `description = "v3: add log_level"

[[structure]]
op = "add_field"
path = "log_level"
type = "string"
default = "info"
`).
		Migrate().
		AssertSuccess().
		AssertApplied(3).
		AssertFileExists("config").
		AssertKey("config", "title", "Untitled").
		AssertKey("config", "debug", false).
		AssertKey("config", "log_level", "info").
		AssertVersion("config", "3.0.0")
}

func TestEdge_CrossFileMoveSourceEmpty(t *testing.T) {
	// Move the only field from themes to config. Source still exists but
	// should be nearly empty (no _schema_version since config is the version file).
	New(t).
		MultiFile(map[string]string{
			"config": "config.toml",
			"themes": "themes.toml",
		}, "config").
		Input("config", `title = "App"
`).
		Input("themes", `color = "#ff0000"
`).
		Migration("1.0.0", `description = "Move color from themes to config"

[[structure]]
op = "move_field"
from = "themes.color"
to = "config.accent_color"
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertMissingKey("themes", "color").
		AssertKey("config", "accent_color", "#ff0000").
		AssertFileExists("themes").
		AssertVersion("config", "1.0.0")
}

func TestEdge_NegativeArrayIndex(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "0.0.0"

[[items]]
name = "first"
value = 1

[[items]]
name = "second"
value = 2

[[items]]
name = "third"
value = 3
`).
		Migration("1.0.0", `description = "Update last item"

[[data]]
op = "set_value_where"
path = "items"
match_mode = "index"
where = -1
set = { value = 999 }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0")
}

func TestEdge_ArrayDownOps(t *testing.T) {
	s := New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
`).
		Migration("1.0.0", `description = "Add UI fields"

[[structure]]
op = "add_field"
path = "ui_theme"
type = "string"
default = "dark"

[[structure]]
op = "add_field"
path = "ui_font_size"
type = "integer"
default = 14
down = [
  { op = "remove_field", path = "ui_font_size" },
  { op = "remove_field", path = "ui_theme" },
]
`)

	s.Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertHasKey("config", "ui_theme").
		AssertHasKey("config", "ui_font_size")

	s.Rollback().
		AssertSuccess().
		AssertApplied(1).
		AssertMissingKey("config", "ui_theme").
		AssertMissingKey("config", "ui_font_size").
		AssertVersion("config", "0.0.0")
}

func TestEdge_ConfigOnlySchemaVersion(t *testing.T) {
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "0.0.0"
`).
		Migration("1.0.0", `description = "Add fields to bare config"

[[structure]]
op = "add_field"
path = "title"
type = "string"
default = "My App"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertKey("config", "title", "My App").
		AssertKey("config", "debug", false).
		AssertVersion("config", "1.0.0")
}

func TestEdge_RollbackOnlyMigration(t *testing.T) {
	s := New(t).
		SingleFile("config", "config.toml").
		Input("config", `title = "My App"
`).
		Migration("1.0.0", `description = "Add debug"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = { op = "remove_field", path = "debug" }
`)

	// Migrate to v1.0.0.
	s.Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0")

	// Rollback to 0.0.0.
	s.Rollback().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "0.0.0").
		AssertMissingKey("config", "debug")

	// Re-migrate: should re-apply v1.0.0.
	s.Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0").
		AssertKey("config", "debug", false)
}

func TestEdge_NotHasKeyNoMatch(t *testing.T) {
	// Every item has the "role" key, so not_has_key on "role" matches nothing.
	New(t).
		SingleFile("config", "config.toml").
		Input("config", `_schema_version = "0.0.0"

[[users]]
name = "alice"
role = "admin"

[[users]]
name = "bob"
role = "user"
`).
		Migration("1.0.0", `description = "Set default role where missing"

[[data]]
op = "set_value_where"
path = "users"
match_mode = "not_has_key"
where = "role"
set = { role = "guest" }
`).
		Migrate().
		AssertSuccess().
		AssertApplied(1).
		AssertVersion("config", "1.0.0")
}
