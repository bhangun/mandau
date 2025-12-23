package systemd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/bhangun/mandau/pkg/plugin"
)

type SystemdPlugin struct {
	name    string
	version string
	config  *SystemdConfig
}

type SystemdConfig struct {
	UnitDir      string
	SystemctlCmd string
}

type ServiceUnit struct {
	Name        string
	Description string
	After       []string
	Requires    []string
	Type        string // simple, forking, oneshot, etc.
	User        string
	Group       string
	WorkingDir  string
	ExecStart   string
	ExecStop    string
	ExecReload  string
	Environment map[string]string
	Restart     string
	RestartSec  int
	KillMode    string
	// Resource limits
	LimitNOFILE int
	LimitNPROC  int
	CPUQuota    string
	MemoryLimit string
	// Security
	PrivateTmp        bool
	ProtectSystem     string
	ProtectHome       bool
	NoNewPrivileges   bool
	ReadWritePaths    []string
	ReadOnlyPaths     []string
	InaccessiblePaths []string
	// Custom sections
	CustomUnit    string
	CustomService string
	CustomInstall string
}

func New() *SystemdPlugin {
	return &SystemdPlugin{
		name:    "systemd-manager",
		version: "1.0.0",
	}
}

func (p *SystemdPlugin) Name() string    { return p.name }
func (p *SystemdPlugin) Version() string { return p.version }

func (p *SystemdPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityStorage}
}

func (p *SystemdPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	p.config = &SystemdConfig{
		UnitDir:      "/etc/systemd/system",
		SystemctlCmd: "systemctl",
	}

	return nil
}

func (p *SystemdPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// CreateService creates a systemd service unit
func (p *SystemdPlugin) CreateService(unit *ServiceUnit) error {
	unitPath := filepath.Join(p.config.UnitDir, unit.Name+".service")

	tmpl := template.Must(template.New("service").Parse(systemdServiceTemplate))

	file, err := os.Create(unitPath)
	if err != nil {
		return fmt.Errorf("create unit: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, unit); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	// Reload systemd
	return p.daemonReload()
}

// EnableService enables a systemd service
func (p *SystemdPlugin) EnableService(serviceName string) error {
	cmd := exec.Command(p.config.SystemctlCmd, "enable", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("enable failed: %s", output)
	}
	return nil
}

// DisableService disables a systemd service
func (p *SystemdPlugin) DisableService(serviceName string) error {
	cmd := exec.Command(p.config.SystemctlCmd, "disable", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("disable failed: %s", output)
	}
	return nil
}

// StartService starts a systemd service
func (p *SystemdPlugin) StartService(serviceName string) error {
	cmd := exec.Command(p.config.SystemctlCmd, "start", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start failed: %s", output)
	}
	return nil
}

// StopService stops a systemd service
func (p *SystemdPlugin) StopService(serviceName string) error {
	cmd := exec.Command(p.config.SystemctlCmd, "stop", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop failed: %s", output)
	}
	return nil
}

// RestartService restarts a systemd service
func (p *SystemdPlugin) RestartService(serviceName string) error {
	cmd := exec.Command(p.config.SystemctlCmd, "restart", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart failed: %s", output)
	}
	return nil
}

// GetServiceStatus returns the status of a service
func (p *SystemdPlugin) GetServiceStatus(serviceName string) (string, error) {
	cmd := exec.Command(p.config.SystemctlCmd, "is-active", serviceName)
	output, err := cmd.Output()
	if err != nil {
		return "unknown", nil
	}
	return string(output), nil
}

func (p *SystemdPlugin) daemonReload() error {
	cmd := exec.Command(p.config.SystemctlCmd, "daemon-reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("daemon-reload failed: %s", output)
	}
	return nil
}

const systemdServiceTemplate = `# Managed by Mandau
[Unit]
Description={{.Description}}
{{range .After}}After={{.}}
{{end}}{{range .Requires}}Requires={{.}}
{{end}}
{{.CustomUnit}}

[Service]
Type={{if .Type}}{{.Type}}{{else}}simple{{end}}
{{if .User}}User={{.User}}{{end}}
{{if .Group}}Group={{.Group}}{{end}}
{{if .WorkingDir}}WorkingDirectory={{.WorkingDir}}{{end}}
ExecStart={{.ExecStart}}
{{if .ExecStop}}ExecStop={{.ExecStop}}{{end}}
{{if .ExecReload}}ExecReload={{.ExecReload}}{{end}}

{{range $key, $value := .Environment}}
Environment="{{$key}}={{$value}}"
{{end}}

{{if .Restart}}Restart={{.Restart}}{{end}}
{{if .RestartSec}}RestartSec={{.RestartSec}}{{end}}
{{if .KillMode}}KillMode={{.KillMode}}{{end}}

# Resource Limits
{{if .LimitNOFILE}}LimitNOFILE={{.LimitNOFILE}}{{end}}
{{if .LimitNPROC}}LimitNPROC={{.LimitNPROC}}{{end}}
{{if .CPUQuota}}CPUQuota={{.CPUQuota}}{{end}}
{{if .MemoryLimit}}MemoryLimit={{.MemoryLimit}}{{end}}

# Security
{{if .PrivateTmp}}PrivateTmp=yes{{end}}
{{if .ProtectSystem}}ProtectSystem={{.ProtectSystem}}{{end}}
{{if .ProtectHome}}ProtectHome=yes{{end}}
{{if .NoNewPrivileges}}NoNewPrivileges=yes{{end}}
{{range .ReadWritePaths}}ReadWritePaths={{.}}
{{end}}{{range .ReadOnlyPaths}}ReadOnlyPaths={{.}}
{{end}}{{range .InaccessiblePaths}}InaccessiblePaths={{.}}
{{end}}

{{.CustomService}}

[Install]
WantedBy=multi-user.target
{{.CustomInstall}}`
