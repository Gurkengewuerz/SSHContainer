package server

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
)

type ContainerConfig struct {
	Image   string
	Cmd     []string
	Env     []string
	IsPty   bool
	PtyRows uint16
	PtyCols uint16
	User    string
}

type ContainerManager struct {
	client *client.Client
	config *Config
	log    *logrus.Logger
}

func NewContainerManager(config *Config, log *logrus.Logger) (*ContainerManager, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	containerId, err := getCurrentContainerId()
	if err != nil {
		return nil, fmt.Errorf("failed to get current container ID: %v", err)
	}

	ctx := context.Background()
	container, err := dockerClient.ContainerInspect(ctx, containerId)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %v", err)
	}

	if len(container.NetworkSettings.Networks) == 0 && len(config.Networks) == 0 {
		return nil, fmt.Errorf("no network settings found")
	}

	for networkName := range container.NetworkSettings.Networks {
		if len(container.NetworkSettings.Networks) == 1 || strings.HasSuffix(networkName, "_default") {
			config.Networks = append(config.Networks, networkName)
		}
	}

	return &ContainerManager{
		client: dockerClient,
		config: config,
		log:    log,
	}, nil
}

func (cm *ContainerManager) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	env := cfg.Env
	devices := cm.config.DockerDevices
	capAdd := cm.config.DockerCapAdd
	secOpt := cm.config.DockerSecurityOpt

	blockDevice, err := cm.CreateVFSMount(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create VFS mount: %w", err)
	}
	// Used to pass the block device to the container
	env = append(env, fmt.Sprintf("BLOCK_DEVICE=%s", blockDevice))
	devices = append(devices, blockDevice)
	devices = append(devices, "/dev/loop-control")
	capAdd = append(capAdd, "SYS_ADMIN")

	containerConfig := &container.Config{
		Image:        cfg.Image,
		Cmd:          cfg.Cmd,
		Env:          env,
		Tty:          cfg.IsPty,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		StdinOnce:    true,
		Labels: map[string]string{
			"de.mc8051.sshcontainer":      "true",
			"de.mc8051.sshcontainer.user": cfg.User,
		},
	}

	containerFields := logrus.Fields{
		"image":       cfg.Image,
		"user":        cfg.User,
		"networkMode": cm.config.NetworkMode,
		"networks":    cm.config.Networks,
		"devices":     devices,
		"capAdd":      capAdd,
		"secOpt":      secOpt,
	}

	cm.log.WithFields(containerFields).Debug("Creating container")

	var devMappings []container.DeviceMapping
	for _, dev := range devices {
		devMappings = append(devMappings, container.DeviceMapping{
			PathOnHost:        dev,
			PathInContainer:   dev,
			CgroupPermissions: "rwm",
		})
	}

	mounts := make([]mount.Mount, 0)

	cm.log.WithFields(containerFields).WithField("mounts", mounts).WithField("devices", devMappings).Debug("Generated mounts")

	hostConfig := &container.HostConfig{
		NetworkMode:    container.NetworkMode(cm.config.NetworkMode),
		CapAdd:         capAdd,
		SecurityOpt:    secOpt,
		ReadonlyRootfs: cm.config.DockerReadOnly,
		Mounts:         mounts,
		Resources: container.Resources{
			Memory:   cm.config.memoryLimitBytes,
			NanoCPUs: cm.config.cpuLimitNano,
			Devices:  devMappings,
		},
	}

	// Create an empty networking config that we'll fill if additional networks are specified
	networkingConfig := &network.NetworkingConfig{}
	endpointsConfig := make(map[string]*network.EndpointSettings)

	// If we have additional networks to connect to, set up the first one as the default
	if len(cm.config.Networks) > 0 {
		// First network becomes the default network at container creation
		endpointsConfig[cm.config.Networks[0]] = &network.EndpointSettings{}
		networkingConfig.EndpointsConfig = endpointsConfig
	}

	resp, err := cm.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	containerFields["containerID"] = resp.ID
	cm.log.WithFields(containerFields).Debug("Created container")

	if len(cm.config.Networks) > 1 {
		cm.log.WithFields(containerFields).Debug("Connecting to additional networks")
		// Connect to additional networks (skip the first one as it's already connected)
		for _, networkName := range cm.config.Networks[1:] {
			err := cm.client.NetworkConnect(ctx, networkName, resp.ID, &network.EndpointSettings{})
			if err != nil {
				// If network connection fails, cleanup the container and return error
				cm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
				return "", fmt.Errorf("failed to connect to network %s: %w", networkName, err)
			}
		}
	}

	cm.log.WithFields(containerFields).Info("Created container")

	return resp.ID, nil
}

func (cm *ContainerManager) StartContainer(ctx context.Context, containerID string) error {
	if err := cm.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	cm.log.WithField("containerID", containerID).Info("Started container")
	return nil
}

func (cm *ContainerManager) AttachContainer(ctx context.Context, containerID string) (types.HijackedResponse, error) {
	resp, err := cm.client.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return types.HijackedResponse{}, fmt.Errorf("failed to attach to container: %w", err)
	}

	return resp, nil
}

