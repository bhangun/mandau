#!/bin/bash
# =============================================================================
# Mandau Certificate Distribution Script
# =============================================================================
# Securely distributes certificates to remote servers via SSH
# Supports:
#   1. Core server deployment
#   2. Agent server deployment (generates unique certs per agent)
#   3. CLI user deployment (distributes client certificates)
# =============================================================================

set -euo pipefail

# Default values
CA_CERT=""
CA_KEY=""
CERT_DIR="./certs"
REMOTE_USER="root"
REMOTE_HOST=""
REMOTE_PORT=22
REMOTE_CERT_DIR="/etc/mandau/certs"
DEPLOY_TYPE="core"  # core | agent | cli
AGENT_HOSTNAME=""
AGENT_IP=""
SSH_KEY=""
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10"
DRY_RUN=false
BACKUP_REMOTE=true

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

show_usage() {
    cat << 'EOF'
Usage: cert-distribute.sh --type TYPE --host HOST [OPTIONS]

Deployment Types:
  --type core                 Deploy core server certificates
  --type agent                Deploy agent certificates (generates unique cert)
  --type cli                  Deploy CLI client certificates

Required:
  --host HOST                 Remote server hostname or IP
  --type TYPE                 Deployment type (core/agent/cli)

Agent Type Options:
  --agent-hostname NAME       Agent hostname for certificate CN (required for agent type)
  --agent-ip IP               Agent IP address (required for agent type)

Connection Options:
  --user USER                 Remote SSH user (default: root)
  --port PORT                 Remote SSH port (default: 22)
  --ssh-key PATH              SSH private key path (default: ~/.ssh/id_rsa)
  --remote-dir PATH           Remote certificate directory (default: /etc/mandau/certs)

CA Options:
  --ca-cert PATH              CA certificate path (default: $CERT_DIR/ca.crt)
  --ca-key PATH               CA private key path (required for agent type)
  --cert-dir PATH             Local certificate directory (default: ./certs)

Other Options:
  --dry-run                   Show what would be done without executing
  --no-backup                 Don't backup existing certificates
  --help                      Show this help message

Examples:
  # Deploy core server certificates
  ./scripts/cert-distribute.sh --type core --host 192.168.1.100

  # Deploy agent with unique hostname
  ./scripts/cert-distribute.sh --type agent \
    --host 192.168.1.101 \
    --agent-hostname agent-us-east-1 \
    --agent-ip 192.168.1.101

  # Deploy to multiple agents from a hosts file
  cat agents.txt | while read host; do
    ./scripts/cert-distribute.sh --type agent \
      --host "$host" \
      --agent-hostname "agent-$(echo $host | tr '.' '-')" \
      --agent-ip "$host"
  done

  # Deploy CLI client cert to user's home directory
  ./scripts/cert-distribute.sh --type cli \
    --host developer-laptop \
    --user developer \
    --remote-dir /home/developer/.mandau

  # Dry run to test configuration
  ./scripts/cert-distribute.sh --type core --host 192.168.1.100 --dry-run
EOF
    exit 0
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                show_usage
                ;;
            --type)
                DEPLOY_TYPE="$2"
                if [[ ! "$DEPLOY_TYPE" =~ ^(core|agent|cli)$ ]]; then
                    print_error "Invalid deployment type: $DEPLOY_TYPE (must be core, agent, or cli)"
                    exit 1
                fi
                shift 2
                ;;
            --host)
                REMOTE_HOST="$2"
                shift 2
                ;;
            --agent-hostname)
                AGENT_HOSTNAME="$2"
                shift 2
                ;;
            --agent-ip)
                AGENT_IP="$2"
                shift 2
                ;;
            --user)
                REMOTE_USER="$2"
                shift 2
                ;;
            --port)
                REMOTE_PORT="$2"
                shift 2
                ;;
            --ssh-key)
                SSH_KEY="$2"
                shift 2
                ;;
            --remote-dir)
                REMOTE_CERT_DIR="$2"
                shift 2
                ;;
            --ca-cert)
                CA_CERT="$2"
                shift 2
                ;;
            --ca-key)
                CA_KEY="$2"
                shift 2
                ;;
            --cert-dir)
                CERT_DIR="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --no-backup)
                BACKUP_REMOTE=false
                shift
                ;;
            -*)
                print_error "Unknown option: $1"
                show_usage
                ;;
            *)
                print_error "Unexpected argument: $1"
                show_usage
                ;;
        esac
    done

    # Validate required arguments
    if [[ -z "$REMOTE_HOST" ]]; then
        print_error "--host is required"
        show_usage
    fi

    # Set defaults
    if [[ -z "$CA_CERT" ]]; then
        CA_CERT="$CERT_DIR/ca.crt"
    fi
    if [[ -z "$CA_KEY" ]]; then
        CA_KEY="$CERT_DIR/ca.key"
    fi
    if [[ -n "$SSH_KEY" ]]; then
        SSH_OPTS="$SSH_OPTS -i $SSH_KEY"
    fi

    # Validate agent type requirements
    if [[ "$DEPLOY_TYPE" == "agent" ]]; then
        if [[ -z "$AGENT_HOSTNAME" ]] || [[ -z "$AGENT_IP" ]]; then
            print_error "--agent-hostname and --agent-ip are required for agent deployment type"
            exit 1
        fi
    fi
}

