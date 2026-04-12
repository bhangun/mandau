# Nginx Management Guide

Mandau allows you to manage Nginx virtual hosts and reverse proxies on any connected agent directly from your local CLI. This guide covers the available commands and configuration patterns.

## Commands

### List Sites
View all active and available Nginx site configurations on a specific agent.
```bash
mandau services nginx list [agent-id]
```

### Create Reverse Proxy
Quickly set up a reverse proxy for a domain. This command generates a standard Nginx configuration and enables it automatically.
```bash
mandau services nginx create-proxy [agent-id] [domain] [upstream-url] [port]
```
- **Example**: `mandau services nginx create-proxy my-agent example.com http://localhost:8080 80`

### Delete a Site
Remove a virtual host configuration from the remote agent.
```bash
mandau services nginx delete [agent-id] [site-name]
```

### Reload Nginx
Trigger a configuration test (`nginx -t`) and a reload on the remote agent. Mandau will only reload if the configuration test passes.
```bash
mandau services nginx reload [agent-id]
```

## Advanced Configuration

The Mandau Agent interacts with the following paths on the remote host:
- **Available Sites**: `/etc/nginx/sites-available/`
- **Enabled Sites**: `/etc/nginx/sites-enabled/`

### Automatic Nginx Detection
The agent automatically detects the presence of Nginx. If Nginx is not installed on the remote host, these commands will return a "not installed" error.

## Best Practices
- **Security**: Always run `reload` after deleting or modifying sites to ensure the active Nginx process reflects your changes.
- **Port Management**: When creating proxies, ensure the listener port (e.g., 80 or 443) is allowed in the agent's firewall.