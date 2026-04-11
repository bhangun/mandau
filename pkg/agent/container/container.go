package container

import (
	"context"
	"os/exec"
	"strings"

	agentv1 "github.com/bhangun/mandau/api/v1"
	"github.com/moby/moby/client"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Manager manages containers
type Manager struct {
	docker *client.Client
}

// NewManager creates a new container manager
func NewManager(docker *client.Client) *Manager {
	return &Manager{
		docker: docker,
	}
}

// ListContainers returns all containers on the host
func (m *Manager) ListContainers(ctx context.Context) ([]*agentv1.Container, error) {
	containers, err := m.docker.ContainerList(ctx, client.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	result := make([]*agentv1.Container, len(containers.Items))
	for i, c := range containers.Items {
		// Clean up name (Docker adds a leading slash)
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		result[i] = &agentv1.Container{
			Id:      c.ID[:12],
			Name:    name,
			Image:   c.Image,
			State:   string(c.State),
			Status:  c.Status,
			Created: timestamppb.New(timestamppb.Now().AsTime()), // c.Created is an int64 timestamp
			Labels:  c.Labels,
		}

		// Map ports
		for _, p := range c.Ports {
			result[i].Ports = append(result[i].Ports, &agentv1.Port{
				Ip:          p.IP.String(),
				PrivatePort: uint32(p.PrivatePort),
				PublicPort:  uint32(p.PublicPort),
				Type:        p.Type,
			})
		}
	}

	return result, nil
}

// ExecuteDockerCommand executes a docker command on the host
func (m *Manager) ExecuteDockerCommand(ctx context.Context, args []string, onOutput func([]byte) error) (int32, error) {
	// We use the docker CLI directly for generic command wrapping
	cmd := exec.CommandContext(ctx, "docker", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, err
	}
	cmd.Stderr = cmd.Stdout // Redirect stderr to stdout for streaming

	if err := cmd.Start(); err != nil {
		return 1, err
	}

	buf := make([]byte, 4096)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			if err := onOutput(buf[:n]); err != nil {
				return 1, err
			}
		}
		if err != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return int32(exitErr.ExitCode()), nil
		}
		return 1, err
	}

	return 0, nil
}
