package cli

import (
	"errors"
	"fmt"
	"os"

	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/config"
	"github.com/smm-h/migrable/engine"
	"github.com/smm-h/strictcli/go/strictcli"
)

func runMigrate(ctx *strictcli.Context, kwargs map[string]interface{}) strictcli.Outcome {
	configDir := kwargs["config_dir"].(string)
	quiet := ctx.Quiet()
	verbose := ctx.Verbose()
	dryRun := ctx.DryRun()
	rollback := kwargs["rollback"].(bool)

	cfg, err := config.Load(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	if rollback {
		return doRollback(cfg, quiet, verbose, dryRun)
	}

	result, err := engine.Migrate(cfg, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitMigrationError)
	}

	if quiet {
		return strictcli.Exit(ExitSuccess)
	}

	if result.Applied == 0 {
		fmt.Printf("Already up to date (version %s)\n", result.FromVersion)
		return strictcli.Exit(ExitSuccess)
	}

	if dryRun {
		fmt.Printf("Dry run: %d migration(s) would be applied (%s -> %s)\n",
			result.Applied, result.FromVersion, result.ToVersion)
		printDryRunChanges(result)
		return strictcli.Exit(ExitSuccess)
	}

	fmt.Printf("Applied %d migration(s). Version: %s -> %s\n",
		result.Applied, result.FromVersion, result.ToVersion)

	if verbose {
		printVerboseMigrations(result)
	}

	return strictcli.Exit(ExitSuccess)
}

func doRollback(cfg *config.Config, quiet, verbose, dryRun bool) strictcli.Outcome {
	result, err := engine.Rollback(cfg, dryRun)
	if err != nil {
		var blocked *engine.RollbackBlockedError
		if errors.As(err, &blocked) {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return strictcli.Exit(ExitRollbackBlocked)
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitMigrationError)
	}

	if quiet {
		return strictcli.Exit(ExitSuccess)
	}

	if dryRun {
		fmt.Printf("Dry run: would roll back migration %s. Version: %s -> %s\n",
			result.FromVersion, result.FromVersion, result.ToVersion)
		printDryRunChanges(result)
		return strictcli.Exit(ExitSuccess)
	}

	fmt.Printf("Rolled back migration %s. Version: %s -> %s\n",
		result.FromVersion, result.FromVersion, result.ToVersion)

	if verbose {
		for ver, desc := range result.Descriptions {
			if desc != "" {
				fmt.Printf("  %s: %s\n", ver, desc)
			}
		}
	}

	return strictcli.Exit(ExitSuccess)
}

func printDryRunChanges(result *engine.MigrateResult) {
	for fileKey, changes := range result.FileChanges {
		if len(changes) == 0 {
			continue
		}
		fmt.Printf("\nFile: %s\n", fileKey)
		for _, c := range changes {
			printChange(c)
		}
	}
}

func printChange(c tomledit.Change) {
	switch c.Kind {
	case tomledit.Added:
		fmt.Printf("  + %s = %s\n", c.Path, formatValue(c.NewValue))
	case tomledit.Removed:
		fmt.Printf("  - %s = %s\n", c.Path, formatValue(c.OldValue))
	case tomledit.Modified:
		fmt.Printf("  ~ %s: %s -> %s\n", c.Path, formatValue(c.OldValue), formatValue(c.NewValue))
	}
}

func printVerboseMigrations(result *engine.MigrateResult) {
	for ver, desc := range result.Descriptions {
		if desc != "" {
			fmt.Printf("  %s: %s\n", ver, desc)
		} else {
			fmt.Printf("  %s\n", ver)
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
