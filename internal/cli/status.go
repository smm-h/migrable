package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current version and pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented")
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