# Validate local prerequisites
validate_local() {
    # Check CA certificate exists
    if [[ ! -f "$CA_CERT" ]]; then
        print_error "CA certificate not found: $CA_CERT"
        print_info "Run generate-certs.sh first or specify --ca-cert"
        exit 1
    fi

    # For agent type, we need CA key to generate new certs
    if [[ "$DEPLOY_TYPE" == "agent" ]]; then
        if [[ ! -f "$CA_KEY" ]]; then
            print_error "CA private key required for agent deployment: $CA_KEY"
            print_info "CA key is needed to generate unique agent certificates"
            exit 1
        fi
    fi

    # Check SSH key if specified
    if [[ -n "$SSH_KEY" ]] && [[ ! -f "$SSH_KEY" ]]; then
        print_error "SSH key not found: $SSH_KEY"
        exit 1
    fi
}

# Test SSH connectivity
test_ssh_connection() {
    print_info "Testing SSH connection to $REMOTE_USER@$REMOTE_HOST:$REMOTE_PORT..."
    
    if ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "echo 'SSH connection successful'" &>/dev/null; then
        print_success "SSH connection established"
        return 0
    else
        print_error "Failed to connect to $REMOTE_USER@$REMOTE_HOST:$REMOTE_PORT"
        print_info "Check SSH service, firewall rules, and authentication"
        exit 1
    fi
}

# Backup existing remote certificates
backup_remote_certs() {
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would backup existing certificates to ${REMOTE_CERT_DIR}.backup"
        return 0
    fi

    print_info "Backing up existing certificates on remote server..."
    
    ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        if [ -d '$REMOTE_CERT_DIR' ]; then
            BACKUP_DIR='${REMOTE_CERT_DIR}.backup.\$(date +%Y%m%d_%H%M%S)'
            cp -r '$REMOTE_CERT_DIR' \"\$BACKUP_DIR\"
            echo \"Backup created: \$BACKUP_DIR\"
        else
            echo 'No existing certificates to backup'
        fi
    "
}

# Generate agent-specific certificate
generate_agent_cert_for_host() {
    local temp_dir
    temp_dir=$(mktemp -d)
    
    print_info "Generating unique certificate for agent: $AGENT_HOSTNAME ($AGENT_IP)"
    
    # Copy CA to temp dir
    cp "$CA_CERT" "$temp_dir/ca.crt"
    cp "$CA_KEY" "$temp_dir/ca.key"
    
    # Generate agent key
    openssl genrsa -out "$temp_dir/agent.key" 4096 2>/dev/null
    chmod 600 "$temp_dir/agent.key"
    
    # Generate CSR
    openssl req -new -key "$temp_dir/agent.key" \
        -out "$temp_dir/agent.csr" \
        -subj "/CN=${AGENT_HOSTNAME}/O=Mandau/C=US"
    
    # Create SAN extension
    cat > "$temp_dir/agent.ext" <<EOF
subjectAltName = DNS:${AGENT_HOSTNAME},DNS:mandau-agent,IP:${AGENT_IP}
extendedKeyUsage = serverAuth,clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
EOF

    # Sign with CA
    openssl x509 -req -in "$temp_dir/agent.csr" \
        -CA "$temp_dir/ca.crt" -CAkey "$temp_dir/ca.key" \
        -CAcreateserial -out "$temp_dir/agent.crt" \
        -days 365 -extfile "$temp_dir/agent.ext" 2>/dev/null
    
    chmod 644 "$temp_dir/agent.crt"
    
    # Verify
    if openssl verify -CAfile "$temp_dir/ca.crt" "$temp_dir/agent.crt" &>/dev/null; then
        print_success "Agent certificate generated and verified"
        AGENT_CERT_DIR="$temp_dir"
        return 0
    else
        print_error "Agent certificate verification failed"
        rm -rf "$temp_dir"
        exit 1
    fi
}

