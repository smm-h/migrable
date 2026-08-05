package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/smm-h/migrable/config"
	"github.com/smm-h/migrable/engine"
	"github.com/smm-h/strictcli/go/strictcli"
)

func runValidate(ctx *strictcli.Context, kwargs map[string]interface{}) strictcli.Outcome {
	configDir := kwargs["config_dir"].(string)
	quiet := ctx.Quiet()
	verbose := ctx.Verbose()

	cfg, err := config.Load(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	result, err := engine.Validate(migrationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	if quiet {
		if len(result.Errors) > 0 {
			return strictcli.Exit(ExitValidationError)
		}
		return strictcli.Exit(ExitSuccess)
	}

	if verbose {
		for _, e := range result.Errors {
			fmt.Printf("ERROR: %s: %s\n", e.File, e.Message)
		}
		for _, warn := range result.Warnings {
			fmt.Printf("WARN:  %s: %s\n", warn.File, warn.Message)
		}
		if len(result.Errors) > 0 || len(result.Warnings) > 0 {
			fmt.Println()
		}
	}

	fmt.Printf("Validated %d migration(s). %d error(s), %d warning(s).\n",
		result.FileCount, len(result.Errors), len(result.Warnings))

	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "validation failed with %d error(s)\n", len(result.Errors))
		return strictcli.Exit(ExitValidationError)
	}
	return strictcli.Exit(ExitSuccess)
}
