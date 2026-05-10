package cli

import (
	"fmt"

	"github.com/smm-h/migrable/engine"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold migrable.toml and migrations directory",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	dir := "."
	if ConfigDir != "" {
		dir = ConfigDir
	}

	if err := engine.Init(dir); err != nil {
		return NewExitError(ExitGeneralError, "%v", err)
	}

	if Quiet {
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Initialized migrable project. Edit migrable.toml to configure target files.")
	return nil
}
