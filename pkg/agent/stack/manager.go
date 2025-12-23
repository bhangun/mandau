package stack

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/bhangun/mandau/pkg/agent/operation"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/client"
	"gopkg.in/yaml.v3"
)

type Manager struct {
	mu        sync.RWMutex
	stackRoot string
	docker    *client.Client
	stacks    map[string]*Stack
	opMgr     *operation.Manager
}

type Stack struct {
	ID         string
	Name       string
	Path       string
	Project    *types.Project
	State      StackState
	Containers []ContainerInfo
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Labels     map[string]string
}

type StackState int

const (
	StateUnknown StackState = iota
	StateRunning
	StateStopped
	StateError
	StatePartial
)

type ContainerInfo struct {
	ID      string
	Name    string
	Service string
	State   string
	Status  string
	Image   string
}

func NewManager(stackRoot string, docker *client.Client, opMgr *operation.Manager) *Manager {
	return &Manager{
		stackRoot: stackRoot,
		docker:    docker,
		stacks:    make(map[string]*Stack),
		opMgr:     opMgr,
	}
}

// ListStacks discovers all stacks in the stack root
func (m *Manager) ListStacks(ctx context.Context) ([]*Stack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.stackRoot)
	if err != nil {
		return nil, fmt.Errorf("read stack root: %w", err)
	}

	stacks := make([]*Stack, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stackName := entry.Name()
		stack, err := m.loadStack(ctx, stackName)
		if err != nil {
			// Log but continue - don't fail entire listing
			continue
		}

		stacks = append(stacks, stack)
	}

	return stacks, nil
}

// GetStack retrieves a specific stack with current state
func (m *Manager) GetStack(ctx context.Context, name string) (*Stack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.loadStack(ctx, name)
}

func (m *Manager) loadStack(ctx context.Context, name string) (*Stack, error) {
	stackPath := filepath.Join(m.stackRoot, name)

	// Check if stack directory exists
	if _, err := os.Stat(stackPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("stack not found: %s", name)
	}

	// Load compose file
	composePath := filepath.Join(stackPath, "compose.yaml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		composePath = filepath.Join(stackPath, "docker-compose.yaml")
	}

	composeData, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}

	// Parse compose file
	project, err := m.parseCompose(ctx, name, composeData, stackPath)
	if err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}

	// Get container state
	containers, err := m.getStackContainers(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get containers: %w", err)
	}

	stack := &Stack{
		ID:         name,
		Name:       name,
		Path:       stackPath,
		Project:    project,
		Containers: containers,
		State:      m.determineState(containers),
		Labels:     make(map[string]string),
		UpdatedAt:  time.Now(),
	}

	// Get creation time from directory
	info, _ := os.Stat(stackPath)
	if info != nil {
		stack.CreatedAt = info.ModTime()
	}

	return stack, nil
}

func (m *Manager) parseCompose(ctx context.Context, name string, data []byte, workingDir string) (*types.Project, error) {
	// Parse YAML
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Use compose-go loader
	project, err := loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{
				Content: data,
			},
		},
		Environment: types.NewMapping(nil),
	})
	if err != nil {
		return nil, err
	}

	project.Name = name
	return project, nil
}

func (m *Manager) getStackContainers(ctx context.Context, stackName string) ([]ContainerInfo, error) {
	// Filter by compose project label
	containerFilters := client.Filters{}
	containerFilters.Add("label", fmt.Sprintf("com.docker.compose.project=%s", stackName))

	containerListResult, err := m.docker.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: containerFilters,
	})
	if err != nil {
		return nil, err
	}

	result := make([]ContainerInfo, len(containerListResult.Items))
	for i, c := range containerListResult.Items {
		result[i] = ContainerInfo{
			ID:      c.ID[:12],
			Name:    c.Names[0],
			Service: c.Labels["com.docker.compose.service"],
			State:   string(c.State),
			Status:  c.Status,
			Image:   c.Image,
		}
	}

	return result, nil
}

func (m *Manager) determineState(containers []ContainerInfo) StackState {
	if len(containers) == 0 {
		return StateStopped
	}

	running := 0
	stopped := 0

	for _, c := range containers {
		if c.State == "running" {
			running++
		} else {
			stopped++
		}
	}

	if running == len(containers) {
		return StateRunning
	}
	if stopped == len(containers) {
		return StateStopped
	}
	return StatePartial
}

