package main

import (
	"os"

	"ngxsetup/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args))
}
