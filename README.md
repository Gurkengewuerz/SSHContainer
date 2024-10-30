# SSH Container Environment

A secure, containerized SSH environment that provides isolated Docker containers for each user session. The system
authenticates users via OAuth2 and creates dedicated containers with customizable resource limits and security settings.

## Features

- 🔒 Secure user isolation through Docker containers
- 🔑 OAuth2 authentication integration
- 💾 Persistent user storage through VFS mounting
- 🎯 Configurable resource limits (CPU, Memory, Disk Quota)
- 🛡️ Enhanced security with read-only root filesystem option
- 🔧 Customizable Docker capabilities and security options
- 🌐 Flexible network configuration with multi-network support
- 📦 Support for custom Docker images
- ⚡ PTY (pseudo-terminal) support with dynamic window resizing
- 🔄 Graceful cleanup of containers on system shutdown

## Prerequisites

- Docker
- Go 1.23 or higher
- Docker Compose v2
- Linux host system (for VFS mounting)

## Installation

1. Clone the repository:

```bash
git clone https://github.com/gurkengewuerz/sshcontainer.git
cd sshcontainer/docker
```

2. Create the required directories and files:

```bash
mkdir -p data/{server,vfs,squid}
cp ./squid.conf data/squid/squid.conf
# edit data/squid/squid.conf
```

3. Generate an SSH host key:

```bash
ssh-keygen -t rsa -f data/server/ssh_host_key -N ""
```

4. Create a `.env` file with your configuration:

```env
OAUTH2_OAUTH_ENDPOINT=http://proxy:3000
OAUTH2_CLIENT_ID=your_client_id
OAUTH2_CLIENT_SECRET=your_client_secret
QUOTA=1GB
DOCKER_MEMORY_LIMIT=512M
DOCKER_CPU_LIMIT=1.0
DOCKER_READ_ONLY=false
```

## Configuration

The system can be configured through environment variables:

| Variable                 | Description                      | Default           |
|--------------------------|----------------------------------|-------------------|
| `SSH_PORT`               | SSH server port                  | 2222              |
| `SSH_HOST_KEY`           | Path to SSH host key             | /app/ssh_host_key |
| `LOG_LEVEL`              | Log level from 0-6. 4 being Info | `4`               |
| `QUOTA`                  | Disk quota for user storage      | 1GB               |
| `OAUTH_ENDPOINT`         | OAuth2 endpoint URL              | http://proxy:3000 |
| `CLIENT_ID`              | OAuth2 client ID                 | (required)        |
| `CLIENT_SECRET`          | OAuth2 client secret             | (required)        |
| `DOCKER_IMAGE`           | Base Docker image for containers | ubuntu:latest     |
| `DOCKER_MEMORY_LIMIT`    | Container memory limit           | 512M              |
| `DOCKER_CPU_LIMIT`       | Container CPU limit              | 1.0               |
| `DOCKER_NETWORK_MODE`    | Docker network mode              | bridge            |
| `DOCKER_CAP_ADD`         | Additional Docker capabilities   | []                |
| `DOCKER_SEC_OPT`         | Docker security options          | []                |
| `DOCKER_READ_ONLY`       | Enable read-only root filesystem | false             |
| `CONTAINER_IDLE_TIMEOUT` | Container cleaup timeout         | 60                |
| `CONTAINER_CMD`          | Container exec cmd               | `/bin/bash`       |
| `CONTAINER_USER`         | Container user                   | _empty_           |

## Usage

1. Manually pull the Docker you are using in $DOCKER_IMAGE shell image. There is no mechanism implemented to
   automatically pull the image at runtime.

2. Start the services using Docker Compose inside [`docker/`](docker/):

```bash
docker-compose up -d
```

3. Connect to the SSH server:

```bash
ssh -p 2222 username@hostname
```

Users will be prompted for their OAuth2 credentials during authentication. 2FA is not supported because we are using
password authentication.

## Development

To build the project locally:

```bash
go build -o sshcontainer ./cmd/server
```

## License

This project is licensed under the AGPL - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
