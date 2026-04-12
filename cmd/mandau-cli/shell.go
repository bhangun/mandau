package main

import (
	"context"
	"fmt"
	"io"
	"os"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var shellCmd = &cobra.Command{
	Use:   "shell [agent-id]",
	Short: "Open an interactive host shell on a remote agent",
	Long: `Opens an interactive secure shell directly on the agent's underlying operating system.
This requires the agent to have 'enable_host_shell' configured securely.
	
Example:
  mandau shell agent-001`,
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

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stream, err := cli.coreClient.HostShell(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect shell stream: %w", err)
		}

		// Initial connection request
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
					// Don't send error to channel if it's just a closure
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
			// Restore term before printing error
			term.Restore(fd, oldState)
			return err
		}

		return nil
	},
}