# Deploy core server certificates
deploy_core() {
    print_info "Deploying core server certificates to $REMOTE_HOST..."
    
    local src_dir="$CERT_DIR"
    
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would create directory: $REMOTE_CERT_DIR"
        print_info "[DRY RUN] Would upload: ca.crt, core.crt, core.key"
        return 0
    fi
    
    # Create remote directory
    ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        mkdir -p '$REMOTE_CERT_DIR'
        chmod 700 '$REMOTE_CERT_DIR'
    "
    
    # Upload certificates
    scp $SSH_OPTS -P "$REMOTE_PORT" "$src_dir/ca.crt" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    scp $SSH_OPTS -P "$REMOTE_PORT" "$src_dir/core.crt" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    scp $SSH_OPTS -P "$REMOTE_PORT" "$src_dir/core.key" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    
    # Set permissions
    ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        chmod 600 '$REMOTE_CERT_DIR/core.key'
        chmod 644 '$REMOTE_CERT_DIR/core.crt' '$REMOTE_CERT_DIR/ca.crt'
        chown -R $REMOTE_USER:$REMOTE_USER '$REMOTE_CERT_DIR'
    "
    
    print_success "Core server certificates deployed"
}

# Deploy agent certificates
deploy_agent() {
    print_info "Deploying agent certificates to $REMOTE_HOST..."
    
    # Generate unique certificate for this host
    generate_agent_cert_for_host
    
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would create directory: $REMOTE_CERT_DIR"
        print_info "[DRY RUN] Would upload: ca.crt, agent.crt, agent.key"
        print_info "[DRY RUN] Agent hostname: $AGENT_HOSTNAME"
        print_info "[DRY RUN] Agent IP: $AGENT_IP"
        rm -rf "$AGENT_CERT_DIR"
        return 0
    fi
    
    # Create remote directory
    ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        mkdir -p '$REMOTE_CERT_DIR'
        chmod 700 '$REMOTE_CERT_DIR'
    "
    
    # Upload certificates
    scp $SSH_OPTS -P "$REMOTE_PORT" "$AGENT_CERT_DIR/ca.crt" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    scp $SSH_OPTS -P "$REMOTE_PORT" "$AGENT_CERT_DIR/agent.crt" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    scp $SSH_OPTS -P "$REMOTE_PORT" "$AGENT_CERT_DIR/agent.key" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    
    # Set permissions
    ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        chmod 600 '$REMOTE_CERT_DIR/agent.key'
        chmod 644 '$REMOTE_CERT_DIR/agent.crt' '$REMOTE_CERT_DIR/ca.crt'
        chown -R $REMOTE_USER:$REMOTE_USER '$REMOTE_CERT_DIR'
    "
    
    # Cleanup temp dir
    rm -rf "$AGENT_CERT_DIR"
    
    print_success "Agent certificates deployed with unique identity: $AGENT_HOSTNAME"
}

# Deploy CLI client certificates
deploy_cli() {
    print_info "Deploying CLI client certificates to $REMOTE_HOST..."
    
    local src_dir="$CERT_DIR"
    
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would create directory: $REMOTE_CERT_DIR"
        print_info "[DRY RUN] Would upload: ca.crt, client.crt, client.key"
        return 0
    fi
    
    # Create remote directory
    ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        mkdir -p '$REMOTE_CERT_DIR'
        chmod 700 '$REMOTE_CERT_DIR'
    "
    
    # Upload certificates
    scp $SSH_OPTS -P "$REMOTE_PORT" "$src_dir/ca.crt" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    scp $SSH_OPTS -P "$REMOTE_PORT" "$src_dir/client.crt" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    scp $SSH_OPTS -P "$REMOTE_PORT" "$src_dir/client.key" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_CERT_DIR/"
    
    # Set permissions
    ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        chmod 600 '$REMOTE_CERT_DIR/client.key'
        chmod 644 '$REMOTE_CERT_DIR/client.crt' '$REMOTE_CERT_DIR/ca.crt'
        chown -R $REMOTE_USER:$REMOTE_USER '$REMOTE_CERT_DIR'
    "
    
    # Create basic config file
    ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        cat > '$REMOTE_CERT_DIR/config.yaml' <<EOCONFIG
# Mandau CLI Configuration
server:
  listen_addr: \"$REMOTE_HOST:9443\"
  tls:
    cert_path: \"$REMOTE_CERT_DIR/client.crt\"
    key_path: \"$REMOTE_CERT_DIR/client.key\"
    ca_path: \"$REMOTE_CERT_DIR/ca.crt\"
    min_version: \"TLS1.3\"
    server_name: \"mandau-core\"
timeout: \"30s\"
EOCONFIG
        chmod 600 '$REMOTE_CERT_DIR/config.yaml'
        chown $REMOTE_USER:$REMOTE_USER '$REMOTE_CERT_DIR/config.yaml'
    "
    
    print_success "CLI client certificates deployed"
}

