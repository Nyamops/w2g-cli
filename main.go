package main

import (
	"os"

	"w2g-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
