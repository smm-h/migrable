package cli

import (
	"fmt"

	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/internal/config"
	"github.com/smm-h/migrable/internal/engine"
	"github.com/spf13/cobra"
)

var dryRun bool

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run pending migrations",
	RunE:  runMigrate,
}

func init() {
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without writing")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(ConfigDir)
	if err != nil {
		return NewExitError(ExitGeneralError, "%v", err)
	}

	result, err := engine.Migrate(cfg, dryRun)
	if err != nil {
		return NewExitError(ExitMigrationError, "%v", err)
	}

	if Quiet {
		return nil
	}

	w := cmd.OutOrStdout()

	if result.Applied == 0 {
		fmt.Fprintf(w, "Already up to date (version %s)\n", result.FromVersion)
		return nil
	}

	if dryRun {
		fmt.Fprintf(w, "Dry run: %d migration(s) would be applied (%s -> %s)\n",
			result.Applied, result.FromVersion, result.ToVersion)
		printDryRunChanges(cmd, result)
		return nil
	}

	fmt.Fprintf(w, "Applied %d migration(s). Version: %s -> %s\n",
		result.Applied, result.FromVersion, result.ToVersion)

	if Verbose {
		printVerboseMigrations(cmd, result)
	}

	return nil
}

func printDryRunChanges(cmd *cobra.Command, result *engine.MigrateResult) {
	w := cmd.OutOrStdout()
	for fileKey, changes := range result.FileChanges {
		if len(changes) == 0 {
			continue
		}
		fmt.Fprintf(w, "\nFile: %s\n", fileKey)
		for _, c := range changes {
			printChange(cmd, c)
		}
	}
}

func printChange(cmd *cobra.Command, c tomledit.Change) {
	w := cmd.OutOrStdout()
	switch c.Kind {
	case tomledit.Added:
		fmt.Fprintf(w, "  + %s = %s\n", c.Path, formatValue(c.NewValue))
	case tomledit.Removed:
		fmt.Fprintf(w, "  - %s = %s\n", c.Path, formatValue(c.OldValue))
	case tomledit.Modified:
		fmt.Fprintf(w, "  ~ %s: %s -> %s\n", c.Path, formatValue(c.OldValue), formatValue(c.NewValue))
	}
}

func printVerboseMigrations(cmd *cobra.Command, result *engine.MigrateResult) {
	w := cmd.OutOrStdout()
	for ver, desc := range result.Descriptions {
		if desc != "" {
			fmt.Fprintf(w, "  %s: %s\n", ver, desc)
		} else {
			fmt.Fprintf(w, "  %s\n", ver)
		}
	}
}

func formatValue(v any) string {
	if v == nil {
		return "(no value)"
	}
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
