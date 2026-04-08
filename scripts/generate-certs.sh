#!/bin/bash
# =============================================================================
# Mandau Certificate Generation Script
# =============================================================================
# Supports both development (localhost) and production (custom hostnames/IPs)
# Provides two modes:
#   1. Full generation (CA + server certs + client cert) - for admin setup
#   2. Sign-only mode (sign CSR with existing CA) - for distributed teams
# =============================================================================

set -euo pipefail

# Default values
CERT_DIR=""
MODE="full"  # full | sign-only
CORE_HOSTNAME="localhost"
CORE_IP="127.0.0.1"
AGENT_HOSTNAME=""
AGENT_IP=""
CA_DAYS=3650
CERT_DAYS=365
KEY_SIZE=4096

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
Usage: generate-certs.sh [CERT_DIR] [OPTIONS]

Modes:
  --full                      Generate CA + all certificates (default)
  --sign-only                 Sign CSRs with existing CA (CA must exist in CERT_DIR)

Core Server Settings:
  --core-hostname HOST        Core server hostname (default: localhost)
  --core-ip IP                Core server IP address (default: 127.0.0.1)

Agent Settings:
  --agent-hostname HOST       Agent server hostname (default: localhost)
  --agent-ip IP               Agent server IP address (default: 127.0.0.1)

Certificate Settings:
  --ca-days DAYS              CA certificate validity (default: 3650)
  --cert-days DAYS            Server/client certificate validity (default: 365)
  --key-size BITS             RSA key size (default: 4096)

Examples:
  # Development (all on localhost)
  ./scripts/generate-certs.sh ./certs

  # Production - core server with public IP
  ./scripts/generate-certs.sh ./certs --full \
    --core-hostname mandau-core.example.com \
    --core-ip 192.168.1.100 \
    --agent-hostname agent-server-01 \
    --agent-ip 192.168.1.101

  # Production - sign agent cert with existing CA
  ./scripts/generate-certs.sh ./agent-certs --sign-only \
    --agent-hostname agent-server-02 \
    --agent-ip 192.168.1.102

  # Multiple agents with unique names
  ./scripts/generate-certs.sh ./certs --full \
    --core-hostname core.prod.example.com \
    --core-ip 10.0.0.10 \
    --agent-hostname agent-us-east-1 \
    --agent-ip 10.0.1.10
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
            --full)
                MODE="full"
                shift
                ;;
            --sign-only)
                MODE="sign-only"
                shift
                ;;
            --core-hostname)
                CORE_HOSTNAME="$2"
                shift 2
                ;;
            --core-ip)
                CORE_IP="$2"
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
            --ca-days)
                CA_DAYS="$2"
                shift 2
                ;;
            --cert-days)
                CERT_DAYS="$2"
                shift 2
                ;;
            --key-size)
                KEY_SIZE="$2"
                shift 2
                ;;
            -*)
                print_error "Unknown option: $1"
                show_usage
                ;;
            *)
                if [[ -z "$CERT_DIR" ]]; then
                    CERT_DIR="$1"
                fi
                shift
                ;;
        esac
    done

    # Set defaults for agent if not provided
    if [[ -z "$AGENT_HOSTNAME" ]]; then
        AGENT_HOSTNAME="$CORE_HOSTNAME"
    fi
    if [[ -z "$AGENT_IP" ]]; then
        AGENT_IP="$CORE_IP"
    fi

    # Set default certificate directory if not provided
    if [[ -z "$CERT_DIR" ]]; then
        homeDir="$HOME"
        if command -v getent >/dev/null 2>&1; then
            homeDir=$(getent passwd "$USER" | cut -d: -f6)
        fi
        CERT_DIR="$homeDir/.mandau/certs"
    fi
}

