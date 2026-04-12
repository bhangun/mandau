package main

import (
	"io"
	"os/exec"

	agentv1 "github.com/bhangun/mandau/api/v1"
	"github.com/creack/pty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HostShell provides an interactive PTY session to the underlying host OS
func (a *Agent) HostShell(stream agentv1.HostEnvironmentService_HostShellServer) error {
	if a.config.FullConfig.Security.DisableHostShell {
		return status.Errorf(codes.PermissionDenied, "host shell is disabled on this agent by security policy")
	}

	// Try to use bash, fallback to sh
	shellPath := "/bin/bash"
	if _, err := exec.LookPath(shellPath); err != nil {
		shellPath = "/bin/sh"
	}

	cmd := exec.Command(shellPath)
	cmd.Env = append(cmd.Environ(), "TERM=xterm-256color")

	// Start the command with a pty
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to start pty: %v", err)
	}
	defer ptmx.Close()

	// Handle window resizing and stdin in a separate goroutine
	errChan := make(chan error, 1)
	go func() {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				errChan <- nil
				return
			}
			if err != nil {
				errChan <- err
				return
			}

			// Handle window resize
			if req.TermWidth > 0 && req.TermHeight > 0 {
				winsize := &pty.Winsize{
					Rows: uint16(req.TermHeight),
					Cols: uint16(req.TermWidth),
				}
				pty.Setsize(ptmx, winsize)
			}

			// Write incoming data to the pty
			if len(req.Data) > 0 {
				_, err = ptmx.Write(req.Data)
				if err != nil {
					errChan <- err
					return
				}
			}
		}
	}()

	// Read from pty and stream back to client
	buf := make([]byte, 8192)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&agentv1.HostShellResponse{
				Data: buf[:n],
			}); sendErr != nil {
				errChan <- sendErr
				break
			}
		}
		if err != nil {
			// EOF means the shell exited
			if err == io.EOF || err.Error() == "read /dev/ptmx: input/output error" {
				break
			}
			return status.Errorf(codes.Internal, "failed to read from pty: %v", err)
		}
	}

	// Wait for stdin goroutine
	<-errChan

	// Wait for process to exit
	cmd.Wait()

	return nil
}