# Verify remote deployment
verify_remote() {
    print_info "Verifying remote certificate deployment..."
    
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would verify certificates on remote server"
        return 0
    fi
    
    local cert_file
    if [[ "$DEPLOY_TYPE" == "core" ]]; then
        cert_file="core.crt"
    elif [[ "$DEPLOY_TYPE" == "agent" ]]; then
        cert_file="agent.crt"
    else
        cert_file="client.crt"
    fi
    
    # Verify on remote server
    if ssh $SSH_OPTS -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "
        openssl verify -CAfile '$REMOTE_CERT_DIR/ca.crt' '$REMOTE_CERT_DIR/$cert_file'
    " &>/dev/null; then
        print_success "Remote certificate verification: OK"
    else
        print_warning "Remote certificate verification: FAILED (this may be expected if using different CA)"
    fi
}

# Print deployment summary
print_deployment_summary() {
    echo ""
    print_success "Certificate Deployment Complete"
    echo ""
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│ Deployment Summary                                          │"
    echo "├─────────────────────────────────────────────────────────────┤"
    echo "│ Type:        $DEPLOY_TYPE                                  │"
    echo "│ Host:        $REMOTE_HOST                                  │"
    echo "│ User:        $REMOTE_USER                                  │"
    echo "│ Remote Dir:  $REMOTE_CERT_DIR                             │"
    echo "│                                                             │"
    if [[ "$DEPLOY_TYPE" == "agent" ]]; then
        echo "│ Agent Host:  $AGENT_HOSTNAME                           │"
        echo "│ Agent IP:    $AGENT_IP                                │"
    fi
    echo "│                                                             │"
    echo "│ Next Steps:                                                 │"
    if [[ "$DEPLOY_TYPE" == "core" ]]; then
        echo "│ 1. Start core server:                                   │"
        echo "│    mandau-core --cert $REMOTE_CERT_DIR/core.crt \\     │"
        echo "│      --key $REMOTE_CERT_DIR/core.key \\                │"
        echo "│      --ca $REMOTE_CERT_DIR/ca.crt                      │"
        echo "│ 2. Update firewall to allow agent connections           │"
    elif [[ "$DEPLOY_TYPE" == "agent" ]]; then
        echo "│ 1. Start agent:                                         │"
        echo "│    mandau-agent --server <CORE_IP>:9443 \\             │"
        echo "│      --cert $REMOTE_CERT_DIR/agent.crt \\              │"
        echo "│      --key $REMOTE_CERT_DIR/agent.key \\               │"
        echo "│      --ca $REMOTE_CERT_DIR/ca.crt                      │"
        echo "│ 2. Verify agent registered: mandau agent list           │"
    else
        echo "│ 1. Configure CLI:                                       │"
        echo "│    export MANDAU_CONFIG_PATH=$REMOTE_CERT_DIR/config.yaml"
        echo "│ 2. Test connection: mandau agent list                   │"
    fi
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
}

# Main execution
main() {
    parse_args "$@"
    
    print_info "Mandau Certificate Distribution"
    print_info "Type: $DEPLOY_TYPE"
    print_info "Target: $REMOTE_USER@$REMOTE_HOST:$REMOTE_PORT"
    
    # Validate local setup
    validate_local
    
    # Test SSH connectivity
    test_ssh_connection
    
    # Backup existing certs if requested
    if [[ "$BACKUP_REMOTE" == true ]]; then
        backup_remote_certs
    fi
    
    # Deploy based on type
    case $DEPLOY_TYPE in
        core)
            deploy_core
            ;;
        agent)
            deploy_agent
            ;;
        cli)
            deploy_cli
            ;;
    esac
    
    # Verify deployment
    verify_remote
    
    # Print summary
    print_deployment_summary
}

main "$@"
