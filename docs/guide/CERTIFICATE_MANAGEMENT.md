# Certificate Management Guide

This guide covers Mandau's certificate lifecycle, from development setup to production deployment with centralized CA management.

## Table of Contents

- [Overview](#overview)
- [Certificate Architecture](#certificate-architecture)
- [Development Setup](#development-setup)
- [Production Deployment](#production-deployment)
  - [Centralized CA Management](#centralized-ca-management)
  - [Generating Production Certificates](#generating-production-certificates)
  - [Distributing Certificates to Servers](#distributing-certificates-to-servers)
- [Certificate Distribution Script](#certificate-distribution-script)
- [Manual Certificate Generation](#manual-certificate-generation)
- [Certificate Verification](#certificate-verification)
- [Certificate Rotation](#certificate-rotation)
- [Troubleshooting](#troubleshooting)
- [Security Best Practices](#security-best-practices)

---

## Overview

Mandau uses **mutual TLS (mTLS)** for all communications between components:

- **Core Server** ↔ **Agents** (gRPC)
- **CLI Client** ↔ **Core Server** (gRPC/HTTP)
- **Agents** ↔ **Docker** (local socket)

All certificates are signed by a **Certificate Authority (CA)** that you control. In production, this CA should be generated once on a secure admin machine and used to sign certificates for all servers.

### New: CLI Certificate Management

Mandau now includes built-in certificate management commands:

```bash
# Generate certificates (auto-places in ~/.mandau/certs/)
mandau cert gen

# View certificate status
mandau cert status

# Verify certificates
mandau cert verify

# No need to specify cert paths - CLI auto-discovers from ~/.mandau/certs/
mandau agent list  # Just works!
```

The CLI automatically discovers certificates in `~/.mandau/certs/`, so you don't need to specify `--cert`, `--key`, or `--ca` flags.

---

## Certificate Architecture

### Certificate Types

```
┌─────────────────────────────────────────────────────────┐
│  Certificate Authority (CA)                             │
│  - Generated once on secure admin machine               │
│  - Signs all server and client certificates             │
│  - Valid for 10 years (3650 days)                       │
│  - MUST be kept secure - can sign new certs!            │
└─────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ Core Server  │    │ Agent Server │    │ CLI Client   │
│ Certificate  │    │ Certificate  │    │ Certificate  │
│              │    │              │    │              │
│ Valid: 1 yr  │    │ Valid: 1 yr  │    │ Valid: 1 yr  │
│ SANs:        │    │ SANs:        │    │ Usage:       │
│ - hostname   │    │ - hostname   │    │ clientAuth   │
│ - IP addr    │    │ - IP addr    │    │              │
│ - localhost  │    │ - localhost  │    │              │
└──────────────┘    └──────────────┘    └──────────────┘
```

### Certificate Purpose

| Certificate | Used By | Purpose | Permissions |
|------------|---------|---------|-------------|
| **CA** | Admin only | Sign server/client certs | N/A (signing only) |
| **Core** | Core server | Authenticate core server | serverAuth, clientAuth |
| **Agent** | Each agent | Authenticate agent servers | serverAuth, clientAuth |
| **Client** | CLI/users | Authenticate CLI users | clientAuth only |

### File Permissions

| File | Permission | Reason |
|------|-----------|--------|
| `*.key` | `600` | Private keys - owner read/write only |
| `*.crt` | `644` | Public certs - world readable |
| `ca.key` | `600` | **CRITICAL** - CA key must be protected |
| `certs/` | `700` | Certificate directory - owner only |

---

## Development Setup

For local development, use the built-in certificate generation command:

```bash
# Quick generation (places certs in ~/.mandau/certs/)
mandau cert gen

# Or using the make target (legacy)
make certs

# Verify everything is working
mandau cert status
mandau agent list
```

**Note:** The CLI automatically discovers certificates in `~/.mandau/certs/`, so no additional configuration is needed.

---

## Production Deployment

### Centralized CA Management

**IMPORTANT:** In production, generate the CA **once** on a secure admin machine, then distribute it to all servers. Do NOT generate a separate CA on each server.

#### Step 1: Generate CA on Admin Machine

```bash
# On secure admin workstation
mkdir -p ~/mandau-ca
cd ~/mandau-ca

# Generate CA (valid for 10 years)
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 -key ca.key \
  -out ca.crt \
  -subj "/CN=Mandau CA/O=Mandau/C=US"

# Secure the CA key
chmod 600 ca.key

# Backup CA to secure location
cp ca.key ca.crt /secure/backup/location/
```

**WARNING:** The `ca.key` file can sign new certificates. Store it securely!

#### Step 2: Distribute CA to All Servers

```bash
# Copy CA certificate to all servers (public cert)
scp ca.crt admin@core-server:/etc/mandau/certs/
scp ca.crt admin@agent-001:/etc/mandau/certs/
scp ca.crt admin@agent-002:/etc/mandau/certs/

# DO NOT distribute ca.key to servers (keep on admin machine only)
```

---

### Generating Production Certificates

#### Option A: Using Certificate Distribution Script (Recommended)

The `cert-distribute.sh` script automates certificate generation and distribution:

```bash
# Deploy to core server
./scripts/cert-distribute.sh \
  --type core \
  --host 192.168.1.100 \
  --agent-hostname mandau-core.example.com \
  --agent-ip 192.168.1.100

# Deploy to agent server (generates unique cert)
./scripts/cert-distribute.sh \
  --type agent \
  --host 192.168.1.101 \
  --agent-hostname agent-us-east-1 \
  --agent-ip 192.168.1.101

# Deploy to another agent
./scripts/cert-distribute.sh \
  --type agent \
  --host 192.168.1.102 \
  --agent-hostname agent-eu-west-1 \
  --agent-ip 192.168.1.102
```

**Features:**
- ✅ Generates unique certificate per agent
- ✅ Sets correct SANs (hostname + IP)
- ✅ Secure file permissions
- ✅ Automatic backup of existing certs
- ✅ SSH-based distribution
- ✅ Verification after deployment
- ✅ Dry-run mode for testing

**See:** [Certificate Distribution Script](#certificate-distribution-script) for full usage.

#### Option B: Using generate-certs.sh with Custom Hostnames

```bash
# On admin machine, generate all certs for a specific server
./scripts/generate-certs.sh ./certs-core \
  --full \
  --core-hostname mandau-core.example.com \
  --core-ip 192.168.1.100 \
  --agent-hostname agent-us-east-1 \
  --agent-ip 192.168.1.101

# Deploy manually via SCP
scp ./certs-core/* admin@core-server:/etc/mandau/certs/
scp ./certs-core/* admin@agent-001:/etc/mandau/certs/
```

#### Option C: Manual Certificate Generation

For full control, generate certificates manually:

```bash
# On admin machine with CA

# 1. Generate core server certificate
openssl genrsa -out core.key 4096
openssl req -new -key core.key -out core.csr \
  -subj "/CN=mandau-core/O=Mandau/C=US"

# Create SAN extension
cat > core.ext <<EOF
subjectAltName = DNS:mandau-core.example.com,IP:192.168.1.100
extendedKeyUsage = serverAuth,clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
EOF

# Sign with CA
openssl x509 -req -in core.csr \
  -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out core.crt \
  -days 365 -extfile core.ext

# 2. Generate agent certificate (repeat for each agent)
openssl genrsa -out agent-001.key 4096
openssl req -new -key agent-001.key -out agent-001.csr \
  -subj "/CN=agent-us-east-1/O=Mandau/C=US"

cat > agent-001.ext <<EOF
subjectAltName = DNS:agent-us-east-1,IP:192.168.1.101
extendedKeyUsage = serverAuth,clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
EOF

openssl x509 -req -in agent-001.csr \
  -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out agent-001.crt \
  -days 365 -extfile agent-001.ext

# 3. Generate CLI client certificate
openssl genrsa -out client.key 4096
openssl req -new -key client.key -out client.csr \
  -subj "/CN=admin-user/O=Mandau/C=US"

cat > client.ext <<EOF
extendedKeyUsage = clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
EOF

openssl x509 -req -in client.csr \
  -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt \
  -days 365 -extfile client.ext
```

---

### Distributing Certificates to Servers

After generation, distribute certificates to each server:

#### Manual Distribution

```bash
# Core server
scp ca.crt core.crt core.key admin@core-server:/etc/mandau/certs/
ssh admin@core-server "
  chmod 600 /etc/mandau/certs/core.key
  chmod 644 /etc/mandau/certs/core.crt /etc/mandau/certs/ca.crt
  chown -R mandau:mandau /etc/mandau/certs/
"

# Agent server
scp ca.crt agent-001.crt agent-001.key admin@agent-001:/etc/mandau/certs/
ssh admin@agent-001 "
  chmod 600 /etc/mandau/certs/agent-001.key
  chmod 644 /etc/mandau/certs/agent-001.crt /etc/mandau/certs/ca.crt
  chown -R mandau:mandau /etc/mandau/certs/
"
```

#### Using cert-distribute.sh

See [Certificate Distribution Script](#certificate-distribution-script) section below.

---

## Certificate Distribution Script

The `cert-distribute.sh` script provides automated certificate distribution via SSH.

### Prerequisites

- SSH access to target servers
- CA certificate and key on admin machine
- `openssl` installed on admin machine

### Usage Examples

#### Deploy Core Server

```bash
./scripts/cert-distribute.sh \
  --type core \
  --host 192.168.1.100 \
  --user admin \
  --agent-hostname mandau-core.example.com \
  --agent-ip 192.168.1.100
```

#### Deploy Agent Server

```bash
./scripts/cert-distribute.sh \
  --type agent \
  --host 192.168.1.101 \
  --user admin \
  --agent-hostname agent-us-east-1 \
  --agent-ip 192.168.1.101
```

#### Deploy CLI Client

```bash
./scripts/cert-distribute.sh \
  --type cli \
  --host developer-laptop.example.com \
  --user developer \
  --remote-dir /home/developer/.mandau
```

#### Batch Deployment

```bash
# Deploy to multiple agents
cat <<EOF > agents.txt
192.168.1.101 agent-us-east-1
192.168.1.102 agent-us-west-1
192.168.1.103 agent-eu-west-1
EOF

while read ip hostname; do
  ./scripts/cert-distribute.sh \
    --type agent \
    --host "$ip" \
    --agent-hostname "$hostname" \
    --agent-ip "$ip"
done < agents.txt
```

#### Dry Run (Test Mode)

```bash
./scripts/cert-distribute.sh \
  --type agent \
  --host 192.168.1.101 \
  --agent-hostname agent-test \
  --agent-ip 192.168.1.101 \
  --dry-run
```

### Script Options

| Option | Description | Default |
|--------|-------------|---------|
| `--type TYPE` | Deployment type (core/agent/cli) | Required |
| `--host HOST` | Target server hostname/IP | Required |
| `--user USER` | SSH user | `root` |
| `--port PORT` | SSH port | `22` |
| `--ssh-key PATH` | SSH private key path | `~/.ssh/id_rsa` |
| `--remote-dir PATH` | Remote cert directory | `/etc/mandau/certs` |
| `--ca-cert PATH` | CA certificate path | `./certs/ca.crt` |
| `--ca-key PATH` | CA private key path | `./certs/ca.key` |
| `--agent-hostname NAME` | Agent hostname (agent type only) | Required for agent |
| `--agent-ip IP` | Agent IP (agent type only) | Required for agent |
| `--dry-run` | Show actions without executing | `false` |
| `--no-backup` | Skip backup of existing certs | `false` |

---

## Manual Certificate Generation

### Full Certificate Generation

```bash
# Generate all certificates with custom hostnames
./scripts/generate-certs.sh ./certs \
  --full \
  --core-hostname mandau-core.example.com \
  --core-ip 192.168.1.100 \
  --agent-hostname agent-us-east-1 \
  --agent-ip 192.168.1.101
```

### Sign-Only Mode (Agent Certificates)

If you already have a CA and just need to generate agent certificates:

```bash
# Generate agent cert signed with existing CA
./scripts/generate-certs.sh ./agent-certs \
  --sign-only \
  --agent-hostname agent-eu-west-1 \
  --agent-ip 192.168.1.102
```

**Note:** CA certificate and key must exist in the target directory.

---

## Certificate Verification

### Verify Certificates Locally

```bash
# Verify core certificate
openssl verify -CAfile ca.crt core.crt

# Verify agent certificate
openssl verify -CAfile ca.crt agent.crt

# Verify client certificate
openssl verify -CAfile ca.crt client.crt
```

### View Certificate Details

```bash
# View certificate information
openssl x509 -in core.crt -text -noout

# Check SANs
openssl x509 -in core.crt -noout -ext subjectAltName

# Check certificate expiry
openssl x509 -in core.crt -noout -dates
```

### Verify Remote Deployment

```bash
# Verify certificates on remote server
ssh admin@core-server "openssl verify -CAfile /etc/mandau/certs/ca.crt /etc/mandau/certs/core.crt"

# Check certificate expiry on remote server
ssh admin@core-server "openssl x509 -in /etc/mandau/certs/core.crt -noout -dates"
```

### Test mTLS Connection

```bash
# Test core server connection
openssl s_client \
  -connect core-server:9443 \
  -cert client.crt \
  -key client.key \
  -CAfile ca.crt
```

---

## Certificate Rotation

### Automatic Rotation

Certificates are valid for 365 days. Set up monitoring to alert when certificates expire:

```bash
# Check days until expiry
openssl x509 -in core.crt -noout -checkend $((30*24*60*60))
# Returns 0 if cert expires within 30 days, 1 otherwise
```

### Manual Rotation

1. **Generate new certificates:**
   ```bash
   ./scripts/generate-certs.sh ./certs-new \
     --full \
     --core-hostname mandau-core.example.com \
     --core-ip 192.168.1.100
   ```

2. **Deploy to servers:**
   ```bash
   ./scripts/cert-distribute.sh \
     --type core \
     --host 192.168.1.100
   ```

3. **Restart services:**
   ```bash
   # On core server
   sudo systemctl restart mandau-core

   # On agent servers
   sudo systemctl restart mandau-agent
   ```

4. **Verify:**
   ```bash
   mandau agent list
   ```

---

## Troubleshooting

### Error: "tls: bad certificate"

**Cause:** Certificate not signed by expected CA or hostname mismatch.

**Fix:**
```bash
# Verify certificate is signed by correct CA
openssl verify -CAfile /etc/mandau/certs/ca.crt /etc/mandau/certs/core.crt

# Check certificate SANs match server hostname
openssl x509 -in /etc/mandau/certs/core.crt -noout -ext subjectAltName

# Check certificate CN
openssl x509 -in /etc/mandau/certs/core.crt -noout -subject
```

### Error: "certificate signed by unknown authority"

**Cause:** Client doesn't have the correct CA certificate.

**Fix:**
```bash
# Ensure CA cert is distributed to all clients
scp ca.crt user@client:~/.mandau/ca.crt

# Configure CLI to use CA cert
export MANDAU_CA=~/.mandau/ca.crt
```

### Error: "remote certificate verify failed"

**Cause:** Server cannot verify client certificate.

**Fix:**
1. Ensure client cert is signed by same CA as server
2. Check client cert has `clientAuth` extended key usage
3. Verify file permissions (600 for keys, 644 for certs)

### Agent Cannot Connect to Core

**Symptoms:**
```
dial tcp 192.168.1.100:9443: connect: connection refused
```

**Check:**
```bash
# Verify core server is running
ssh admin@core-server "sudo systemctl status mandau-core"

# Check core server logs
ssh admin@core-server "sudo journalctl -u mandau-core -n 50"

# Verify firewall allows connections
ssh admin@core-server "sudo iptables -L -n | grep 9443"

# Test port connectivity
nc -zv core-server 9443
```

### Certificate Hostname Mismatch

**Symptoms:**
```
x509: certificate is valid for localhost, not 192.168.1.100
```

**Fix:**
```bash
# Regenerate certificate with correct hostname
./scripts/generate-certs.sh ./certs \
  --sign-only \
  --core-hostname 192.168.1.100 \
  --core-ip 192.168.1.100

# Or use cert-distribute.sh
./scripts/cert-distribute.sh \
  --type core \
  --host 192.168.1.100 \
  --agent-hostname 192.168.1.100 \
  --agent-ip 192.168.1.100
```

---

## Security Best Practices

### 1. Protect the CA Key

```bash
# Store CA key on encrypted volume or hardware security module (HSM)
chmod 600 ~/mandau-ca/ca.key

# Backup to secure location (offline storage recommended)
cp ~/mandau-ca/ca.key /encrypted/backup/location/

# NEVER distribute ca.key to servers
```

### 2. Use Unique Certificates Per Server

**DON'T:** Use same certificate for all agents  
**DO:** Generate unique certificate per agent with correct hostname/IP

```bash
# Wrong - all agents share cert
scp agent.crt agent.key admin@agent-*:/etc/mandau/certs/

# Right - unique cert per agent
./scripts/cert-distribute.sh --type agent --host agent-001 --agent-hostname agent-us-east-1 --agent-ip 10.0.1.10
./scripts/cert-distribute.sh --type agent --host agent-002 --agent-hostname agent-us-west-1 --agent-ip 10.0.2.10
```

### 3. Set Correct File Permissions

```bash
# Private keys
chmod 600 *.key

# Public certificates
chmod 644 *.crt

# Certificate directory
chmod 700 /etc/mandau/certs/

# Verify permissions
ls -la /etc/mandau/certs/
```

### 4. Rotate Certificates Annually

Set up calendar reminders to rotate certificates before they expire:

```bash
# Check expiry date
openssl x509 -in core.crt -noout -enddate

# Rotate 30 days before expiry
openssl x509 -in core.crt -noout -checkend $((30*24*60*60))
```

### 5. Monitor Certificate Health

```bash
#!/bin/bash
# Monitor certificate expiry for all servers
for server in core-server agent-001 agent-002; do
  expiry=$(ssh admin@$server "openssl x509 -in /etc/mandau/certs/*.crt -noout -enddate | cut -d= -f2")
  echo "$server: $expiry"
done
```

### 6. Use Environment Variables for Production

```bash
# On core server
export MANDAU_CORE_HOSTNAME=mandau-core.example.com
export MANDAU_CORE_IP=192.168.1.100

# On agent servers
export MANDAU_AGENT_HOSTNAME=agent-us-east-1
export MANDAU_AGENT_IP=192.168.1.101
```

### 7. Audit Certificate Usage

```bash
# List all certificates and their purposes
for cert in /etc/mandau/certs/*.crt; do
  echo "=== $cert ==="
  openssl x509 -in "$cert" -noout -subject -ext subjectAltName,extendedKeyUsage
  echo ""
done
```

---

## Quick Reference

### CLI Certificate Commands (Recommended)

```bash
# Generate all certificates (development)
mandau cert gen

# Generate certificates with custom hostnames
mandau cert gen --core-hostname mandau-core.example.com --core-ip 192.168.1.100

# View certificate status and expiry
mandau cert status

# Verify certificates are valid
mandau cert verify

# Rotate all certificates
mandau cert rotate

# Migrate from old locations
mandau cert migrate
mandau cert migrate --dry-run
```

### Manual Certificate Generation (Scripts)

```bash
# Generate development certificates
make certs

# Generate production certificates
./scripts/generate-certs.sh --full \
  --core-hostname core.example.com \
  --core-ip 192.168.1.100

# Distribute to servers
./scripts/cert-distribute.sh --type core --host 192.168.1.100
./scripts/cert-distribute.sh --type agent --host 192.168.1.101 --agent-hostname agent-001 --agent-ip 192.168.1.101
```

### Certificate Verification

```bash
# Verify certificates
openssl verify -CAfile ca.crt core.crt

# Check expiry
openssl x509 -in core.crt -noout -dates

# View certificate details
openssl x509 -in core.crt -text -noout
```

### File Locations

| Component | Certificate Directory | Config Directory |
|-----------|----------------------|------------------|
| Standard (All) | `~/.mandau/certs/` | `~/.mandau/` |
| CA Materials | `~/.mandau/ca/` | N/A |
| Legacy Development | `./certs/` | N/A |
| Legacy macOS | `~/mandau-certs/` | `~/.mandau/` |
| Legacy Linux | `/etc/mandau/certs/` | `/etc/mandau/` |

**Migration:** Use `mandau cert migrate` to move from legacy locations to `~/.mandau/`.

### Default Ports

| Component | Default Port | Protocol |
|-----------|-------------|----------|
| Core Server | `9443` | gRPC + HTTP (Web Dashboard) |
| Agent Server | `9444` | gRPC |
| WebSocket | `8445` | WebSocket |
