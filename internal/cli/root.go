package cli

import "github.com/spf13/cobra"

var version string

var (
	ConfigDir string
	Quiet     bool
	Verbose   bool
)

var rootCmd = &cobra.Command{
	Use:   "migrable",
	Short: "Declarative config file migrations for TOML",
}

func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

func init() {
	rootCmd.PersistentFlags().StringVar(&ConfigDir, "config-dir", "", "path to configuration directory")
	rootCmd.PersistentFlags().BoolVar(&Quiet, "quiet", false, "suppress non-error output")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "enable verbose output")
}

func Execute() error {
	return rootCmd.Execute()
}
