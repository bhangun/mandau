package nginx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/bhangun/mandau/pkg/plugin"
)

type NginxPlugin struct {
	name    string
	version string
	config  *NginxConfig
}

type NginxConfig struct {
	ConfigDir     string
	EnabledDir    string
	AvailableDir  string
	ReloadCommand string
	TestCommand   string
	AutoReload    bool
}

type VirtualHost struct {
	ServerName   string
	Listen       int
	Root         string
	Index        []string
	Locations    []Location
	SSL          *SSLConfig
	UpstreamName string
	ProxyPass    string
	AccessLog    string
	ErrorLog     string
	CustomConfig string
}

type Location struct {
	Path      string
	ProxyPass string
	Root      string
	TryFiles  []string
	Headers   map[string]string
}

type SSLConfig struct {
	Certificate    string
	CertificateKey string
	Protocols      []string
	Ciphers        string
}

func New() *NginxPlugin {
	return &NginxPlugin{
		name:    "nginx-manager",
		version: "1.0.0",
	}
}

func (p *NginxPlugin) Name() string    { return p.name }
func (p *NginxPlugin) Version() string { return p.version }

func (p *NginxPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityStorage}
}

func (p *NginxPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	p.config = &NginxConfig{
		ConfigDir:     "/etc/nginx",
		EnabledDir:    "/etc/nginx/sites-enabled",
		AvailableDir:  "/etc/nginx/sites-available",
		ReloadCommand: "nginx -s reload",
		TestCommand:   "nginx -t",
		AutoReload:    true,
	}

	if configDir, ok := config["config_dir"].(string); ok {
		p.config.ConfigDir = configDir
	}

	// Ensure directories exist
	os.MkdirAll(p.config.EnabledDir, 0755)
	os.MkdirAll(p.config.AvailableDir, 0755)

	return nil
}

func (p *NginxPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// CreateVirtualHost creates a new nginx virtual host configuration
func (p *NginxPlugin) CreateVirtualHost(vhost *VirtualHost) error {
	configPath := filepath.Join(p.config.AvailableDir, vhost.ServerName+".conf")

	// Generate config from template
	tmpl := template.Must(template.New("vhost").Parse(nginxVhostTemplate))

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, vhost); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	// Test configuration
	if err := p.testConfig(); err != nil {
		os.Remove(configPath)
		return fmt.Errorf("invalid config: %w", err)
	}

	return nil
}

// EnableVirtualHost enables a virtual host by creating symlink
func (p *NginxPlugin) EnableVirtualHost(serverName string) error {
	source := filepath.Join(p.config.AvailableDir, serverName+".conf")
	target := filepath.Join(p.config.EnabledDir, serverName+".conf")

	if _, err := os.Stat(source); os.IsNotExist(err) {
		return fmt.Errorf("config not found: %s", serverName)
	}

	// Remove existing symlink if any
	os.Remove(target)

	// Create symlink
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	if p.config.AutoReload {
		return p.reload()
	}

	return nil
}

// DisableVirtualHost disables a virtual host
func (p *NginxPlugin) DisableVirtualHost(serverName string) error {
	target := filepath.Join(p.config.EnabledDir, serverName+".conf")

	if err := os.Remove(target); err != nil {
		return fmt.Errorf("remove symlink: %w", err)
	}

	if p.config.AutoReload {
		return p.reload()
	}

	return nil
}

// DeleteVirtualHost deletes a virtual host configuration
func (p *NginxPlugin) DeleteVirtualHost(serverName string) error {
	// First disable it
	p.DisableVirtualHost(serverName)

	// Then delete the config
	configPath := filepath.Join(p.config.AvailableDir, serverName+".conf")
	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("delete config: %w", err)
	}

	return nil
}

func (p *NginxPlugin) testConfig() error {
	cmd := exec.Command("sh", "-c", p.config.TestCommand)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("test failed: %s", output)
	}
	return nil
}

func (p *NginxPlugin) reload() error {
	cmd := exec.Command("sh", "-c", p.config.ReloadCommand)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload failed: %s", output)
	}
	return nil
}

// CreateReverseProxy creates a reverse proxy configuration
func (p *NginxPlugin) CreateReverseProxy(serverName, upstream string, port int) error {
	vhost := &VirtualHost{
		ServerName: serverName,
		Listen:     port,
		ProxyPass:  upstream,
		Locations: []Location{
			{
				Path:      "/",
				ProxyPass: upstream,
				Headers: map[string]string{
					"Host":              "$host",
					"X-Real-IP":         "$remote_addr",
					"X-Forwarded-For":   "$proxy_add_x_forwarded_for",
					"X-Forwarded-Proto": "$scheme",
				},
			},
		},
	}

	return p.CreateVirtualHost(vhost)
}

// CreateLoadBalancer creates a load balancer configuration
func (p *NginxPlugin) CreateLoadBalancer(name string, backends []string, algorithm string) error {
	upstreamPath := filepath.Join(p.config.ConfigDir, "conf.d", name+"-upstream.conf")

	tmpl := template.Must(template.New("upstream").Parse(nginxUpstreamTemplate))

	file, err := os.Create(upstreamPath)
	if err != nil {
		return fmt.Errorf("create upstream: %w", err)
	}
	defer file.Close()

	data := map[string]interface{}{
		"Name":      name,
		"Backends":  backends,
		"Algorithm": algorithm,
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	if p.config.AutoReload {
		return p.reload()
	}

	return nil
}

const nginxVhostTemplate = `# Managed by Mandau
server {
    listen {{.Listen}}{{if .SSL}} ssl{{end}};
    server_name {{.ServerName}};

    {{if .Root}}
    root {{.Root}};
    {{if .Index}}index {{range .Index}}{{.}} {{end}};{{end}}
    {{end}}

    {{if .SSL}}
    ssl_certificate {{.SSL.Certificate}};
    ssl_certificate_key {{.SSL.CertificateKey}};
    {{if .SSL.Protocols}}
    ssl_protocols {{range .SSL.Protocols}}{{.}} {{end}};
    {{end}}
    {{if .SSL.Ciphers}}
    ssl_ciphers {{.SSL.Ciphers}};
    {{end}}
    ssl_prefer_server_ciphers on;
    {{end}}

    {{if .AccessLog}}
    access_log {{.AccessLog}};
    {{else}}
    access_log /var/log/nginx/{{.ServerName}}-access.log;
    {{end}}

    {{if .ErrorLog}}
    error_log {{.ErrorLog}};
    {{else}}
    error_log /var/log/nginx/{{.ServerName}}-error.log;
    {{end}}

    {{range .Locations}}
    location {{.Path}} {
        {{if .ProxyPass}}
        proxy_pass {{.ProxyPass}};
        {{range $key, $value := .Headers}}
        proxy_set_header {{$key}} {{$value}};
        {{end}}
        {{end}}

        {{if .Root}}
        root {{.Root}};
        {{end}}

        {{if .TryFiles}}
        try_files {{range .TryFiles}}{{.}} {{end}};
        {{end}}
    }
    {{end}}

    {{if .ProxyPass}}
    location / {
        proxy_pass {{.ProxyPass}};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    {{end}}

    {{.CustomConfig}}
}`

const nginxUpstreamTemplate = `# Managed by Mandau
upstream {{.Name}} {
    {{if eq .Algorithm "least_conn"}}least_conn;{{end}}
    {{if eq .Algorithm "ip_hash"}}ip_hash;{{end}}

    {{range .Backends}}
    server {{.}};
    {{end}}
}`
