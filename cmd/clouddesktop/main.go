package main

import (
	"os"

	"github.com/nbugash-viafoura/clouddesktop/internal/cli"
)

func main() {
	rootCmd := cli.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
