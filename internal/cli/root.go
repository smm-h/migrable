package cli

import (
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var version = "dev"

var (
	ConfigDir string
	Quiet     bool
	Verbose   bool
)

var rootCmd = &cobra.Command{
	Use:   "migrable",
	Short: "Declarative config file migrations for TOML",
}

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = strings.TrimPrefix(info.Main.Version, "v")
		}
	}
	rootCmd.Version = version
	rootCmd.PersistentFlags().StringVar(&ConfigDir, "config-dir", "", "path to configuration directory")
	rootCmd.PersistentFlags().BoolVar(&Quiet, "quiet", false, "suppress non-error output")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "enable verbose output")
}

func Execute() error {
	return rootCmd.Execute()
}
