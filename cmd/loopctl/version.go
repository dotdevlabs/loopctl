package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		if jsonFlag {
			out, _ := json.Marshal(map[string]string{"version": version})
			fmt.Println(string(out))
			return
		}
		fmt.Printf("loopctl %s\n", version)
	},
}

func init() {
	versionCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(versionCmd)
}
