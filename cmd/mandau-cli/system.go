package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	// Enhanced shell command
	shellCmd.Long = `Opens an interactive secure shell directly on the agent's underlying operating system.
This requires the agent to have 'enable_host_shell' configured securely.

Examples:
  mandau shell
  mandau shell agent-001
  mandau shell  # auto-selects agent`

	// System monitoring commands
	systemCmd := &cobra.Command{
		Use:   "system",
		Short: "System monitoring commands",
		Long:  `System monitoring and information commands for remote agents.`,
	}

	// System info (comprehensive)
	systemCmd.AddCommand(&cobra.Command{
		Use:   "info [agent-id]",
		Short: "Display comprehensive system information",
		Long: `Display comprehensive system information including OS, CPU, memory, disk, and network.

Examples:
  mandau system info
  mandau system info agent-001`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, err := cli.resolveAgent(cmd)
			if err != nil {
				return err
			}
			return cli.runSystemInfo(agentID)
		},
	})

	// PS - Process list
	systemCmd.AddCommand(&cobra.Command{
		Use:   "ps [agent-id] [flags...]",
		Short: "List running processes",
		Long: `List running processes on the remote agent.

Examples:
  mandau system ps
  mandau system ps agent-001
  mandau system ps agent-001 aux
  mandau system ps agent-001 -ef`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			psFlags := "aux"
			if len(cmdArgs) > 0 {
				psFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runPs(agentID, psFlags)
		},
	})

	// DF - Disk usage
	systemCmd.AddCommand(&cobra.Command{
		Use:   "df [agent-id] [flags...]",
		Short: "Report file system disk space usage",
		Long: `Report file system disk space usage on the remote agent.

Examples:
  mandau system df
  mandau system df agent-001
  mandau system df agent-001 -h
  mandau system df agent-001 -hi`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			dfFlags := "-h"
			if len(cmdArgs) > 0 {
				dfFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runDf(agentID, dfFlags)
		},
	})

	// DU - Directory space usage
	systemCmd.AddCommand(&cobra.Command{
		Use:   "du [agent-id] [path] [flags...]",
		Short: "Estimate file space usage",
		Long: `Estimate file space usage on the remote agent.

Examples:
  mandau system du /var/log
  mandau system du agent-001 /home
  mandau system du agent-001 / -h --max-depth=1`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			path := "."
			if len(cmdArgs) > 0 {
				path = cmdArgs[0]
				cmdArgs = cmdArgs[1:]
			}
			duFlags := "-sh"
			if len(cmdArgs) > 0 {
				duFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runDu(agentID, path, duFlags)
		},
	})

	// Free - Memory usage
	systemCmd.AddCommand(&cobra.Command{
		Use:   "free [agent-id] [flags...]",
		Short: "Display amount of free and used memory",
		Long: `Display amount of free and used memory in the system.

Examples:
  mandau system free
  mandau system free agent-001
  mandau system free agent-001 -h
  mandau system free agent-001 -m`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			freeFlags := "-h"
			if len(cmdArgs) > 0 {
				freeFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runFree(agentID, freeFlags)
		},
	})

	// Uptime
	systemCmd.AddCommand(&cobra.Command{
		Use:   "uptime [agent-id]",
		Short: "Tell how long the system has been running",
		Long: `Tell how long the system has been running.

Examples:
  mandau system uptime
  mandau system uptime agent-001`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, _, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			return cli.runUptime(agentID)
		},
	})

	// Top - Process activity
	systemCmd.AddCommand(&cobra.Command{
		Use:   "top [agent-id]",
		Short: "Display Linux processes (interactive)",
		Long: `Display Linux processes in interactive mode.

Examples:
  mandau system top
  mandau system top agent-001`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, _, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			return cli.runTop(agentID)
		},
	})

	// Who - Logged in users
	systemCmd.AddCommand(&cobra.Command{
		Use:   "who [agent-id]",
		Short: "Show who is logged on",
		Long: `Show who is logged on the remote system.

Examples:
  mandau system who
  mandau system who agent-001`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, _, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			return cli.runWho(agentID)
		},
	})

	// Last - Recent logins
	systemCmd.AddCommand(&cobra.Command{
		Use:   "last [agent-id] [flags...]",
		Short: "Show listing of last logged in users",
		Long: `Show listing of last logged in users.

Examples:
  mandau system last
  mandau system last agent-001
  mandau system last agent-001 -10`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			lastFlags := "-10"
			if len(cmdArgs) > 0 {
				lastFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runLast(agentID, lastFlags)
		},
	})

	// Netstat - Network statistics
	systemCmd.AddCommand(&cobra.Command{
		Use:   "netstat [agent-id] [flags...]",
		Short: "Show network statistics",
		Long: `Show network statistics on the remote agent.

Examples:
  mandau system netstat
  mandau system netstat agent-001
  mandau system netstat agent-001 -tulpn`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			netstatFlags := "-tulpn"
			if len(cmdArgs) > 0 {
				netstatFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runNetstat(agentID, netstatFlags)
		},
	})

	// Htop - Interactive process viewer (if available)
	systemCmd.AddCommand(&cobra.Command{
		Use:   "htop [agent-id]",
		Short: "Interactive process viewer (if available)",
		Long: `Interactive process viewer on the remote agent.
Requires htop to be installed on the remote system.

Examples:
  mandau system htop
  mandau system htop agent-001`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, _, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			return cli.runHtop(agentID)
		},
	})

	// Tail logs
	systemCmd.AddCommand(&cobra.Command{
		Use:   "logs [agent-id] [logfile] [flags...]",
		Short: "Tail log files",
		Long: `Tail log files on the remote agent.

Examples:
  mandau system logs /var/log/syslog
  mandau system logs agent-001 /var/log/syslog
  mandau system logs agent-001 /var/log/syslog -f
  mandau system logs agent-001 /var/log/syslog -n 100`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			logFile := "/var/log/syslog"
			if len(cmdArgs) > 0 {
				logFile = cmdArgs[0]
				cmdArgs = cmdArgs[1:]
			}
			tailFlags := "-n 50"
			if len(cmdArgs) > 0 {
				tailFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runTailLogs(agentID, logFile, tailFlags)
		},
	})

	// Quick commands (shortcuts)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "ps [agent-id] [flags...]",
		Short: "Quick process list (alias for system ps)",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			psFlags := "aux"
			if len(cmdArgs) > 0 {
				psFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runPs(agentID, psFlags)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "df [agent-id] [flags...]",
		Short: "Quick disk usage (alias for system df)",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			dfFlags := "-h"
			if len(cmdArgs) > 0 {
				dfFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runDf(agentID, dfFlags)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "free [agent-id] [flags...]",
		Short: "Quick memory info (alias for system free)",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID, cmdArgs, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			freeFlags := "-h"
			if len(cmdArgs) > 0 {
				freeFlags = strings.Join(cmdArgs, " ")
			}
			return cli.runFree(agentID, freeFlags)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "uptime [agent-id]",
		Short: "Quick uptime check (alias for system uptime)",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID, _, err := cli.parseSystemArgs(cmd, args)
			if err != nil {
				return err
			}
			return cli.runUptime(agentID)
		},
	})

	rootCmd.AddCommand(systemCmd)
}

