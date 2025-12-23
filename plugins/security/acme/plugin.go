package acme

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bhangun/mandau/pkg/plugin"
)

type ACMEPlugin struct {
	name    string
	version string
	config  *ACMEConfig
}

type ACMEConfig struct {
	Email      string
	CertDir    string
	Provider   string // letsencrypt, zerossl
	Production bool
	Webroot    string
}

type Certificate struct {
	Domain    string
	CertPath  string
	KeyPath   string
	ExpiresAt string
	IssuedAt  string
	Issuer    string
}

func New() *ACMEPlugin {
	return &ACMEPlugin{
		name:    "acme-manager",
		version: "1.0.0",
	}
}

func (p *ACMEPlugin) Name() string    { return p.name }
func (p *ACMEPlugin) Version() string { return p.version }

func (p *ACMEPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilitySecurity}
}

func (p *ACMEPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	p.config = &ACMEConfig{
		Email:      plugin.GetStringConfig(config, "email"),
		CertDir:    "/etc/letsencrypt/live",
		Provider:   "letsencrypt",
		Production: false,
		Webroot:    "/var/www/html",
	}

	if prod, ok := config["production"].(bool); ok {
		p.config.Production = prod
	}

	return nil
}

func (p *ACMEPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// ObtainCertificate obtains a new SSL certificate using certbot
func (p *ACMEPlugin) ObtainCertificate(domain string) (*Certificate, error) {
	args := []string{
		"certonly",
		"--webroot",
		"-w", p.config.Webroot,
		"-d", domain,
		"--email", p.config.Email,
		"--agree-tos",
		"--non-interactive",
	}

	if !p.config.Production {
		args = append(args, "--staging")
	}

	cmd := exec.Command("certbot", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("certbot failed: %s", output)
	}

	cert := &Certificate{
		Domain:   domain,
		CertPath: filepath.Join(p.config.CertDir, domain, "fullchain.pem"),
		KeyPath:  filepath.Join(p.config.CertDir, domain, "privkey.pem"),
	}

	return cert, nil
}

// RenewCertificate renews an existing certificate
func (p *ACMEPlugin) RenewCertificate(domain string) error {
	cmd := exec.Command("certbot", "renew", "--cert-name", domain)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("renew failed: %s", output)
	}

	return nil
}

// RenewAllCertificates renews all certificates
func (p *ACMEPlugin) RenewAllCertificates() error {
	cmd := exec.Command("certbot", "renew")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("renew all failed: %s", output)
	}

	return nil
}

// RevokeCertificate revokes a certificate
func (p *ACMEPlugin) RevokeCertificate(domain string) error {
	certPath := filepath.Join(p.config.CertDir, domain, "fullchain.pem")

	cmd := exec.Command("certbot", "revoke", "--cert-path", certPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("revoke failed: %s", output)
	}

	return nil
}

// ListCertificates lists all managed certificates
func (p *ACMEPlugin) ListCertificates() ([]*Certificate, error) {
	certs := []*Certificate{}

	cmd := exec.Command("certbot", "certificates")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse certbot output
	lines := strings.Split(string(output), "\n")
	var currentCert *Certificate

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Certificate Name:") {
			if currentCert != nil {
				certs = append(certs, currentCert)
			}
			currentCert = &Certificate{
				Domain: strings.TrimSpace(strings.TrimPrefix(line, "Certificate Name:")),
			}
		} else if strings.HasPrefix(line, "Certificate Path:") && currentCert != nil {
			currentCert.CertPath = strings.TrimSpace(strings.TrimPrefix(line, "Certificate Path:"))
		} else if strings.HasPrefix(line, "Private Key Path:") && currentCert != nil {
			currentCert.KeyPath = strings.TrimSpace(strings.TrimPrefix(line, "Private Key Path:"))
		} else if strings.HasPrefix(line, "Expiry Date:") && currentCert != nil {
			currentCert.ExpiresAt = strings.TrimSpace(strings.TrimPrefix(line, "Expiry Date:"))
		}
	}

	if currentCert != nil {
		certs = append(certs, currentCert)
	}

	return certs, nil
}
