package dns

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/bhangun/mandau/pkg/plugin"
)

type DNSPlugin struct {
	name    string
	version string
	config  *DNSConfig
}

type DNSConfig struct {
	ZoneDir   string
	NamedConf string
	ReloadCmd string
}

type DNSZone struct {
	Domain string
	TTL    int
	SOA    SOARecord
	NS     []string
	A      []ARecord
	AAAA   []AAAARecord
	CNAME  []CNAMERecord
	MX     []MXRecord
	TXT    []TXTRecord
}

type SOARecord struct {
	Primary    string
	Admin      string
	Serial     int
	Refresh    int
	Retry      int
	Expire     int
	MinimumTTL int
}

type ARecord struct {
	Name string
	IP   string
	TTL  int
}

type AAAARecord struct {
	Name string
	IP   string
	TTL  int
}

type CNAMERecord struct {
	Name   string
	Target string
	TTL    int
}

type MXRecord struct {
	Priority int
	Host     string
	TTL      int
}

type TXTRecord struct {
	Name  string
	Value string
	TTL   int
}

func New() *DNSPlugin {
	return &DNSPlugin{
		name:    "dns-manager",
		version: "1.0.0",
	}
}

func (p *DNSPlugin) Name() string    { return p.name }
func (p *DNSPlugin) Version() string { return p.version }

func (p *DNSPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityStorage}
}

func (p *DNSPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	p.config = &DNSConfig{
		ZoneDir:   "/etc/bind/zones",
		NamedConf: "/etc/bind/named.conf.local",
		ReloadCmd: "rndc reload",
	}

	os.MkdirAll(p.config.ZoneDir, 0755)

	return nil
}

func (p *DNSPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// CreateZone creates a DNS zone file
func (p *DNSPlugin) CreateZone(zone *DNSZone) error {
	zoneFile := filepath.Join(p.config.ZoneDir, "db."+zone.Domain)

	tmpl := template.Must(template.New("zone").Parse(dnsZoneTemplate))

	file, err := os.Create(zoneFile)
	if err != nil {
		return fmt.Errorf("create zone file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, zone); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	// Add zone to named.conf
	if err := p.addZoneConfig(zone.Domain, zoneFile); err != nil {
		return fmt.Errorf("add zone config: %w", err)
	}

	return p.reloadDNS()
}

func (p *DNSPlugin) addZoneConfig(domain, zoneFile string) error {
	config := fmt.Sprintf(`
zone "%s" {
    type master;
    file "%s";
};
`, domain, zoneFile)

	f, err := os.OpenFile(p.config.NamedConf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(config)
	return err
}

func (p *DNSPlugin) reloadDNS() error {
	cmd := exec.Command("sh", "-c", p.config.ReloadCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload failed: %s", output)
	}
	return nil
}

// AddARecord adds an A record to a zone
func (p *DNSPlugin) AddARecord(domain, name, ip string, ttl int) error {
	// Read existing zone
	zoneFile := filepath.Join(p.config.ZoneDir, "db."+domain)

	content, err := os.ReadFile(zoneFile)
	if err != nil {
		return err
	}

	// Append new record
	record := fmt.Sprintf("%s\t%d\tIN\tA\t%s\n", name, ttl, ip)
	content = append(content, []byte(record)...)

	// Increment serial
	// (simplified - in production would parse and increment properly)

	if err := os.WriteFile(zoneFile, content, 0644); err != nil {
		return err
	}

	return p.reloadDNS()
}

// AddCNAMERecord adds a CNAME record
func (p *DNSPlugin) AddCNAMERecord(domain, name, target string, ttl int) error {
	zoneFile := filepath.Join(p.config.ZoneDir, "db."+domain)

	content, err := os.ReadFile(zoneFile)
	if err != nil {
		return err
	}

	record := fmt.Sprintf("%s\t%d\tIN\tCNAME\t%s.\n", name, ttl, target)
	content = append(content, []byte(record)...)

	if err := os.WriteFile(zoneFile, content, 0644); err != nil {
		return err
	}

	return p.reloadDNS()
}

const dnsZoneTemplate = `; Managed by Mandau
$TTL {{.TTL}}
@   IN  SOA {{.SOA.Primary}}. {{.SOA.Admin}}. (
    {{.SOA.Serial}}     ; Serial
    {{.SOA.Refresh}}    ; Refresh
    {{.SOA.Retry}}      ; Retry
    {{.SOA.Expire}}     ; Expire
    {{.SOA.MinimumTTL}} ; Minimum TTL
)

; Name servers
{{range .NS}}
@   IN  NS  {{.}}.
{{end}}

; A records
{{range .A}}
{{.Name}}   {{if .TTL}}{{.TTL}}{{else}}{{$.TTL}}{{end}}  IN  A   {{.IP}}
{{end}}

; AAAA records
{{range .AAAA}}
{{.Name}}   {{if .TTL}}{{.TTL}}{{else}}{{$.TTL}}{{end}}  IN  AAAA    {{.IP}}
{{end}}

; CNAME records
{{range .CNAME}}
{{.Name}}   {{if .TTL}}{{.TTL}}{{else}}{{$.TTL}}{{end}}  IN  CNAME   {{.Target}}.
{{end}}

; MX records
{{range .MX}}
@   {{if .TTL}}{{.TTL}}{{else}}{{$.TTL}}{{end}}  IN  MX  {{.Priority}} {{.Host}}.
{{end}}

; TXT records
{{range .TXT}}
{{.Name}}   {{if .TTL}}{{.TTL}}{{else}}{{$.TTL}}{{end}}  IN  TXT "{{.Value}}"
{{end}}`
