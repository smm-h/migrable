package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manCmd = &cobra.Command{
	Use:    "man",
	Short:  "Generate man page",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		header := &doc.GenManHeader{
			Title:   "MIGRABLE",
			Section: "1",
			Source:  "migrable " + version,
		}
		return doc.GenManTree(rootCmd, header, ".")
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}
