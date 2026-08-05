package cli

import (
	"fmt"
	"os"

	"github.com/smm-h/migrable/engine"
	"github.com/smm-h/strictcli/go/strictcli"
)

func runInit(ctx *strictcli.Context, kwargs map[string]interface{}) strictcli.Outcome {
	configDir := kwargs["config_dir"].(string)
	quiet := ctx.Quiet()

	dir := "."
	if configDir != "" {
		dir = configDir
	}

	if err := engine.Init(dir); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	if quiet {
		return strictcli.Exit(ExitSuccess)
	}

	fmt.Println("Initialized migrable project. Edit migrable.toml to configure target files.")
	return strictcli.Exit(ExitSuccess)
}