// parseSystemArgs extracts agent ID and command arguments
func (c *CLI) parseSystemArgs(cmd *cobra.Command, args []string) (string, []string, error) {
	var agentID string
	var cmdArgs []string

	if len(args) > 0 {
		// Check if first arg looks like an agent ID (not a flag)
		if !strings.HasPrefix(args[0], "-") {
			agentID = args[0]
			if len(args) > 1 {
				cmdArgs = args[1:]
			}
		} else {
			id, err := c.resolveAgent(cmd)
			if err != nil {
				return "", nil, err
			}
			agentID = id
			cmdArgs = args
		}
	} else {
		id, err := c.resolveAgent(cmd)
		if err != nil {
			return "", nil, err
		}
		agentID = id
	}

	return agentID, cmdArgs, nil
}

// executeHostCommand runs a one-off command on the remote host
func (c *CLI) executeHostCommand(agentID string, command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stream, err := c.coreClient.HostShell(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to connect shell stream: %w", err)
	}

	// Send initial request
	err = stream.Send(&v1.HostShellRequest{
		AgentId: agentID,
		Data:    []byte(command + "\nexit\n"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	// Collect output
	var output bytes.Buffer
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return output.String(), fmt.Errorf("stream error: %w", err)
		}
		if resp.Error != "" {
			return output.String(), fmt.Errorf("remote error: %s", resp.Error)
		}
		if len(resp.Data) > 0 {
			output.Write(resp.Data)
		}
	}

	return output.String(), nil
}

// executeInteractiveCommand runs an interactive command
func (c *CLI) executeInteractiveCommand(agentID string, command string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := c.coreClient.HostShell(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect shell stream: %w", err)
	}

	// Initial connection
	err = stream.Send(&v1.HostShellRequest{
		AgentId: agentID,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize shell: %w", err)
	}

	// Put local terminal in raw mode
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	// Get terminal size
	width, height, err := term.GetSize(fd)
	if err == nil {
		stream.Send(&v1.HostShellRequest{
			TermWidth:  int32(width),
			TermHeight: int32(height),
		})
	}

	errChan := make(chan error, 1)

	// Send command
	go func() {
		// Wait a bit for shell to initialize
		time.Sleep(100 * time.Millisecond)
		stream.Send(&v1.HostShellRequest{
			Data: []byte(command + "\n"),
		})
	}()

	// Read from remote
	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					errChan <- nil
					return
				}
				errChan <- err
				return
			}
			if resp.Error != "" {
				errChan <- fmt.Errorf("remote error: %s", resp.Error)
				return
			}
			if len(resp.Data) > 0 {
				os.Stdout.Write(resp.Data)
			}
		}
	}()

	// Read from local
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if sendErr := stream.Send(&v1.HostShellRequest{
					Data: buf[:n],
				}); sendErr != nil {
					errChan <- sendErr
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					errChan <- err
				}
				_ = stream.CloseSend()
				return
			}
		}
	}()

	// Wait for completion
	if err := <-errChan; err != nil && err != io.EOF {
		term.Restore(fd, oldState)
		return err
	}

	return nil
}

