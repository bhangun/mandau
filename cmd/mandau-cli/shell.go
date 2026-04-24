package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var shellCmd = &cobra.Command{
	Use:   "shell [agent-id]",
	Short: "Open an interactive host shell on a remote agent",
	Long: `Opens an interactive secure shell directly on the agent's underlying operating system.
This requires the agent to have 'enable_host_shell' configured securely.

Terminal Features:
  • Automatic terminal resize handling
  • Full TTY support
  • Color output preserved
  • Ctrl+C handling

Examples:
  mandau shell
  mandau shell agent-001
  mandau shell  # auto-selects first available agent`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use provided agent ID or interactive selection
		var agentID string
		if len(args) > 0 {
			agentID = args[0]
		} else {
			id, err := cli.resolveAgent(cmd)
			if err != nil {
				return err
			}
			agentID = id
		}

		// Show connection message BEFORE entering raw mode
		fmt.Printf("🔌 Connecting to agent '%s'...\n", agentID)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stream, err := cli.coreClient.HostShell(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect shell stream: %w", err)
		}

		// Get initial terminal size before going raw
		fd := int(os.Stdin.Fd())
		width, height, _ := term.GetSize(fd)

		// Initial connection request with terminal size
		err = stream.Send(&v1.HostShellRequest{
			AgentId:    agentID,
			TermWidth:  int32(width),
			TermHeight: int32(height),
		})
		if err != nil {
			return fmt.Errorf("failed to initialize shell: %w", err)
		}

		// Put local terminal in raw mode
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return fmt.Errorf("failed to set raw terminal mode: %w", err)
		}

		// Ensure terminal is restored on exit
		restored := false
		defer func() {
			if !restored {
				term.Restore(fd, oldState)
			}
		}()

		// Send a newline to trigger the shell prompt
		time.Sleep(100 * time.Millisecond)
		stream.Send(&v1.HostShellRequest{
			Data: []byte("\n"),
		})

		// Set up terminal resize handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGWINCH)
		defer signal.Stop(sigChan)

		// Send resize events
		go func() {
			for range sigChan {
				w, h, err := term.GetSize(fd)
				if err == nil {
					stream.Send(&v1.HostShellRequest{
						TermWidth:  int32(w),
						TermHeight: int32(h),
					})
				}
			}
		}()

		errChan := make(chan error, 2)

		// Read from remote and write to local stdout
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
					os.Stdout.Sync()
				}
			}
		}()

		// Read from local stdin and write to remote
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
		err = <-errChan
		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "\nShell disconnected: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "\nShell session ended.\n")
		}

		// Restore terminal before exiting
		term.Restore(fd, oldState)
		restored = true

		return err
	},
}
