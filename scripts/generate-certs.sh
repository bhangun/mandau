#!/bin/bash
set -euo pipefail

CERT_DIR="${1:-./certs}"
CA_DAYS=3650
CERT_DAYS=365

mkdir -p "$CERT_DIR"

echo "Generating CA certificate..."
openssl genrsa -out "$CERT_DIR/ca.key" 4096
openssl req -new -x509 -days $CA_DAYS -key "$CERT_DIR/ca.key" \
  -out "$CERT_DIR/ca.crt" \
  -subj "/CN=Mandau CA/O=Mandau/C=US"

echo "Generating Core certificate..."
openssl genrsa -out "$CERT_DIR/core.key" 4096
openssl req -new -key "$CERT_DIR/core.key" \
  -out "$CERT_DIR/core.csr" \
  -subj "/CN=mandau-core/O=Mandau/C=US"

cat > "$CERT_DIR/core.ext" <<EOF
subjectAltName = DNS:mandau-core,DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth,clientAuth
EOF

openssl x509 -req -in "$CERT_DIR/core.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial -out "$CERT_DIR/core.crt" \
  -days $CERT_DAYS -extfile "$CERT_DIR/core.ext"

echo "Generating Agent certificate..."
openssl genrsa -out "$CERT_DIR/agent.key" 4096
openssl req -new -key "$CERT_DIR/agent.key" \
  -out "$CERT_DIR/agent.csr" \
  -subj "/CN=mandau-agent/O=Mandau/C=US"

cat > "$CERT_DIR/agent.ext" <<EOF
subjectAltName = DNS:mandau-agent,DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth,clientAuth
EOF

openssl x509 -req -in "$CERT_DIR/agent.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial -out "$CERT_DIR/agent.crt" \
  -days $CERT_DAYS -extfile "$CERT_DIR/agent.ext"

echo "Generating CLI client certificate..."
openssl genrsa -out "$CERT_DIR/client.key" 4096
openssl req -new -key "$CERT_DIR/client.key" \
  -out "$CERT_DIR/client.csr" \
  -subj "/CN=mandau-cli/O=Mandau/C=US"

cat > "$CERT_DIR/client.ext" <<EOF
extendedKeyUsage = clientAuth
EOF

openssl x509 -req -in "$CERT_DIR/client.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial -out "$CERT_DIR/client.crt" \
  -days $CERT_DAYS -extfile "$CERT_DIR/client.ext"

# Set permissions
chmod 600 "$CERT_DIR"/*.key
chmod 644 "$CERT_DIR"/*.crt

echo "Certificates generated in $CERT_DIR"
echo ""
echo "Core:"
echo "  Certificate: $CERT_DIR/core.crt"
echo "  Key: $CERT_DIR/core.key"
echo ""
echo "Agent:"
echo "  Certificate: $CERT_DIR/agent.crt"
echo "  Key: $CERT_DIR/agent.key"
echo ""
echo "CLI Client:"
echo "  Certificate: $CERT_DIR/client.crt"
echo "  Key: $CERT_DIR/client.key"
echo ""
echo "CA Certificate: $CERT_DIR/ca.crt"
