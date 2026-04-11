package main

import (
	"github.com/spf13/cobra"
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Docker Compose command wrapper",
	Long:  "Run Docker Compose commands on a remote agent. Use 'mandau compose ps', 'mandau compose up -d', etc.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// Prepend 'compose' to the arguments and forward to the docker execution logic
		composeArgs := append([]string{"compose"}, args...)
		return cli.executeDockerCommand(cmd, composeArgs)
	},
}

func init() {
	rootCmd.AddCommand(composeCmd)
}
