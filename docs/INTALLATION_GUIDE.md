✦ Mandau Installation and Setup Guide

  Installation

  Using curl (Recommended)

   1 curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash

  Note: The sudo is required because the installation script installs binaries to /usr/local/bin/.

  Alternative Installation Methods

  Manual Installation
   1. Download the appropriate binary package for your platform from the releases page 
      (https://github.com/bhangun/mandau/releases)
   2. Extract the archive
   3. Make binaries executable and move to PATH:

   1 # Example for Linux AMD64:
   2 VERSION=v0.0.6  # Replace with the latest version
   3 wget https://github.com/bhangun/mandau/releases/download/${VERSION}/mandau-linux-amd64-${VERSION}
     .tar.gz
   4 tar -xzf mandau-linux-amd64-${VERSION}.tar.gz
   5 
   6 # Make binaries executable and move to PATH
   7 sudo chmod +x mandau mandau-core mandau-agent
   8 sudo mv mandau mandau-core mandau-agent /usr/local/bin/

  Build from Source

   1 git clone https://github.com/bhangun/mandau.git
   2 cd mandau
   3 make build
   4 sudo make install

  Post-Installation Setup

  1. Generate Certificates
  Mandau uses mTLS (mutual TLS) for secure communication. Generate certificates for Core, Agent, and CLI:

   1 # Create a directory for certificates
   2 mkdir -p ~/mandau-certs
   3 
   4 # Generate certificates (you'll need the scripts from the source)
   5 # If you have the source code, you can use:
   6 git clone https://github.com/bhangun/mandau.git
   7 cd mandau
   8 make certs
   9 cp -r certs ~/mandau-certs/

  2. Configure Mandau Core
  The Core service manages agents and provides the API endpoint.

  Start Core Service:

   1 mandau-core \
   2   --listen :8443 \
   3   --cert ~/mandau-certs/core.crt \
   4   --key ~/mandau-certs/core.key \
   5   --ca ~/mandau-certs/ca.crt

  For Production: Use systemd service files or run in Docker container.

  3. Configure Mandau Agent
  The Agent runs on each Docker host and executes commands.

  Start Agent Service:

   1 mandau-agent \
   2   --server localhost:8443 \
   3   --cert ~/mandau-certs/agent.crt \
   4   --key ~/mandau-certs/agent.key \
   5   --ca ~/mandau-certs/ca.crt \
   6   --stack-root ./stacks

  4. Configure CLI Authentication
  You can use either environment variables or command-line flags:

  Option A: Using Environment Variables

   1 export MANDAU_SERVER=localhost:8443
   2 export MANDAU_CERT=~/mandau-certs/client.crt
   3 export MANDAU_KEY=~/mandau-certs/client.key
   4 export MANDAU_CA=~/mandau-certs/ca.crt

  Option B: Using Command-Line Flags

   1 mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt
     --server localhost:8443 [command]

  Core Configuration

  Configuration File
  Create a configuration file at ~/.config/mandau/config.yaml:

   1 server: "localhost:8443"
   2 cert: "~/mandau-certs/client.crt"
   3 key: "~/mandau-certs/client.key"
   4 ca: "~/mandau-certs/ca.crt"
   5 timeout: "30s"

  Certificate Management
  Mandau uses a PKI (Public Key Infrastructure) system:

   - CA Certificate (ca.crt): Signs all other certificates
   - Core Certificate (core.crt + core.key): For the Core service
   - Agent Certificate (agent.crt + agent.key): For each Agent
   - Client Certificate (client.crt + client.key): For CLI clients

  Authentication & Security

  Certificate-Based Authentication
  Mandau uses mTLS for authentication. Each component (Core, Agent, CLI) has its own certificate signed by the
  CA.

  RBAC Configuration (Optional)
  If using the RBAC plugin, create a roles.yaml file:

    1 roles:
    2   - name: operator
    3     permissions:
    4       - resource: "stack:*"
    5         actions: ["read", "write"]
    6       - resource: "container:*"
    7         actions: ["read", "exec", "logs"]
    8 
    9 users:
   10   - id: "admin@example.com"
   11     name: "Admin User"
   12     roles: ["operator"]

  Service Management

  Using Systemd (Recommended for Production)
   1. Copy binaries and service files to appropriate locations
   2. Enable and start services:

    1 sudo cp /usr/local/bin/mandau-core /usr/local/bin/mandau-agent /usr/local/bin/mandau 
      /usr/local/bin/
    2 sudo cp mandau-core.service mandau-agent.service /etc/systemd/system/
    3 sudo systemctl daemon-reload
    4 
    5 # Enable services to start on boot
    6 sudo systemctl enable mandau-core@$(whoami)
    7 sudo systemctl enable mandau-agent@$(whoami)
    8 
    9 # Start the services
   10 sudo systemctl start mandau-core@$(whoami)
   11 sudo systemctl start mandau-agent@$(whoami)

  Using the Enhanced Runner (Development)

   1 # Clone the repository to get the runner script
   2 git clone https://github.com/bhangun/mandau.git
   3 cd mandau
   4 
   5 # Clean up any stale processes first
   6 ./run-dev.sh --clean
   7 
   8 # Start the system with enhanced reliability features
   9 ./run-dev.sh --host

  Basic Usage

  List Agents

   1 mandau agent list

  Deploy a Stack

   1 mandau stack apply agent-001 web ./compose.yaml

  Stream Logs

   1 mandau stack logs agent-001 web

  Execute Command in Container

   1 mandau container exec agent-001 web-container /bin/sh

  Manage Services

   1 # Create nginx reverse proxy
   2 mandau services nginx create-proxy agent-001 example.com http://localhost:3000 80
   3 
   4 # Start systemd service
   5 mandau services systemd start agent-001 myservice
   6 
   7 # Allow port through firewall
   8 mandau services firewall allow-port agent-001 80 tcp

  Plugin Management

   1 # Get a secret value
   2 mandau plugins secrets get my-secret
   3 
   4 # Check authentication status
   5 mandau plugins auth status
   6 
   7 # List audit logs
   8 mandau plugins audit list

  Troubleshooting

  Common Issues
   1. Connection refused: Make sure Core service is running on the specified port
   2. Certificate errors: Verify certificates are valid and properly configured
   3. Permission denied: Check file permissions and certificate paths
   4. Agent not found: Ensure Agent is properly registered with Core

  Verify Installation

   1 # Check if binaries are installed
   2 which mandau mandau-core mandau-agent
   3 
   4 # Check version
   5 mandau --help
   6 
   7 # Test connection to Core (if running)
   8 mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt
     agent list

  Log Files
   - Core logs: Check the terminal where Core is running
   - Agent logs: Check the terminal where Agent is running
   - CLI logs: Verbose output with --verbose flag

  Next Steps

   1. Set up your first stack: Create a compose.yaml file and deploy it
   2. Configure plugins: Set up authentication, secrets, and audit plugins as needed
   3. Set up monitoring: Configure health checks and monitoring for your infrastructure
   4. Production deployment: Set up systemd services or Docker containers for production use

  Your Mandau installation is now complete and ready for use!