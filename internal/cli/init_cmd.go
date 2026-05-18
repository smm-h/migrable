package cli

import (
	"fmt"
	"os"

	"github.com/smm-h/migrable/engine"
)

func runInit(kwargs map[string]interface{}) int {
	configDir := kwargs["config_dir"].(string)
	quiet := kwargs["quiet"].(bool)

	dir := "."
	if configDir != "" {
		dir = configDir
	}

	if err := engine.Init(dir); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return ExitGeneralError
	}

	if quiet {
		return ExitSuccess
	}

	fmt.Println("Initialized migrable project. Edit migrable.toml to configure target files.")
	return ExitSuccess
}
