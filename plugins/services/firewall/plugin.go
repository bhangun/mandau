package firewall

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bhangun/mandau/pkg/plugin"
)

type FirewallPlugin struct {
	name    string
	version string
	config  *FirewallConfig
	backend string // ufw or iptables
}

type FirewallConfig struct {
	Backend       string
	DefaultPolicy string
}

type FirewallRule struct {
	Action   string // allow, deny, reject
	Proto    string // tcp, udp, any
	FromIP   string
	FromPort int
	ToIP     string
	ToPort   int
	Comment  string
}

func New() *FirewallPlugin {
	return &FirewallPlugin{
		name:    "firewall-manager",
		version: "1.0.0",
	}
}

func (p *FirewallPlugin) Name() string    { return p.name }
func (p *FirewallPlugin) Version() string { return p.version }

func (p *FirewallPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityStorage}
}

func (p *FirewallPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	backend := "ufw"
	if b, ok := config["backend"].(string); ok {
		backend = b
	}

	p.config = &FirewallConfig{
		Backend:       backend,
		DefaultPolicy: "deny",
	}

	// Detect available backend
	if _, err := exec.LookPath("ufw"); err == nil {
		p.backend = "ufw"
	} else if _, err := exec.LookPath("iptables"); err == nil {
		p.backend = "iptables"
	} else {
		return fmt.Errorf("no firewall backend found")
	}

	return nil
}

func (p *FirewallPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// AddRule adds a firewall rule
func (p *FirewallPlugin) AddRule(rule *FirewallRule) error {
	if p.backend == "ufw" {
		return p.addRuleUFW(rule)
	}
	return p.addRuleIPTables(rule)
}

func (p *FirewallPlugin) addRuleUFW(rule *FirewallRule) error {
	args := []string{rule.Action}

	if rule.Proto != "" && rule.Proto != "any" {
		args = append(args, "proto", rule.Proto)
	}

	if rule.FromIP != "" {
		args = append(args, "from", rule.FromIP)
	}

	if rule.FromPort > 0 {
		args = append(args, "port", strconv.Itoa(rule.FromPort))
	}

	if rule.ToIP != "" {
		args = append(args, "to", rule.ToIP)
	}

	if rule.ToPort > 0 {
		args = append(args, "port", strconv.Itoa(rule.ToPort))
	}

	if rule.Comment != "" {
		args = append(args, "comment", rule.Comment)
	}

	cmd := exec.Command("ufw", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ufw failed: %s", output)
	}

	return nil
}

func (p *FirewallPlugin) addRuleIPTables(rule *FirewallRule) error {
	chain := "INPUT"
	args := []string{"-A", chain}

	if rule.Proto != "" && rule.Proto != "any" {
		args = append(args, "-p", rule.Proto)
	}

	if rule.FromIP != "" {
		args = append(args, "-s", rule.FromIP)
	}

	if rule.ToIP != "" {
		args = append(args, "-d", rule.ToIP)
	}

	if rule.ToPort > 0 {
		args = append(args, "--dport", strconv.Itoa(rule.ToPort))
	}

	target := strings.ToUpper(rule.Action)
	args = append(args, "-j", target)

	cmd := exec.Command("iptables", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables failed: %s", output)
	}

	return nil
}

// DeleteRule deletes a firewall rule
func (p *FirewallPlugin) DeleteRule(ruleNumber int) error {
	if p.backend == "ufw" {
		cmd := exec.Command("ufw", "delete", strconv.Itoa(ruleNumber))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("delete failed: %s", output)
		}
	} else {
		cmd := exec.Command("iptables", "-D", "INPUT", strconv.Itoa(ruleNumber))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("delete failed: %s", output)
		}
	}

	return nil
}

// AllowPort is a convenience method to allow a port
func (p *FirewallPlugin) AllowPort(port int, proto string) error {
	return p.AddRule(&FirewallRule{
		Action: "allow",
		Proto:  proto,
		ToPort: port,
	})
}

// DenyPort is a convenience method to deny a port
func (p *FirewallPlugin) DenyPort(port int, proto string) error {
	return p.AddRule(&FirewallRule{
		Action: "deny",
		Proto:  proto,
		ToPort: port,
	})
}

// Enable enables the firewall
func (p *FirewallPlugin) Enable() error {
	if p.backend == "ufw" {
		cmd := exec.Command("ufw", "--force", "enable")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("enable failed: %s", output)
		}
	}
	return nil
}

// Disable disables the firewall
func (p *FirewallPlugin) Disable() error {
	if p.backend == "ufw" {
		cmd := exec.Command("ufw", "disable")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("disable failed: %s", output)
		}
	}
	return nil
}

// ListRules lists all firewall rules
func (p *FirewallPlugin) ListRules() ([]string, error) {
	var cmd *exec.Cmd

	if p.backend == "ufw" {
		cmd = exec.Command("ufw", "status", "numbered")
	} else {
		cmd = exec.Command("iptables", "-L", "-n", "--line-numbers")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	return lines, nil
}
