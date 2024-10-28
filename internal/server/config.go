package server

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	// SSH Configuration
	SSHPort    string `envconfig:"SSH_PORT" default:"2222"`
	SSHHostKey string `envconfig:"SSH_HOST_KEY" default:"/app/ssh_host_key"`
	LogLevel   int    `envconfig:"LOG_LEVEL" default:"4"`

	// OAuth Configuration
	OAuthEndpoint string `envconfig:"OAUTH_ENDPOINT" default:"http://proxy:3000"`
	ClientID      string `envconfig:"CLIENT_ID" required:"true"`
	ClientSecret  string `envconfig:"CLIENT_SECRET" required:"true"`

	// Docker Configuration
	DockerImage       string   `envconfig:"DOCKER_IMAGE" default:"ubuntu:latest"`
	MemoryLimit       string   `envconfig:"DOKCER_MEMORY_LIMIT" default:"512M"`
	CPULimit          float64  `envconfig:"DOCKER_CPU_LIMIT" default:"1.0"`
	NetworkMode       string   `envconfig:"DOCKER_NETWORK_MODE" default:"bridge"`
	Networks          []string `envconfig:"DOCKER_NETWORKS" default:""`
	DockerDevices     []string `envconfig:"DOCKER_DEVICES" default:""`
	DockerCapAdd      []string `envconfig:"DOCKER_CAP_ADD" default:""`
	DockerSecurityOpt []string `envconfig:"DOCKER_SEC_OPT" default:""`
	DockerReadOnly    bool     `envconfig:"DOCKER_READ_ONLY" default:"false"`

	// Parsed values
	memoryLimitBytes int64
	cpuLimitNano     int64
}

func LoadConfig() (*Config, error) {
	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	// Parse memory limit
	memLimit, err := parseMemoryString(config.MemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid memory limit: %w", err)
	}
	config.memoryLimitBytes = memLimit

	// Convert CPU limit to nano CPUs
	config.cpuLimitNano = int64(config.CPULimit * 1000000000)

	return &config, nil
}

func parseMemoryString(val string) (int64, error) {
	var multiplier int64 = 1
	val = strings.TrimSpace(val)

	if strings.HasSuffix(val, "G") {
		multiplier = 1024 * 1024 * 1024
		val = strings.TrimSuffix(val, "G")
	} else if strings.HasSuffix(val, "M") {
		multiplier = 1024 * 1024
		val = strings.TrimSuffix(val, "M")
	} else if strings.HasSuffix(val, "K") {
		multiplier = 1024
		val = strings.TrimSuffix(val, "K")
	}

	var result int64
	if _, err := fmt.Sscanf(val, "%d", &result); err != nil {
		return 0, fmt.Errorf("invalid memory value format: %s", val)
	}

	return result * multiplier, nil
}