# Validate prerequisites
check_prerequisites() {
    if ! command -v openssl &> /dev/null; then
        print_error "openssl is required but not installed"
        exit 1
    fi

    if [[ "$MODE" == "sign-only" ]]; then
        if [[ ! -f "$CERT_DIR/ca.crt" ]] || [[ ! -f "$CERT_DIR/ca.key" ]]; then
            print_error "CA certificate and key must exist in $CERT_DIR for sign-only mode"
            print_info "Run with --full mode first to generate CA, or provide existing CA files"
            exit 1
        fi
    fi
}

# Generate Certificate Authority
generate_ca() {
    print_info "Generating Certificate Authority (CA)..."
    
    openssl genrsa -out "$CERT_DIR/ca.key" $KEY_SIZE
    chmod 600 "$CERT_DIR/ca.key"
    
    openssl req -new -x509 -days $CA_DAYS -key "$CERT_DIR/ca.key" \
        -out "$CERT_DIR/ca.crt" \
        -subj "/CN=Mandau CA/O=Mandau/C=US"
    
    chmod 644 "$CERT_DIR/ca.crt"
    print_success "CA certificate generated"
}

# Generate Core Server Certificate
generate_core_cert() {
    print_info "Generating Core Server certificate for ${CORE_HOSTNAME} (${CORE_IP})..."
    
    # Generate private key
    openssl genrsa -out "$CERT_DIR/core.key" $KEY_SIZE
    chmod 600 "$CERT_DIR/core.key"
    
    # Generate CSR
    openssl req -new -key "$CERT_DIR/core.key" \
        -out "$CERT_DIR/core.csr" \
        -subj "/CN=mandau-core/O=Mandau/C=US"
    
    # Create SAN extension file with actual hostnames/IPs
    cat > "$CERT_DIR/core.ext" <<EOF
subjectAltName = DNS:${CORE_HOSTNAME},DNS:localhost,IP:${CORE_IP},IP:127.0.0.1
extendedKeyUsage = serverAuth,clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
EOF

    # Sign with CA
    openssl x509 -req -in "$CERT_DIR/core.csr" \
        -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
        -CAcreateserial -out "$CERT_DIR/core.crt" \
        -days $CERT_DAYS -extfile "$CERT_DIR/core.ext"
    
    chmod 644 "$CERT_DIR/core.crt"
    print_success "Core server certificate generated"
}

# Generate Agent Certificate
generate_agent_cert() {
    print_info "Generating Agent certificate for ${AGENT_HOSTNAME} (${AGENT_IP})..."
    
    # Generate private key
    openssl genrsa -out "$CERT_DIR/agent.key" $KEY_SIZE
    chmod 600 "$CERT_DIR/agent.key"
    
    # Generate CSR with unique agent identifier
    openssl req -new -key "$CERT_DIR/agent.key" \
        -out "$CERT_DIR/agent.csr" \
        -subj "/CN=${AGENT_HOSTNAME}/O=Mandau/C=US"
    
    # Create SAN extension file
    cat > "$CERT_DIR/agent.ext" <<EOF
subjectAltName = DNS:${AGENT_HOSTNAME},DNS:mandau-agent,DNS:localhost,IP:${AGENT_IP},IP:127.0.0.1
extendedKeyUsage = serverAuth,clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
EOF

    # Sign with CA
    openssl x509 -req -in "$CERT_DIR/agent.csr" \
        -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
        -CAcreateserial -out "$CERT_DIR/agent.crt" \
        -days $CERT_DAYS -extfile "$CERT_DIR/agent.ext"
    
    chmod 644 "$CERT_DIR/agent.crt"
    print_success "Agent certificate generated"
}

