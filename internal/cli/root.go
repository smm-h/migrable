package cli

import (
	"github.com/smm-h/strictcli/go/strictcli"
)

// NewApp creates the migrable CLI application with all commands registered.
func NewApp(version string) *strictcli.App {
	app := strictcli.NewApp("migrable", version, "Declarative config file migrations for TOML")

	app.GlobalFlag(strictcli.StringFlag("config-dir", "path to configuration directory", strictcli.Default("")))
	// --quiet, --verbose and --dry-run are framework-owned (the reserved
	// quartet) and are read off the Context. Declaring any of them here is a
	// registration-time hard error.

	// mutating: writes migrable.toml and the migrations directory into the
	// target directory.
	app.Command("init", "Scaffold migrable.toml and migrations directory", runInit,
		strictcli.WithEffect(strictcli.EffectMutating))

	// mutating: rewrites the configured TOML files in place and advances their
	// recorded schema version (or, with --rollback, reverses the most recent
	// one). Not consequential -- the files are local, --dry-run previews the
	// exact diff, and --rollback is the built-in undo.
	app.Command("migrate", "Run pending migrations", runMigrate,
		strictcli.WithEffect(strictcli.EffectMutating),
		strictcli.WithFlags(
			strictcli.BoolFlag("rollback", "roll back the most recently applied migration", strictcli.Default(false)),
		),
	)

	// mutating: writes a new versioned migration file and removes the next/
	// staging files it consumed.
	app.Command("merge", "Combine next/ staging files into a versioned migration", runMerge,
		strictcli.WithEffect(strictcli.EffectMutating),
		strictcli.WithArgs(
			strictcli.NewArg("version", "target semver version for the migration"),
		),
	)

	// read_only: reads the config, the migrations directory and the recorded
	// schema version, and prints them.
	app.Command("status", "Show current version and pending migrations", runStatus,
		strictcli.WithEffect(strictcli.EffectReadOnly))

	// read_only: parses and checks the migration files, writing nothing.
	app.Command("validate", "Validate migration files", runValidate,
		strictcli.WithEffect(strictcli.EffectReadOnly))

	app.Deprecated("man", "Use 'migrable --help' for command documentation.")

	return app
}
