package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(containerCmd)

	// Container commands
	containerCmd.AddCommand(&cobra.Command{
		Use:   "exec [agent] [container] [command] [args...]",
		Short: "Execute command in container",
		Long:  "Execute a command in a running container on the specified agent",
		Args:  cobra.MinimumNArgs(3),
		RunE:  execContainer,
	})

	containerCmd.AddCommand(&cobra.Command{
		Use:   "list [agent]",
		Short: "List containers",
		Long:  "List all containers on the specified agent",
		Args:  cobra.ExactArgs(1),
		RunE:  listContainers,
	})

	containerCmd.AddCommand(&cobra.Command{
		Use:   "logs [agent] [container]",
		Short: "Get container logs",
		Long:  "Get logs from a container on the specified agent",
		Args:  cobra.ExactArgs(2),
		RunE:  getContainerLogs,
	})

	containerCmd.AddCommand(&cobra.Command{
		Use:   "start [agent] [container]",
		Short: "Start container",
		Long:  "Start a container on the specified agent",
		Args:  cobra.ExactArgs(2),
		RunE:  startContainer,
	})

	containerCmd.AddCommand(&cobra.Command{
		Use:   "stop [agent] [container]",
		Short: "Stop container",
		Long:  "Stop a container on the specified agent",
		Args:  cobra.ExactArgs(2),
		RunE:  stopContainer,
	})
}

var containerCmd = &cobra.Command{
	Use:   "container",
	Short: "Manage containers",
	Long:  "Commands to manage containers on agents",
}

func (c *CLI) execContainer(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	containerID := args[1]
	command := args[2:]

	fmt.Printf("Executing command in container %s on agent %s: %v\n", containerID, agentID, command)
	fmt.Println("Note: This would call the container exec functionality in the actual implementation")
	return nil
}

func execContainer(cmd *cobra.Command, args []string) error {
	return cli.execContainer(cmd, args)
}

func (c *CLI) listContainers(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	fmt.Printf("Listing containers on agent %s\n", agentID)
	fmt.Println("Note: This would call the container list functionality in the actual implementation")
	return nil
}

func listContainers(cmd *cobra.Command, args []string) error {
	return cli.listContainers(cmd, args)
}

func (c *CLI) getContainerLogs(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	containerID := args[1]
	fmt.Printf("Getting logs for container %s on agent %s\n", containerID, agentID)
	fmt.Println("Note: This would call the container logs functionality in the actual implementation")
	return nil
}

func getContainerLogs(cmd *cobra.Command, args []string) error {
	return cli.getContainerLogs(cmd, args)
}

func (c *CLI) startContainer(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	containerID := args[1]
	fmt.Printf("Starting container %s on agent %s\n", containerID, agentID)
	fmt.Println("Note: This would call the container start functionality in the actual implementation")
	return nil
}

func startContainer(cmd *cobra.Command, args []string) error {
	return cli.startContainer(cmd, args)
}

func (c *CLI) stopContainer(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	containerID := args[1]
	fmt.Printf("Stopping container %s on agent %s\n", containerID, agentID)
	fmt.Println("Note: This would call the container stop functionality in the actual implementation")
	return nil
}

func stopContainer(cmd *cobra.Command, args []string) error {
	return cli.stopContainer(cmd, args)
}