func (cm *ContainerManager) ResizeContainer(ctx context.Context, containerID string, height, width uint16) error {
	return cm.client.ContainerResize(ctx, containerID, container.ResizeOptions{
		Height: uint(height),
		Width:  uint(width),
	})
}

func (cm *ContainerManager) WaitContainer(ctx context.Context, containerID string) (<-chan container.WaitResponse, <-chan error) {
	return cm.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
}

func (cm *ContainerManager) RemoveContainer(ctx context.Context, cfg ContainerConfig, containerID string) error {
	err := cm.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	cm.log.WithField("containerID", containerID).Info("Removed container")

	err = cm.RemoveVFSMount(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to remove vfs mount: %w", err)
	}
	return nil
}

func (cm *ContainerManager) CleanUpContainers(ctx context.Context) error {
	cm.log.Info("Cleaning up containers")
	// Create a filter for the specified label
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "de.mc8051.sshcontainer=true")

	// List containers with the filter
	containers, err := cm.client.ContainerList(ctx, container.ListOptions{
		All:     true, // Include stopped containers
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	for _, c := range containers {
		cm.RemoveContainer(ctx, ContainerConfig{
			User: c.Labels["de.mc8051.sshcontainer.user"],
		}, c.ID)
	}
	return nil
}

func (cm *ContainerManager) CreateVFSMount(ctx context.Context, cfg ContainerConfig) (string, error) {
	// Check if file exists at /vfs/cfg.User, if not copy from /vfs.img
	userVFS := path.Join("/vfs", fmt.Sprintf("%s.img", cfg.User))

	fields := logrus.Fields{
		"user":    cfg.User,
		"userVFS": userVFS,
	}

	_, err := os.Stat(userVFS)
	if err != nil {
		if os.IsNotExist(err) {
			cm.log.WithFields(fields).Info("Copying VFS image to user VFS")
			// Copy /vfs.img to /vfs/cfg.User
			vfsImg, err := os.Open("/vfs.img")
			if err != nil {
				return "", fmt.Errorf("failed to open VFS image: %w", err)
			}
			defer vfsImg.Close()
			cm.log.WithFields(fields).Info("Opened VFS image")

			userVFS, err := os.Create(userVFS)
			if err != nil {
				return "", fmt.Errorf("failed to create user VFS: %w", err)
			}
			defer userVFS.Close()
			cm.log.WithFields(fields).Info("Created user VFS")

			if _, err := io.Copy(userVFS, vfsImg); err != nil {
				return "", fmt.Errorf("failed to copy VFS image: %w", err)
			}
			cm.log.WithFields(fields).Info("Copied VFS image")
		} else {
			return "", fmt.Errorf("failed to stat user VFS: %w", err)
		}
	}

	bd, _ := getBlockDevice(userVFS)
	if bd != "" {
		return bd, nil
	}

	cm.log.WithFields(fields).Info("Mounting user VFS")
	// exec command to mount userVFS
	if err := exec.Command("bash", "-c", fmt.Sprintf("NEXT=\"$(losetup -f)\" && losetup $NEXT \"%s\" && echo $NEXT", userVFS)).Run(); err != nil {
		return "", fmt.Errorf("failed to mount user VFS: %w", err)
	}
	cm.log.WithFields(fields).Info("Mounted user VFS")
	return getBlockDevice(userVFS)
}

func (cm *ContainerManager) RemoveVFSMount(ctx context.Context, cfg ContainerConfig) error {
	// Check if file exists at /vfs/cfg.User, if not copy from /vfs.img
	userVFS := path.Join("/vfs", fmt.Sprintf("%s.img", cfg.User))

	bd, err := getBlockDevice(userVFS)
	if err != nil {
		return fmt.Errorf("no blockdevice found: %w", err)
	}

	fields := logrus.Fields{
		"user":        cfg.User,
		"userVFS":     userVFS,
		"blockdevice": bd,
	}

	cm.log.WithFields(fields).Debug("Try VFS cleanups")
	if err := exec.Command("losetup", "-d", bd).Run(); err != nil {
		cm.log.WithFields(fields).Warn("Failed to unmount user VFS. This is not good but not terrible. Maybe the device is busy.")
		return nil
	}
	cm.log.WithFields(fields).Info("Unmounted user VFS")
	return nil
}

// getBlockDevice returns the block device associated with the mountpoint
func getBlockDevice(mp string) (string, error) {
	out, err := exec.Command("losetup", "--noheadings", "--output=NAME", "--associated", mp).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get blockdevice %s: %w", mp, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// getCurrentContainerId reads the container ID from the cgroup file
func getCurrentContainerId() (string, error) {
	// Read the cgroup file
	content, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("failed to read cgroup file: %v", err)
	}

	// Parse the content to find the container ID
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, "docker") {
			parts := strings.Split(line, "/")
			if len(parts) > 2 {
				return parts[len(parts)-1], nil
			}
		}
	}

	return "", fmt.Errorf("container ID not found in cgroup file")
}
