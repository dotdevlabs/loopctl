package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "loopctl",
	Short: "loopctl — CLI for managing LoopControl",
	Long:  `loopctl is a command-line interface for interacting with the LoopControl platform.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