# Generate CLI Client Certificate
generate_client_cert() {
    print_info "Generating CLI Client certificate..."
    
    # Generate private key
    openssl genrsa -out "$CERT_DIR/client.key" $KEY_SIZE
    chmod 600 "$CERT_DIR/client.key"
    
    # Generate CSR
    openssl req -new -key "$CERT_DIR/client.key" \
        -out "$CERT_DIR/client.csr" \
        -subj "/CN=mandau-cli/O=Mandau/C=US"
    
    # Create extension file (client auth only)
    cat > "$CERT_DIR/client.ext" <<EOF
extendedKeyUsage = clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
EOF

    # Sign with CA
    openssl x509 -req -in "$CERT_DIR/client.csr" \
        -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
        -CAcreateserial -out "$CERT_DIR/client.crt" \
        -days $CERT_DAYS -extfile "$CERT_DIR/client.ext"
    
    chmod 644 "$CERT_DIR/client.crt"
    print_success "CLI client certificate generated"
}

# Verify generated certificates
verify_certificates() {
    print_info "Verifying certificates..."
    
    local errors=0
    
    # Verify core cert
    if openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/core.crt" &>/dev/null; then
        print_success "Core certificate verification: OK"
    else
        print_error "Core certificate verification: FAILED"
        errors=$((errors + 1))
    fi
    
    # Verify agent cert
    if openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/agent.crt" &>/dev/null; then
        print_success "Agent certificate verification: OK"
    else
        print_error "Agent certificate verification: FAILED"
        errors=$((errors + 1))
    fi
    
    # Verify client cert
    if openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/client.crt" &>/dev/null; then
        print_success "Client certificate verification: OK"
    else
        print_error "Client certificate verification: FAILED"
        errors=$((errors + 1))
    fi
    
    return $errors
}

# Print certificate summary
print_summary() {
    echo ""
    print_success "Certificates generated in: $CERT_DIR"
    echo ""
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│ Certificate Summary                                         │"
    echo "├─────────────────────────────────────────────────────────────┤"
    echo "│ CA Certificate (valid $CA_DAYS days):                     │"
    echo "│   $CERT_DIR/ca.crt                                       │"
    echo "│                                                             │"
    echo "│ Core Server (valid $CERT_DAYS days):                      │"
    echo "│   Hostname: $CORE_HOSTNAME                                │"
    echo "│   IP: $CORE_IP                                            │"
    echo "│   Cert: $CERT_DIR/core.crt                                │"
    echo "│   Key:  $CERT_DIR/core.key                                │"
    echo "│                                                             │"
    echo "│ Agent Server (valid $CERT_DAYS days):                     │"
    echo "│   Hostname: $AGENT_HOSTNAME                              │"
    echo "│   IP: $AGENT_IP                                          │"
    echo "│   Cert: $CERT_DIR/agent.crt                               │"
    echo "│   Key:  $CERT_DIR/agent.key                               │"
    echo "│                                                             │"
    echo "│ CLI Client (valid $CERT_DAYS days):                       │"
    echo "│   Cert: $CERT_DIR/client.crt                              │"
    echo "│   Key:  $CERT_DIR/client.key                              │"
    echo "│                                                             │"
    echo "│ Permissions:                                                │"
    echo "│   *.key  → 600 (owner read/write only)                     │"
    echo "│   *.crt  → 644 (owner read/write, others read)             │"
    echo "│   certs/ → 700 (owner access only)                         │"
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
    print_warning "SECURITY NOTE: Keep ca.key secure! It can sign new certificates."
    print_info "Distribute ca.crt to all clients and agents for verification."
}

# Main execution
main() {
    parse_args "$@"
    
    print_info "Mandau Certificate Generation"
    print_info "Mode: $MODE"
    print_info "Output directory: $CERT_DIR"
    
    # Create directory
    mkdir -p "$CERT_DIR"
    chmod 700 "$CERT_DIR"
    
    # Validate prerequisites
    check_prerequisites
    
    # Generate based on mode
    if [[ "$MODE" == "full" ]]; then
        generate_ca
        generate_core_cert
        generate_agent_cert
        generate_client_cert
    else
        # Sign-only mode - just generate agent cert
        generate_agent_cert
    fi
    
    # Verify certificates
    if verify_certificates; then
        print_summary
    else
        print_error "Certificate verification failed!"
        exit 1
    fi
}

main "$@"
