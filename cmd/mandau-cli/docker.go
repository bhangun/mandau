package main

import (
	"context"
	"fmt"
	"io"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Docker command wrapper",
	Long:  "Run Docker commands on a remote agent. Use 'mandau docker ps' or 'mandau docker images'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// Generic wrapper for all unknown docker commands
		return cli.executeDockerCommand(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(dockerCmd)

	// Specific subcommands that we might want to handle differently or keep from previous version
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List containers (alias for ps)",
		RunE:  listContainers,
	})

	dockerCmd.AddCommand(&cobra.Command{
		Use:   "ps",
		Short: "List containers",
		RunE:  listContainers,
	})
	
	// Add other common ones for help/visibility, but they will all use the generic wrapper
	// if not explicitly overridden.
}

func (c *CLI) executeDockerCommand(cmd *cobra.Command, args []string) error {
	agentID, err := c.resolveAgent(cmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	stream, err := c.coreClient.ExecuteDockerCommand(ctx, &v1.DockerCommandRequest{
		AgentId: agentID,
		Args:    args,
	})
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if len(resp.Output) > 0 {
			fmt.Print(string(resp.Output))
		}

		if resp.Error != "" {
			return fmt.Errorf("agent error: %s", resp.Error)
		}

		if resp.ExitCode != 0 {
			// This might be the final message, but let's keep looping until EOF just in case
			// Actually, in our implementation we send the exit code at the end.
		}
	}

	return nil
}

func (c *CLI) listContainers(cmd *cobra.Command, args []string) error {
	agentID, err := c.resolveAgent(cmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	resp, err := c.coreClient.ListContainers(ctx, &v1.ListContainersRequest{
		AgentId: agentID,
	})
	if err != nil {
		return err
	}

	if len(resp.Containers) == 0 {
		fmt.Printf("No containers found on agent %s\n", agentID)
		return nil
	}

	fmt.Printf("%-15s %-30s %-30s %-10s %-20s\n", "ID", "NAME", "IMAGE", "STATE", "STATUS")
	for _, container := range resp.Containers {
		image := container.Image
		if len(image) > 27 {
			image = image[:27] + "..."
		}

		fmt.Printf("%-15s %-30s %-30s %-10s %-20s\n",
			container.Id,
			container.Name,
			image,
			container.State,
			container.Status,
		)
	}

	return nil
}

func listContainers(cmd *cobra.Command, args []string) error {
	return cli.listContainers(cmd, args)
}