# Docker Configuration

This directory contains Docker-related configuration files and scripts for the SSH Container Learning Environment. It includes Dockerfiles for both the server and shell containers, along with their respective entrypoint scripts.

## Directory Structure

```
docker/
├── Server.Dockerfile    # Server container configuration
├── Shell.Dockerfile     # User shell container configuration
└── scripts/
    ├── server/
    │   └── entrypoint.sh    # Server container initialization
    └── shell/
        └── entrypoint.sh    # Shell container initialization
```

## Components

### Server.Dockerfile
The main server container that handles SSH connections and spawns user containers.

**Key Features:**
- Built on `debian:<latest>-slim`
- Two-stage build process for minimal image size
- Includes necessary utilities for VFS management
- Configures permissions for secure operation

### Shell.Dockerfile
The container image used for individual user sessions.

**Key Features:**
- Based on Ubuntu 22.04
- Comprehensive development environment with:
    - Python 3 with pip
    - Node.js v23.1.0 (via nvm)
    - Go 1.22.1
    - Neovim (latest)
    - Common development tools
- Configurable user workspace
- Network traffic control via iptables and squid proxy

### Entrypoint Scripts

#### Server Entrypoint (`scripts/server/entrypoint.sh`)
Handles the initialization of the server container:
- Converts and validates storage quotas
- Creates and formats VFS templates
- Supports various size formats (B, KB, MB, GB, TB)
- Includes logging functionality

**Environment Variables:**
- `QUOTA`: Required. Specifies the size of user VFS (e.g., "1G", "512MB")

#### Shell Entrypoint (`scripts/shell/entrypoint.sh`)
Manages user container initialization:
- Mounts user-specific VFS
- Configures workspace permissions
- Sets up network restrictions
- Implements security policies

**Environment Variables:**
- `BLOCK_DEVICE`: VFS device path (loopback device)

## Usage

### Building the Server Image
```bash
docker build -f docker/Server.Dockerfile -t sshcontainer-server .
```

### Building the Shell Image
```bash
docker build -f docker/Shell.Dockerfile -t sshcontainer-shell .
```

### Configuration
The containers can be configured through environment variables in your `compose.yml`. See the main README for available configuration options.

## Security Considerations

### Server Container
- Runs with privileged access (required for container management)
- Minimal base image to reduce attack surface
- Strict filesystem permissions

### Shell Container
- Restricted network access
    - Only allows proxy traffic on port 3128
    - IPv6 traffic blocked by default
- Isolated user workspace
- Non-root user execution
- Read-only root filesystem option
