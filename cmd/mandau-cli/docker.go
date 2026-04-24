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
	Use:   "docker",
	Short: "Docker command wrapper",
	Long: `Run Docker commands on a remote agent. Use 'mandau docker ps' or 'mandau docker images'.

Available subcommands:
  ps, list      List containers
  images        List images
  stop          Stop one or more containers
  start         Start one or more containers
  restart       Restart one or more containers
  pause         Pause one or more containers
  unpause       Unpause one or more containers
  rm            Remove one or more containers
  kill          Kill one or more running containers
  logs          Fetch logs of a container
  inspect       Display detailed information on one or more containers/images
  exec          Execute a command in a running container
  stats         Display a live stream of container resource usage statistics
  version       Show Docker version information
  info          Display system-wide information
  network       Manage Docker networks
  volume        Manage Docker volumes

Examples:
  mandau docker ps
  mandau docker images
  mandau docker stop container1 container2
  mandau docker start container1
  mandau docker logs -f container1
  mandau docker exec -it container1 /bin/bash`,
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

	// List containers
	dockerCmd.AddCommand(&cobra.Command{
		Use:     "ps",
		Aliases: []string{"list"},
		Short:   "List containers",
		RunE:    listContainers,
	})

	// Stop containers
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "stop [CONTAINER...]",
		Short: "Stop one or more running containers",
		Long: `Stop one or more running containers on the remote agent.

Examples:
  mandau docker stop mycontainer
  mandau docker stop container1 container2
  mandau docker stop -t 30 mycontainer`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"stop"}, args...))
		},
	})

	// Start containers
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "start [CONTAINER...]",
		Short: "Start one or more stopped containers",
		Long: `Start one or more stopped containers on the remote agent.

Examples:
  mandau docker start mycontainer
  mandau docker start container1 container2`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"start"}, args...))
		},
	})

	// Restart containers
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "restart [CONTAINER...]",
		Short: "Restart one or more containers",
		Long: `Restart one or more containers on the remote agent.

Examples:
  mandau docker restart mycontainer
  mandau docker restart container1 container2
  mandau docker restart -t 10 mycontainer`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"restart"}, args...))
		},
	})

	// Pause containers
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "pause [CONTAINER...]",
		Short: "Pause all processes within one or more containers",
		Long: `Pause all processes within one or more containers on the remote agent.

Examples:
  mandau docker pause mycontainer
  mandau docker pause container1 container2`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"pause"}, args...))
		},
	})

	// Unpause containers
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "unpause [CONTAINER...]",
		Short: "Unpause all processes within one or more containers",
		Long: `Unpause all processes within one or more containers on the remote agent.

Examples:
  mandau docker unpause mycontainer
  mandau docker unpause container1 container2`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"unpause"}, args...))
		},
	})

	// Remove containers
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "rm [CONTAINER...]",
		Short: "Remove one or more containers",
		Long: `Remove one or more containers from the remote agent.

Examples:
  mandau docker rm mycontainer
  mandau docker rm container1 container2
  mandau docker rm -f mycontainer
  mandau docker rm -v mycontainer`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"rm"}, args...))
		},
	})

	// Kill containers
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "kill [CONTAINER...]",
		Short: "Kill one or more running containers",
		Long: `Kill one or more running containers on the remote agent.

Examples:
  mandau docker kill mycontainer
  mandau docker kill container1 container2
  mandau docker kill --signal=SIGTERM mycontainer`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"kill"}, args...))
		},
	})

	// Logs
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "logs [CONTAINER]",
		Short: "Fetch the logs of a container",
		Long: `Fetch the logs of a container from the remote agent.

Examples:
  mandau docker logs mycontainer
  mandau docker logs -f mycontainer
  mandau docker logs --tail 100 mycontainer
  mandau docker logs --since 2024-01-01 mycontainer`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"logs"}, args...))
		},
	})

	// Inspect
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "inspect [CONTAINER|IMAGE|NETWORK|VOLUME...]",
		Short: "Return low-level information on Docker objects",
		Long: `Display detailed information on one or more containers, images, networks, or volumes.

Examples:
  mandau docker inspect mycontainer
  mandau docker inspect myimage
  mandau docker inspect --format='{{.State.Status}}' mycontainer`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"inspect"}, args...))
		},
	})

	// Exec
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "exec [CONTAINER] [COMMAND]",
		Short: "Run a command in a running container",
		Long: `Execute a command in a running container on the remote agent.

Examples:
  mandau docker exec mycontainer ls
  mandau docker exec -it mycontainer /bin/bash
  mandau docker exec -e VAR=value mycontainer env`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"exec"}, args...))
		},
	})

	// Stats
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "stats [CONTAINER...]",
		Short: "Display a live stream of container(s) resource usage statistics",
		Long: `Display a live stream of container(s) resource usage statistics.

Examples:
  mandau docker stats
  mandau docker stats mycontainer
  mandau docker stats --no-stream`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.executeDockerCommand(cmd, append([]string{"stats"}, args...))
		},
	})

	// Images
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "images",
		Short: "List images",
		Long: `List Docker images on the remote agent.

Examples:
  mandau docker images
  mandau docker images -a
  mandau docker images --filter "dangling=true"`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.executeDockerCommand(cmd, append([]string{"images"}, args...))
		},
	})

	// Version
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show the Docker version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.executeDockerCommand(cmd, []string{"version"})
		},
	})

	// Info
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Display system-wide information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.executeDockerCommand(cmd, []string{"info"})
		},
	})

	// Network management
	networkCmd := &cobra.Command{
		Use:   "network",
		Short: "Manage Docker networks",
		Long: `Manage Docker networks on the remote agent.

Examples:
  mandau docker network ls
  mandau docker network create mynetwork
  mandau docker network inspect mynetwork
  mandau docker network rm mynetwork`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"network"}, args...))
		},
	}
	networkCmd.AddCommand(&cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.executeDockerCommand(cmd, append([]string{"network", "ls"}, args...))
		},
	})
	networkCmd.AddCommand(&cobra.Command{
		Use:   "create [NETWORK]",
		Short: "Create a network",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"network", "create"}, args...))
		},
	})
	networkCmd.AddCommand(&cobra.Command{
		Use:   "inspect [NETWORK...]",
		Short: "Display detailed information on one or more networks",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"network", "inspect"}, args...))
		},
	})
	networkCmd.AddCommand(&cobra.Command{
		Use:   "rm [NETWORK...]",
		Short: "Remove one or more networks",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"network", "rm"}, args...))
		},
	})
	dockerCmd.AddCommand(networkCmd)

	// Volume management
	volumeCmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage Docker volumes",
		Long: `Manage Docker volumes on the remote agent.

Examples:
  mandau docker volume ls
  mandau docker volume create myvolume
  mandau docker volume inspect myvolume
  mandau docker volume rm myvolume`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"volume"}, args...))
		},
	}
	volumeCmd.AddCommand(&cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.executeDockerCommand(cmd, append([]string{"volume", "ls"}, args...))
		},
	})
	volumeCmd.AddCommand(&cobra.Command{
		Use:   "create [VOLUME]",
		Short: "Create a volume",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"volume", "create"}, args...))
		},
	})
	volumeCmd.AddCommand(&cobra.Command{
		Use:   "inspect [VOLUME...]",
		Short: "Display detailed information on one or more volumes",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"volume", "inspect"}, args...))
		},
	})
	volumeCmd.AddCommand(&cobra.Command{
		Use:   "rm [VOLUME...]",
		Short: "Remove one or more volumes",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"volume", "rm"}, args...))
		},
	})
	dockerCmd.AddCommand(volumeCmd)

	// Pull image
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "pull [IMAGE]",
		Short: "Pull an image or a repository from a registry",
		Long: `Pull an image or repository from a registry.

Examples:
  mandau docker pull nginx
  mandau docker pull nginx:latest
  mandau docker pull myregistry.com/myimage:v1.0`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"pull"}, args...))
		},
	})

	// Push image
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "push [IMAGE]",
		Short: "Push an image or a repository to a registry",
		Long: `Push an image or repository to a registry.

Examples:
  mandau docker push myimage
  mandau docker push myregistry.com/myimage:v1.0`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"push"}, args...))
		},
	})

	// Build image
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "build [PATH]",
		Short: "Build an image from a Dockerfile",
		Long: `Build an image from a Dockerfile.

Examples:
  mandau docker build .
  mandau docker build -t myimage:v1.0 .
  mandau docker build -f Dockerfile.prod .`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"build"}, args...))
		},
	})

	// Prune
	dockerCmd.AddCommand(&cobra.Command{
		Use:   "prune",
		Short: "Remove unused data (containers, images, networks, volumes)",
		Long: `Remove unused Docker data.

Examples:
  mandau docker system prune
  mandau docker container prune
  mandau docker image prune`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cli.executeDockerCommand(cmd, append([]string{"prune"}, args...))
		},
	})
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
