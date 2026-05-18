package cli

import (
	"github.com/smm-h/strictcli/go/strictcli"
)

// NewApp creates the migrable CLI application with all commands registered.
func NewApp(version string) *strictcli.App {
	app := strictcli.NewApp("migrable", version, "Declarative config file migrations for TOML")

	app.GlobalFlag(strictcli.StringFlag("config-dir", "path to configuration directory", strictcli.Default("")))
	app.GlobalFlag(strictcli.BoolFlag("quiet", "suppress non-error output"))
	app.GlobalFlag(strictcli.BoolFlag("verbose", "enable verbose output"))

	app.Command("init", "Scaffold migrable.toml and migrations directory", runInit)

	app.Command("migrate", "Run pending migrations", runMigrate,
		strictcli.WithFlags(
			strictcli.BoolFlag("dry-run", "preview changes without writing"),
			strictcli.BoolFlag("rollback", "roll back the most recently applied migration"),
		),
	)

	app.Command("merge", "Combine next/ staging files into a versioned migration", runMerge,
		strictcli.WithArgs(
			strictcli.NewArg("version", "target semver version for the migration"),
		),
	)

	app.Command("status", "Show current version and pending migrations", runStatus)

	app.Command("validate", "Validate migration files", runValidate)

	app.Deprecated("man", "Use 'migrable --help' for command documentation.")

	return app
}