// ApplyStack applies a compose file (create or update)
func (m *Manager) ApplyStack(ctx context.Context, req *ApplyStackRequest) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stackPath := filepath.Join(m.stackRoot, req.StackName)

	// Create stack directory if doesn't exist
	if err := os.MkdirAll(stackPath, 0755); err != nil {
		return "", fmt.Errorf("create stack dir: %w", err)
	}

	// Write compose file
	composePath := filepath.Join(stackPath, "compose.yaml")
	if err := os.WriteFile(composePath, []byte(req.ComposeContent), 0644); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}

	// Write env file if provided
	if len(req.EnvVars) > 0 {
		envPath := filepath.Join(stackPath, ".env")
		envContent := ""
		for k, v := range req.EnvVars {
			envContent += fmt.Sprintf("%s=%s\n", k, v)
		}
		if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
			return "", fmt.Errorf("write env file: %w", err)
		}
	}

	// Create operation for async execution
	opID := m.opMgr.CreateOperation(operation.OperationTypeStackApply, map[string]string{
		"stack": req.StackName,
	})

	// Execute in background
	go m.executeApply(context.Background(), opID, req, stackPath)

	return opID, nil
}

func (m *Manager) executeApply(ctx context.Context, opID string, req *ApplyStackRequest, stackPath string) {
	m.opMgr.SetState(opID, operation.OperationStateRunning)
	m.opMgr.EmitEvent(opID, "Parsing compose file...")

	// Parse project
	composeData := []byte(req.ComposeContent)
	project, err := m.parseCompose(ctx, req.StackName, composeData, stackPath)
	if err != nil {
		m.opMgr.SetError(opID, fmt.Errorf("parse compose: %w", err))
		return
	}

	// Pull images if requested
	if req.PullImages {
		m.opMgr.EmitEvent(opID, "Pulling images...")
		if err := m.pullImages(ctx, project); err != nil {
			m.opMgr.SetError(opID, fmt.Errorf("pull images: %w", err))
			return
		}
	}

	// Apply using docker compose
	m.opMgr.EmitEvent(opID, "Creating/updating services...")

	// Use docker compose CLI via exec (compose-go doesn't support full lifecycle)
	// In production, this would use the compose API or reimplemented logic
	// Use relative path from stack root directory
	relativeComposePath := filepath.Join(req.StackName, "compose.yaml")
	cmd := []string{"docker", "compose", "-f", relativeComposePath, "up", "-d"}

	if req.ForceRecreate {
		cmd = append(cmd, "--force-recreate")
	}

	if len(req.Services) > 0 {
		cmd = append(cmd, req.Services...)
	}

	// Execute command (simplified - production would stream output)
	if err := m.execCommand(ctx, cmd); err != nil {
		m.opMgr.SetError(opID, fmt.Errorf("compose up: %w", err))
		return
	}

	m.opMgr.EmitEvent(opID, "Stack applied successfully")
	m.opMgr.SetCompleted(opID)
}

func (m *Manager) pullImages(ctx context.Context, project *types.Project) error {
	for _, service := range project.Services {
		if service.Image == "" {
			continue
		}
		// Pull image using Docker SDK
		// Simplified - production would stream progress
		reader, err := m.docker.ImagePull(ctx, service.Image, client.ImagePullOptions{})
		if err != nil {
			return err
		}
		io.Copy(io.Discard, reader)
		reader.Close()
	}
	return nil
}

// DiffStack compares current stack with new compose content
func (m *Manager) DiffStack(ctx context.Context, stackName string, newContent string) (*DiffResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Load current stack
	current, err := m.loadStack(ctx, stackName)
	if err != nil {
		return nil, fmt.Errorf("load current stack: %w", err)
	}

	// Parse new compose
	newProject, err := m.parseCompose(ctx, stackName, []byte(newContent), current.Path)
	if err != nil {
		return nil, fmt.Errorf("parse new compose: %w", err)
	}

	return m.computeDiff(current.Project, newProject), nil
}

