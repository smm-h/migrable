package main

import (
	"github.com/smm-h/migrable/internal/cli"
)

func main() {
	cli.NewApp(Version).Run()
}
