package cli

import (
	"fmt"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/smm-h/migrable/config"
	"github.com/smm-h/migrable/engine"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge <version>",
	Short: "Combine next/ staging files into a versioned migration",
	Args:  cobra.ExactArgs(1),
	RunE:  runMerge,
}

func init() {
	rootCmd.AddCommand(mergeCmd)
}

func runMerge(cmd *cobra.Command, args []string) error {
	version := args[0]

	// Validate semver.
	if _, err := semver.StrictNewVersion(version); err != nil {
		return NewExitError(ExitValidationError, "invalid version %q: %v", version, err)
	}

	cfg, err := config.Load(ConfigDir)
	if err != nil {
		return NewExitError(ExitGeneralError, "%v", err)
	}

	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	outPath, err := engine.Merge(migrationsDir, version)
	if err != nil {
		return NewExitError(ExitGeneralError, "%v", err)
	}

	if outPath == "" {
		if !Quiet {
			fmt.Fprintln(cmd.ErrOrStderr(), "Warning: no staging files in next/, nothing to merge")
		}
		return nil
	}

	if Quiet {
		return nil
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Merged staging files into %s\n", outPath)
	return nil
}
