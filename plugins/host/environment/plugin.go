package environment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bhangun/mandau/pkg/plugin"
)

type EnvironmentPlugin struct {
	name    string
	version string
}

type HostInfo struct {
	Hostname     string
	OS           string
	Kernel       string
	Architecture string
	CPUCores     int
	MemoryMB     int64
	DiskGB       int64
	Uptime       string
}

type Package struct {
	Name    string
	Version string
	Status  string
}

func New() *EnvironmentPlugin {
	return &EnvironmentPlugin{
		name:    "host-environment",
		version: "1.0.0",
	}
}

func (p *EnvironmentPlugin) Name() string    { return p.name }
func (p *EnvironmentPlugin) Version() string { return p.version }

func (p *EnvironmentPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityMonitor}
}

func (p *EnvironmentPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (p *EnvironmentPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// GetHostInfo retrieves host system information
func (p *EnvironmentPlugin) GetHostInfo() (*HostInfo, error) {
	info := &HostInfo{}

	// Hostname
	hostname, _ := os.Hostname()
	info.Hostname = hostname

	// OS Info
	osInfo, _ := exec.Command("uname", "-s").Output()
	info.OS = strings.TrimSpace(string(osInfo))

	// Kernel
	kernel, _ := exec.Command("uname", "-r").Output()
	info.Kernel = strings.TrimSpace(string(kernel))

	// Architecture
	arch, _ := exec.Command("uname", "-m").Output()
	info.Architecture = strings.TrimSpace(string(arch))

	// CPU cores
	cpuInfo, _ := exec.Command("nproc").Output()
	fmt.Sscanf(string(cpuInfo), "%d", &info.CPUCores)

	return info, nil
}

// InstallPackage installs a system package
func (p *EnvironmentPlugin) InstallPackage(packageName string) error {
	// Detect package manager
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.Command("apt-get", "install", "-y", packageName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("apt-get failed: %s", output)
		}
	} else if _, err := exec.LookPath("yum"); err == nil {
		cmd := exec.Command("yum", "install", "-y", packageName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("yum failed: %s", output)
		}
	} else {
		return fmt.Errorf("no package manager found")
	}

	return nil
}

// RemovePackage removes a system package
func (p *EnvironmentPlugin) RemovePackage(packageName string) error {
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.Command("apt-get", "remove", "-y", packageName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("apt-get failed: %s", output)
		}
	} else if _, err := exec.LookPath("yum"); err == nil {
		cmd := exec.Command("yum", "remove", "-y", packageName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("yum failed: %s", output)
		}
	}

	return nil
}

// UpdatePackages updates all system packages
func (p *EnvironmentPlugin) UpdatePackages() error {
	if _, err := exec.LookPath("apt-get"); err == nil {
		// Update package list
		cmd := exec.Command("apt-get", "update")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("apt-get update failed: %s", output)
		}

		// Upgrade packages
		cmd = exec.Command("apt-get", "upgrade", "-y")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("apt-get upgrade failed: %s", output)
		}
	} else if _, err := exec.LookPath("yum"); err == nil {
		cmd := exec.Command("yum", "update", "-y")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("yum update failed: %s", output)
		}
	}

	return nil
}

// ListPackages lists installed packages
func (p *EnvironmentPlugin) ListPackages() ([]*Package, error) {
	packages := []*Package{}

	if _, err := exec.LookPath("dpkg"); err == nil {
		cmd := exec.Command("dpkg", "-l")
		output, err := cmd.Output()
		if err != nil {
			return nil, err
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ii") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					packages = append(packages, &Package{
						Name:    fields[1],
						Version: fields[2],
						Status:  "installed",
					})
				}
			}
		}
	}

	return packages, nil
}

// SetSysctl sets a kernel parameter
func (p *EnvironmentPlugin) SetSysctl(key, value string) error {
	cmd := exec.Command("sysctl", "-w", fmt.Sprintf("%s=%s", key, value))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysctl failed: %s", output)
	}
	return nil
}

// GetSysctl gets a kernel parameter
func (p *EnvironmentPlugin) GetSysctl(key string) (string, error) {
	cmd := exec.Command("sysctl", "-n", key)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