// System command implementations

func (c *CLI) runSystemInfo(agentID string) error {
	fmt.Printf("📊 System Information for agent: %s\n\n", agentID)

	// Hostname and OS
	fmt.Println("🖥️  System Overview:")
	output, err := c.executeHostCommand(agentID, "echo \"Hostname: $(hostname)\" && echo \"OS: $(cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d'\"' -f2 || uname -s)\" && echo \"Kernel: $(uname -r)\" && echo \"Architecture: $(uname -m)\"", 5*time.Second)
	if err != nil {
		return err
	}
	fmt.Println(output)

	// CPU
	fmt.Println("💻 CPU Information:")
	output, err = c.executeHostCommand(agentID, "echo \"CPU: $(grep 'model name' /proc/cpuinfo 2>/dev/null | head -1 | cut -d':' -f2 | xargs || echo 'N/A')\" && echo \"Cores: $(nproc 2>/dev/null || grep -c processor /proc/cpuinfo 2>/dev/null || echo 'N/A')\"", 5*time.Second)
	if err != nil {
		return err
	}
	fmt.Println(output)

	// Memory
	fmt.Println("🧠 Memory Usage:")
	output, err = c.executeHostCommand(agentID, "free -h 2>/dev/null || cat /proc/meminfo | head -5", 5*time.Second)
	if err != nil {
		return err
	}
	fmt.Println(output)

	// Disk
	fmt.Println("💾 Disk Usage:")
	output, err = c.executeHostCommand(agentID, "df -h 2>/dev/null | grep -E '^(Filesystem|/dev/)'", 5*time.Second)
	if err != nil {
		return err
	}
	fmt.Println(output)

	// Uptime
	fmt.Println("⏱️  Uptime:")
	output, err = c.executeHostCommand(agentID, "uptime", 5*time.Second)
	if err != nil {
		return err
	}
	fmt.Println(output)

	// Network
	fmt.Println("🌐 Network Interfaces:")
	output, err = c.executeHostCommand(agentID, "ip -brief addr show 2>/dev/null || ifconfig | grep 'inet ' | head -5", 5*time.Second)
	if err != nil {
		return err
	}
	fmt.Println(output)

	return nil
}

func (c *CLI) runPs(agentID, flags string) error {
	command := fmt.Sprintf("ps %s", flags)
	output, err := c.executeHostCommand(agentID, command, 10*time.Second)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func (c *CLI) runDf(agentID, flags string) error {
	command := fmt.Sprintf("df %s", flags)
	output, err := c.executeHostCommand(agentID, command, 10*time.Second)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func (c *CLI) runDu(agentID, path, flags string) error {
	command := fmt.Sprintf("du %s %s", flags, path)
	output, err := c.executeHostCommand(agentID, command, 30*time.Second)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func (c *CLI) runFree(agentID, flags string) error {
	command := fmt.Sprintf("free %s", flags)
	output, err := c.executeHostCommand(agentID, command, 10*time.Second)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func (c *CLI) runUptime(agentID string) error {
	output, err := c.executeHostCommand(agentID, "uptime", 5*time.Second)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func (c *CLI) runTop(agentID string) error {
	return c.executeInteractiveCommand(agentID, "top")
}

func (c *CLI) runHtop(agentID string) error {
	return c.executeInteractiveCommand(agentID, "htop")
}

func (c *CLI) runWho(agentID string) error {
	output, err := c.executeHostCommand(agentID, "who", 5*time.Second)
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) == "" {
		fmt.Println("No users are currently logged in.")
		return nil
	}
	fmt.Print(output)
	return nil
}

func (c *CLI) runLast(agentID, flags string) error {
	command := fmt.Sprintf("last %s", flags)
	output, err := c.executeHostCommand(agentID, command, 10*time.Second)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func (c *CLI) runNetstat(agentID, flags string) error {
	command := fmt.Sprintf("netstat %s || ss %s", flags, flags)
	output, err := c.executeHostCommand(agentID, command, 10*time.Second)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func (c *CLI) runTailLogs(agentID, logFile, flags string) error {
	// If -f flag is present, use interactive mode
	if strings.Contains(flags, "-f") || strings.Contains(flags, "--follow") {
		command := fmt.Sprintf("tail %s %s", flags, logFile)
		return c.executeInteractiveCommand(agentID, command)
	}

	command := fmt.Sprintf("tail %s %s", flags, logFile)
	output, err := c.executeHostCommand(agentID, command, 10*time.Second)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}
