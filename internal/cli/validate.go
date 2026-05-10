package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate migration files",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented")
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
