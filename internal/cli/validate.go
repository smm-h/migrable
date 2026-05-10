package cli

import (
	"fmt"
	"path/filepath"

	"github.com/smm-h/migrable/config"
	"github.com/smm-h/migrable/engine"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate migration files",
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(ConfigDir)
	if err != nil {
		return NewExitError(ExitGeneralError, "%v", err)
	}

	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	result, err := engine.Validate(migrationsDir)
	if err != nil {
		return NewExitError(ExitGeneralError, "%v", err)
	}

	if Quiet {
		if len(result.Errors) > 0 {
			return NewExitError(ExitValidationError, "")
		}
		return nil
	}

	w := cmd.OutOrStdout()

	if Verbose {
		for _, e := range result.Errors {
			fmt.Fprintf(w, "ERROR: %s: %s\n", e.File, e.Message)
		}
		for _, warn := range result.Warnings {
			fmt.Fprintf(w, "WARN:  %s: %s\n", warn.File, warn.Message)
		}
		if len(result.Errors) > 0 || len(result.Warnings) > 0 {
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintf(w, "Validated %d migration(s). %d error(s), %d warning(s).\n",
		result.FileCount, len(result.Errors), len(result.Warnings))

	if len(result.Errors) > 0 {
		return NewExitError(ExitValidationError, "validation failed with %d error(s)", len(result.Errors))
	}
	return nil
}
