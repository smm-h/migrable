package cli

import (
	"fmt"
	"path/filepath"

	"github.com/smm-h/migrable/internal/config"
	"github.com/smm-h/migrable/internal/engine"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current version and pending migrations",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(ConfigDir)
	if err != nil {
		return err
	}

	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	migrations, err := engine.DiscoverMigrations(migrationsDir)
	if err != nil {
		return err
	}

	versionFile := ""
	if cfg.VersionFile != "" {
		versionFile = cfg.VersionFile
	} else {
		for k := range cfg.Files {
			versionFile = k
			break
		}
	}
	versionFilePath := cfg.Files[versionFile]
	if !filepath.IsAbs(versionFilePath) {
		versionFilePath = filepath.Join(cfg.BaseDir, versionFilePath)
	}

	currentVersion, err := engine.ReadSchemaVersion(versionFilePath)
	if err != nil {
		return err
	}

	var pending []engine.MigrationMeta
	for _, m := range migrations {
		if m.Version.GreaterThan(currentVersion) {
			pending = append(pending, m)
		}
	}

	if Quiet {
		if len(pending) > 0 {
			return NewExitError(ExitGeneralError, "")
		}
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", currentVersion)
	fmt.Fprintf(cmd.OutOrStdout(), "Pending migrations: %d\n", len(pending))

	if len(pending) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "")
		for _, m := range pending {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", m.Version)
		}
	}

	if Verbose {
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintf(cmd.OutOrStdout(), "All migrations (%d):\n", len(migrations))
		for _, m := range migrations {
			marker := " "
			if m.Version.GreaterThan(currentVersion) {
				marker = "*"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", marker, m.Version)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "Files:")
		for key, path := range cfg.Files {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s = %s\n", key, path)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nVersion file: %s\n", versionFile)
		fmt.Fprintf(cmd.OutOrStdout(), "Migrations dir: %s\n", migrationsDir)
	}

	return nil
}
