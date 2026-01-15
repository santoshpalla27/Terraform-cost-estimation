// Package main is the entry point for terraform-cost CLI.
package main

import (
	"os"

	"terraform-cost/cmd/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
