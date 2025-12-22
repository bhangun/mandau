package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/bhangun/mandau/pkg/plugin"
	"github.com/hashicorp/vault/api"
)

type VaultPlugin struct {
	name    string
	version string
	client  *api.Client
	path    string
}

func New() *VaultPlugin {
	return &VaultPlugin{
		name:    "vault-secrets",
		version: "1.0.0",
	}
}

func (p *VaultPlugin) Name() string    { return p.name }
func (p *VaultPlugin) Version() string { return p.version }

func (p *VaultPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilitySecrets}
}

func (p *VaultPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	addr, _ := config["address"].(string)
	if addr == "" {
		addr = "http://127.0.0.1:8200"
	}

	token, _ := config["token"].(string)
	p.path, _ = config["path"].(string)
	if p.path == "" {
		p.path = "secret/data/mandau"
	}

	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = addr

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return fmt.Errorf("create vault client: %w", err)
	}

	if token != "" {
		client.SetToken(token)
	}

	p.client = client
	return nil
}

func (p *VaultPlugin) Get(ctx context.Context, key string) ([]byte, error) {
	path := fmt.Sprintf("%s/%s", p.path, key)

	secret, err := p.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("vault read: %w", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found")
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid secret format")
	}

	value, ok := data["value"].(string)
	if !ok {
		return nil, fmt.Errorf("secret value not found")
	}

	return []byte(value), nil
}

func (p *VaultPlugin) Set(ctx context.Context, key string, value []byte) error {
	path := fmt.Sprintf("%s/%s", p.path, key)

	data := map[string]interface{}{
		"data": map[string]interface{}{
			"value": string(value),
		},
	}

	_, err := p.client.Logical().WriteWithContext(ctx, path, data)
	if err != nil {
		return fmt.Errorf("vault write: %w", err)
	}

	return nil
}

func (p *VaultPlugin) Delete(ctx context.Context, key string) error {
	path := fmt.Sprintf("%s/%s", p.path, key)

	_, err := p.client.Logical().DeleteWithContext(ctx, path)
	if err != nil {
		return fmt.Errorf("vault delete: %w", err)
	}

	return nil
}

func (p *VaultPlugin) InjectEnv(ctx context.Context, env map[string]string) error {
	for k, v := range env {
		// Check if value is a secret reference: ${secret:key}
		if strings.HasPrefix(v, "${secret:") && strings.HasSuffix(v, "}") {
			secretKey := strings.TrimSuffix(strings.TrimPrefix(v, "${secret:"), "}")

			secretValue, err := p.Get(ctx, secretKey)
			if err != nil {
				return fmt.Errorf("inject secret %s: %w", secretKey, err)
			}

			env[k] = string(secretValue)
		}
	}

	return nil
}

func (p *VaultPlugin) Shutdown(ctx context.Context) error {
	return nil
}
