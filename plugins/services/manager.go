package services

import (
	"context"
	"fmt"

	"github.com/bhangun/mandau/pkg/plugin"

	"github.com/bhangun/mandau/plugins/host/cron"
	"github.com/bhangun/mandau/plugins/host/environment"
	"github.com/bhangun/mandau/plugins/security/acme"
	"github.com/bhangun/mandau/plugins/services/dns"
	"github.com/bhangun/mandau/plugins/services/firewall"
	"github.com/bhangun/mandau/plugins/services/nginx"
	"github.com/bhangun/mandau/plugins/services/systemd"
)

// ServiceManager coordinates all host-level service plugins
type ServiceManager struct {
	nginx       *nginx.NginxPlugin
	systemd     *systemd.SystemdPlugin
	firewall    *firewall.FirewallPlugin
	environment *environment.EnvironmentPlugin
	cron        *cron.CronPlugin
	acme        *acme.ACMEPlugin
	dns         *dns.DNSPlugin
}

func NewServiceManager(ctx context.Context) (*ServiceManager, error) {
	mgr := &ServiceManager{
		nginx:       nginx.New(),
		systemd:     systemd.New(),
		firewall:    firewall.New(),
		environment: environment.New(),
		cron:        cron.New(),
		acme:        acme.New(),
		dns:         dns.New(),
	}

	// Initialize all plugins
	plugins := []struct {
		name   string
		plugin plugin.Plugin
		config map[string]interface{}
	}{
		{"nginx", mgr.nginx, map[string]interface{}{}},
		{"systemd", mgr.systemd, map[string]interface{}{}},
		{"firewall", mgr.firewall, map[string]interface{}{"backend": "ufw"}},
		{"environment", mgr.environment, map[string]interface{}{}},
		{"cron", mgr.cron, map[string]interface{}{}},
		{"acme", mgr.acme, map[string]interface{}{"production": false}},
		{"dns", mgr.dns, map[string]interface{}{}},
	}

	for _, p := range plugins {
		if err := p.plugin.Init(ctx, p.config); err != nil {
			return nil, fmt.Errorf("init %s: %w", p.name, err)
		}
	}

	return mgr, nil
}

// DeployWebService deploys a complete web service with nginx, systemd, firewall, and SSL
func (m *ServiceManager) DeployWebService(ctx context.Context, config *WebServiceConfig) error {
	// 1. Create systemd service
	service := &systemd.ServiceUnit{
		Name:        config.Name,
		Description: config.Description,
		User:        config.User,
		WorkingDir:  config.WorkingDir,
		ExecStart:   config.Command,
		Restart:     "always",
		RestartSec:  10,
		Environment: config.Environment,
	}

	if err := m.systemd.CreateService(service); err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	if err := m.systemd.EnableService(config.Name); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	if err := m.systemd.StartService(config.Name); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	// 2. Configure nginx reverse proxy
	if err := m.nginx.CreateReverseProxy(
		config.Domain,
		fmt.Sprintf("http://127.0.0.1:%d", config.Port),
		80,
	); err != nil {
		return fmt.Errorf("create nginx config: %w", err)
	}

	if err := m.nginx.EnableVirtualHost(config.Domain); err != nil {
		return fmt.Errorf("enable nginx vhost: %w", err)
	}

	// 3. Open firewall ports
	if err := m.firewall.AllowPort(80, "tcp"); err != nil {
		return fmt.Errorf("open firewall port 80: %w", err)
	}

	if err := m.firewall.AllowPort(443, "tcp"); err != nil {
		return fmt.Errorf("open firewall port 443: %w", err)
	}

	// 4. Obtain SSL certificate
	if config.SSL {
		cert, err := m.acme.ObtainCertificate(config.Domain)
		if err != nil {
			return fmt.Errorf("obtain certificate: %w", err)
		}

		// Update nginx config with SSL
		vhost := &nginx.VirtualHost{
			ServerName: config.Domain,
			Listen:     443,
			ProxyPass:  fmt.Sprintf("http://127.0.0.1:%d", config.Port),
			SSL: &nginx.SSLConfig{
				Certificate:    cert.CertPath,
				CertificateKey: cert.KeyPath,
				Protocols:      []string{"TLSv1.2", "TLSv1.3"},
			},
		}

		if err := m.nginx.CreateVirtualHost(vhost); err != nil {
			return fmt.Errorf("create SSL vhost: %w", err)
		}
	}

	// 5. Add automatic renewal cron job
	if config.SSL {
		cronJob := &cron.CronJob{
			Name:     config.Name + "-cert-renewal",
			Schedule: "0 0 * * *", // Daily at midnight
			Command:  "certbot renew && nginx -s reload",
		}

		if err := m.cron.AddCronJob(cronJob); err != nil {
			return fmt.Errorf("add cron job: %w", err)
		}
	}

	return nil
}

type WebServiceConfig struct {
	Name        string
	Description string
	Domain      string
	Port        int
	Command     string
	WorkingDir  string
	User        string
	SSL         bool
	Environment map[string]string
}

// Shutdown gracefully shuts down all service plugins
func (m *ServiceManager) Shutdown(ctx context.Context) error {
	plugins := []plugin.Plugin{
		m.nginx,
		m.systemd,
		m.firewall,
		m.environment,
		m.cron,
		m.acme,
		m.dns,
	}

	for _, p := range plugins {
		if err := p.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down %s: %v\n", p.Name(), err)
		}
	}

	return nil
}
