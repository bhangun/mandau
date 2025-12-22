package container

// Container represents a container
type Container struct {
	ID   string
	Name string
}

// Manager manages containers
type Manager struct{}

// NewManager creates a new container manager
func NewManager() *Manager {
	return &Manager{}
}