func (m *Manager) computeDiff(current, new *types.Project) *DiffResult {
	result := &DiffResult{
		Services: make([]ServiceDiff, 0),
	}

	currentServices := make(map[string]types.ServiceConfig)
	for _, svc := range current.Services {
		currentServices[svc.Name] = svc
	}

	newServices := make(map[string]types.ServiceConfig)
	for _, svc := range new.Services {
		newServices[svc.Name] = svc
	}

	// Check for new and updated services
	for name, newSvc := range newServices {
		if currentSvc, exists := currentServices[name]; exists {
			// Compare services
			changes := m.compareServices(currentSvc, newSvc)
			if len(changes) > 0 {
				result.Services = append(result.Services, ServiceDiff{
					Name:    name,
					Action:  DiffActionUpdate,
					Changes: changes,
				})
				result.HasChanges = true
			}
		} else {
			// New service
			result.Services = append(result.Services, ServiceDiff{
				Name:   name,
				Action: DiffActionCreate,
			})
			result.HasChanges = true
		}
	}

	// Check for deleted services
	for name := range currentServices {
		if _, exists := newServices[name]; !exists {
			result.Services = append(result.Services, ServiceDiff{
				Name:   name,
				Action: DiffActionDelete,
			})
			result.HasChanges = true
		}
	}

	return result
}

func (m *Manager) compareServices(current, new types.ServiceConfig) []string {
	changes := make([]string, 0)

	if current.Image != new.Image {
		changes = append(changes, fmt.Sprintf("image: %s → %s", current.Image, new.Image))
	}

	if len(current.Ports) != len(new.Ports) {
		changes = append(changes, "ports changed")
	}

	if len(current.Environment) != len(new.Environment) {
		changes = append(changes, "environment changed")
	}

	// Compare other fields as needed

	return changes
}

// RemoveStack removes a stack and its containers
func (m *Manager) RemoveStack(ctx context.Context, stackName string, removeVolumes bool) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stackPath := filepath.Join(m.stackRoot, stackName)

	opID := m.opMgr.CreateOperation(operation.OperationTypeStackRemove, map[string]string{
		"stack": stackName,
	})

	go m.executeRemove(context.Background(), opID, stackName, stackPath, removeVolumes)

	return opID, nil
}

func (m *Manager) executeRemove(ctx context.Context, opID, stackName, stackPath string, removeVolumes bool) {
	m.opMgr.SetState(opID, operation.OperationStateRunning)
	m.opMgr.EmitEvent(opID, "Stopping containers...")

	// Execute docker compose down
	relativeComposePath := filepath.Join(stackName, "compose.yaml")
	cmd := []string{"docker", "compose", "-f", relativeComposePath, "down"}
	if removeVolumes {
		cmd = append(cmd, "--volumes")
	}

	if err := m.execCommand(ctx, cmd); err != nil {
		m.opMgr.SetError(opID, fmt.Errorf("compose down: %w", err))
		return
	}

	m.opMgr.EmitEvent(opID, "Removing stack directory...")
	if err := os.RemoveAll(stackPath); err != nil {
		m.opMgr.SetError(opID, fmt.Errorf("remove directory: %w", err))
		return
	}

	m.opMgr.EmitEvent(opID, "Stack removed successfully")
	m.opMgr.SetCompleted(opID)
}

func (m *Manager) execCommand(ctx context.Context, cmd []string) error {
	// Execute the command with proper context and error handling
	command := exec.CommandContext(ctx, cmd[0], cmd[1:]...)

	// Set working directory to the stack root directory so compose files can be found
	command.Dir = m.stackRoot

	// Execute the command
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}

	return nil
}

type ApplyStackRequest struct {
	StackName      string
	ComposeContent string
	EnvVars        map[string]string
	ForceRecreate  bool
	Services       []string
	PullImages     bool
}

type DiffResult struct {
	Services   []ServiceDiff
	HasChanges bool
}

type ServiceDiff struct {
	Name    string
	Action  DiffAction
	Changes []string
}

type DiffAction int

const (
	DiffActionNone DiffAction = iota
	DiffActionCreate
	DiffActionUpdate
	DiffActionDelete
)
