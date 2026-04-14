package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:                "docker",
	Short:              "Docker command wrapper",
	Long:               "Run Docker commands on a remote agent. Use 'mandau docker ps' or 'mandau docker images'.",
	DisableFlagParsing: true,
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

	// ANSI color codes
	const (
		reset   = "\033[0m"
		bold    = "\033[1m"
		dim     = "\033[2m"
		green   = "\033[32m"
		red     = "\033[31m"
		yellow  = "\033[33m"
		cyan    = "\033[36m"
		magenta = "\033[35m"
		white   = "\033[37m"
	)

	// Header
	fmt.Printf("%s%-14s  %-28s  %-30s  %-12s  %-30s  %s%s\n",
		bold, "CONTAINER ID", "NAME", "IMAGE", "STATE", "STATUS", "PORTS", reset)

	for _, container := range resp.Containers {
		// Truncate long values
		name := container.Name
		if len(name) > 0 && name[0] == '/' {
			name = name[1:] // Strip leading /
		}
		if len(name) > 28 {
			name = name[:25] + "..."
		}

		image := container.Image
		if len(image) > 30 {
			image = image[:27] + "..."
		}

		// Format ports
		ports := formatPorts(container.Ports)
		if len(ports) > 40 {
			ports = ports[:37] + "..."
		}

		// Colorize state
		var stateColor string
		switch container.State {
		case "running":
			stateColor = green
		case "exited":
			stateColor = red
		case "restarting":
			stateColor = yellow
		case "paused":
			stateColor = magenta
		case "created":
			stateColor = cyan
		default:
			stateColor = dim
		}

		// Colorize status
		statusColor := dim
		status := container.Status
		if strings.Contains(status, "healthy") {
			statusColor = green
		} else if strings.Contains(status, "unhealthy") {
			statusColor = red
		} else if strings.Contains(status, "Exited") {
			statusColor = red
		} else if strings.Contains(status, "Restarting") {
			statusColor = yellow
		} else if strings.Contains(status, "Up") {
			statusColor = green
		}

		fmt.Printf("%s%-14s%s  %s%-28s%s  %s%-30s%s  %s%-12s%s  %s%-30s%s  %s%s%s\n",
			cyan, container.Id, reset,
			white, name, reset,
			dim, image, reset,
			stateColor, container.State, reset,
			statusColor, status, reset,
			dim, ports, reset,
		)
	}

	fmt.Printf("\n%s%d container(s) on agent %s%s\n", dim, len(resp.Containers), agentID, reset)

	return nil
}

func formatPorts(ports []*v1.Port) string {
	if len(ports) == 0 {
		return ""
	}

	var parts []string
	for _, p := range ports {
		if p.PublicPort > 0 {
			ip := p.Ip
			if ip == "" || ip == "0.0.0.0" {
				ip = "0.0.0.0"
			}
			parts = append(parts, fmt.Sprintf("%s:%d->%d/%s", ip, p.PublicPort, p.PrivatePort, p.Type))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
		}
	}
	return strings.Join(parts, ", ")
}

func listContainers(cmd *cobra.Command, args []string) error {
	return cli.listContainers(cmd, args)
}
