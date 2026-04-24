package main

import (
	"io"
	"os/exec"
	"strings"

	agentv1 "github.com/bhangun/mandau/api/v1"
	"github.com/creack/pty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HostShell provides either an interactive PTY session or a raw non-PTY session
// depending on the client's first request. If the client sends TermWidth/TermHeight
// (>0) in the first request, a PTY is allocated. Otherwise a raw stdin/stdout
// session is used which is suitable for streaming binary data like 'docker load -i -'.
func (a *Agent) HostShell(stream agentv1.HostEnvironmentService_HostShellServer) error {
	if a.config.FullConfig.Security.DisableHostShell {
		return status.Errorf(codes.PermissionDenied, "host shell is disabled on this agent by security policy")
	}

	// Try to use bash, fallback to sh
	shellPath := "/bin/bash"
	if _, err := exec.LookPath(shellPath); err != nil {
		shellPath = "/bin/sh"
	}

	// Receive the first request to decide mode (PTY vs raw)
	firstReq, err := stream.Recv()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return status.Errorf(codes.Internal, "failed to receive initial host shell request: %v", err)
	}

	// If TermWidth/TermHeight are provided, allocate a PTY (interactive)
	if firstReq.TermWidth > 0 && firstReq.TermHeight > 0 {
		cmd := exec.Command(shellPath)
		cmd.Env = append(cmd.Environ(), "TERM=xterm-256color")

		ptmx, err := pty.Start(cmd)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to start pty: %v", err)
		}
		defer ptmx.Close()

		// Apply initial winsize if present
		winsize := &pty.Winsize{
			Rows: uint16(firstReq.TermHeight),
			Cols: uint16(firstReq.TermWidth),
		}
		pty.Setsize(ptmx, winsize)

		// If the initial request contains data, write it to the pty
		if len(firstReq.Data) > 0 {
			if _, werr := ptmx.Write(firstReq.Data); werr != nil {
				return status.Errorf(codes.Internal, "failed to write to pty: %v", werr)
			}
		}

		// Goroutine: receive further requests (resize or stdin) and handle them
		errChan := make(chan error, 1)
		go func() {
			for {
				req, rerr := stream.Recv()
				if rerr == io.EOF {
					errChan <- nil
					return
				}
				if rerr != nil {
					errChan <- rerr
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
					if _, werr := ptmx.Write(req.Data); werr != nil {
						errChan <- werr
						return
					}
				}
			}
		}()

		// Read from pty and stream back to client
		buf := make([]byte, 8192)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				if sendErr := stream.Send(&agentv1.HostShellResponse{Data: buf[:n]}); sendErr != nil {
					errChan <- sendErr
					break
				}
			}
			if rerr != nil {
				if rerr == io.EOF || rerr.Error() == "read /dev/ptmx: input/output error" {
					break
				}
				return status.Errorf(codes.Internal, "failed to read from pty: %v", rerr)
			}
		}

		// Wait for stdin goroutine
		<-errChan

		// Wait for process to exit
		cmd.Wait()

		return nil
	}

	// Otherwise: raw non-PTY mode.
	// If the client sent an explicit docker load command as the first request, run
	// the docker process directly to avoid any shell interpretation of binary data.
	cmdStr := strings.TrimSpace(string(firstReq.Data))
	if strings.HasPrefix(cmdStr, "docker load") {
		// Start docker load directly
		dockerCmd := exec.Command("docker", "load", "-i", "-")
		din, inErr := dockerCmd.StdinPipe()
		if inErr != nil {
			return status.Errorf(codes.Internal, "failed to get docker stdin pipe: %v", inErr)
		}
		out, outErr := dockerCmd.StdoutPipe()
		if outErr != nil {
			return status.Errorf(codes.Internal, "failed to get docker stdout pipe: %v", outErr)
		}
		errOut, errErr := dockerCmd.StderrPipe()
		if errErr != nil {
			return status.Errorf(codes.Internal, "failed to get docker stderr pipe: %v", errErr)
		}

		if startErr := dockerCmd.Start(); startErr != nil {
			return status.Errorf(codes.Internal, "failed to start docker load: %v", startErr)
		}

		// Do NOT write the command string to stdin. The initial request was command.

		// Goroutine: receive further data and write to docker stdin
		go func() {
			defer din.Close()
			for {
				req, rerr := stream.Recv()
				if rerr == io.EOF {
					return
				}
				if rerr != nil {
					return
				}
				if len(req.Data) > 0 {
					din.Write(req.Data)
				}
			}
		}()

		// Relay docker stdout/stderr back to client
		buf := make([]byte, 8192)
		go func() {
			for {
				n, rerr := out.Read(buf)
				if n > 0 {
					stream.Send(&agentv1.HostShellResponse{Data: append([]byte{}, buf[:n]...)})
				}
				if rerr != nil {
					return
				}
			}
		}()
		go func() {
			for {
				n, rerr := errOut.Read(buf)
				if n > 0 {
					stream.Send(&agentv1.HostShellResponse{Data: append([]byte{}, buf[:n]...)})
				}
				if rerr != nil {
					return
				}
			}
		}()

		// Wait for docker to finish
		dockerCmd.Wait()
		return nil
	}

	// Fallback: start a plain shell and proxy stdin/stdout/stderr (for other commands)
	cmd := exec.Command(shellPath)
	stdin, inErr := cmd.StdinPipe()
	if inErr != nil {
		return status.Errorf(codes.Internal, "failed to get stdin pipe: %v", inErr)
	}
	stdout, outErr := cmd.StdoutPipe()
	if outErr != nil {
		return status.Errorf(codes.Internal, "failed to get stdout pipe: %v", outErr)
	}
	stderr, errErr := cmd.StderrPipe()
	if errErr != nil {
		return status.Errorf(codes.Internal, "failed to get stderr pipe: %v", errErr)
	}

	if startErr := cmd.Start(); startErr != nil {
		return status.Errorf(codes.Internal, "failed to start shell: %v", startErr)
	}

	// Write initial data (this contains the command)
	if len(firstReq.Data) > 0 {
		if _, werr := stdin.Write(firstReq.Data); werr != nil {
			stdin.Close()
			return status.Errorf(codes.Internal, "failed to write to stdin: %v", werr)
		}
	}

	// Goroutine: receive further data and write to stdin
	go func() {
		defer stdin.Close()
		for {
			req, rerr := stream.Recv()
			if rerr == io.EOF {
				return
			}
			if rerr != nil {
				return
			}
			if len(req.Data) > 0 {
				stdin.Write(req.Data)
			}
		}
	}()

	// Send stdout and stderr back to client
	buf := make([]byte, 8192)
	// stdout reader
	go func() {
		for {
			n, rerr := stdout.Read(buf)
			if n > 0 {
				stream.Send(&agentv1.HostShellResponse{Data: append([]byte{}, buf[:n]...)})
			}
			if rerr != nil {
				return
			}
		}
	}()

	// stderr reader
	go func() {
		for {
			n, rerr := stderr.Read(buf)
			if n > 0 {
				stream.Send(&agentv1.HostShellResponse{Data: append([]byte{}, buf[:n]...)})
			}
			if rerr != nil {
				return
			}
		}
	}()

	// Wait for process to exit
	cmd.Wait()

	return nil
}
