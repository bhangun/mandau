package cron

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bhangun/mandau/pkg/plugin"
)

type CronPlugin struct {
	name    string
	version string
	config  *CronConfig
}

type CronConfig struct {
	CronDir string
	User    string
}

type CronJob struct {
	Name     string
	Schedule string // Cron expression
	Command  string
	User     string
	Enabled  bool
}

func New() *CronPlugin {
	return &CronPlugin{
		name:    "cron-manager",
		version: "1.0.0",
	}
}

func (p *CronPlugin) Name() string    { return p.name }
func (p *CronPlugin) Version() string { return p.version }

func (p *CronPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityStorage}
}

func (p *CronPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	p.config = &CronConfig{
		CronDir: "/etc/cron.d",
		User:    "root",
	}

	if user, ok := config["user"].(string); ok {
		p.config.User = user
	}

	return nil
}

func (p *CronPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// AddCronJob adds a cron job
func (p *CronPlugin) AddCronJob(job *CronJob) error {
	cronFile := filepath.Join(p.config.CronDir, "mandau-"+job.Name)

	user := job.User
	if user == "" {
		user = p.config.User
	}

	content := fmt.Sprintf("# Managed by Mandau\n%s %s %s\n",
		job.Schedule,
		user,
		job.Command,
	)

	if err := os.WriteFile(cronFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("write cron file: %w", err)
	}

	return nil
}

// RemoveCronJob removes a cron job
func (p *CronPlugin) RemoveCronJob(name string) error {
	cronFile := filepath.Join(p.config.CronDir, "mandau-"+name)
	return os.Remove(cronFile)
}

// ListCronJobs lists all Mandau-managed cron jobs
func (p *CronPlugin) ListCronJobs() ([]*CronJob, error) {
	jobs := []*CronJob{}

	files, err := filepath.Glob(filepath.Join(p.config.CronDir, "mandau-*"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "#") || line == "" {
				continue
			}

			parts := strings.Fields(line)
			if len(parts) >= 7 {
				jobs = append(jobs, &CronJob{
					Name:     filepath.Base(file)[7:], // Remove "mandau-" prefix
					Schedule: strings.Join(parts[0:5], " "),
					User:     parts[5],
					Command:  strings.Join(parts[6:], " "),
					Enabled:  true,
				})
			}
		}
	}

	return jobs, nil
}
