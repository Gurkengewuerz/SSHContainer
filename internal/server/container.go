package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

// UserContainer represents a container for a specific user
type UserContainer struct {
	ID            string
	User          string
	ActiveStreams int
	LastUsed      time.Time
	mutex         sync.Mutex
}

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
	client          *client.Client
	config          *Config
	log             *logrus.Logger
	containers      map[string]*UserContainer // map of username to container
	containersMutex sync.RWMutex
	shutdownChan    chan struct{}
	blockDevice     string
}

func NewContainerManager(config *Config, log *logrus.Logger) (*ContainerManager, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	containerId := os.Getenv("CONTAINER_ID")
	if containerId == "" {
		return nil, fmt.Errorf("failed to get current container ID")
	}

	blockDevice := os.Getenv("BLOCK_DEVICE")
	if blockDevice == "" {
		return nil, fmt.Errorf("failed to get current mounted blockdevice")
	}

	ctx := context.Background()
	ct, err := dockerClient.ContainerInspect(ctx, containerId)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %v", err)
	}

	if len(ct.NetworkSettings.Networks) == 0 && len(config.Networks) == 0 {
		return nil, fmt.Errorf("no network settings found")
	}

	for networkName := range ct.NetworkSettings.Networks {
		if len(ct.NetworkSettings.Networks) == 1 || strings.HasSuffix(networkName, "_default") {
			config.Networks = append(config.Networks, networkName)
		}
	}

	cm := &ContainerManager{
		client:       dockerClient,
		config:       config,
		log:          log,
		containers:   make(map[string]*UserContainer),
		shutdownChan: make(chan struct{}),
		blockDevice:  blockDevice,
	}

	// Start container cleanup goroutine
	go cm.cleanupLoop()

	return cm, nil
}

func (cm *ContainerManager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.cleanupIdleContainers()
		case <-cm.shutdownChan:
			return
		}
	}
}

func (cm *ContainerManager) cleanupIdleContainers() {
	cm.containersMutex.Lock()
	defer cm.containersMutex.Unlock()

	ctx := context.Background()
	timeout := time.Duration(cm.config.ContainerIdleTimeout) * time.Second

	for username, uc := range cm.containers {
		uc.mutex.Lock()
		if uc.ActiveStreams == 0 && time.Since(uc.LastUsed) > timeout {
			cm.log.WithFields(logrus.Fields{
				"user":        username,
				"containerID": uc.ID,
				"idleTime":    time.Since(uc.LastUsed),
			}).Info("Removing idle container")

			if err := cm.removeContainer(ctx, username); err != nil {
				cm.log.WithError(err).Error("Failed to remove idle container")
			}
		}
		uc.mutex.Unlock()
	}
}

func (cm *ContainerManager) GetOrCreateContainer(ctx context.Context, username string, env []string) (string, error) {
	cm.containersMutex.Lock()
	defer cm.containersMutex.Unlock()

	// Check if ct exists for user
	if ct, exists := cm.containers[username]; exists {
		ct.mutex.Lock()
		ct.ActiveStreams++
		ct.LastUsed = time.Now()
		ct.mutex.Unlock()
		return ct.ID, nil
	}

	// Create new ct for user
	containerConfig := ContainerConfig{
		Image: cm.config.DockerImage,
		User:  username,
		Env:   env,
	}

	containerID, err := cm.createContainer(ctx, containerConfig)
	if err != nil {
		return "", err
	}

	if err := cm.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start ct: %w", err)
	}

	cm.containers[username] = &UserContainer{
		ID:            containerID,
		User:          username,
		ActiveStreams: 1,
		LastUsed:      time.Now(),
	}

	return containerID, nil
}

func (cm *ContainerManager) createContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	// env is not set for all session
	// env is set via container exec/attach
	env := make([]string, 0)
	devices := cm.config.DockerDevices
	capAdd := cm.config.DockerCapAdd
	secOpt := cm.config.DockerSecurityOpt

	volumeName, err := cm.CreateVFSMount(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create VFS mount: %w", err)
	}

	containerConfig := &container.Config{
		Image:     cfg.Image,
		Env:       env,
		Cmd:       cfg.Cmd,
		OpenStdin: true,
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
	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeVolume,
		Source: volumeName,
		Target: cm.config.ContainerVFSMountPath,
	})
	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeTmpfs,
		Target: "/tmp",
		TmpfsOptions: &mount.TmpfsOptions{
			SizeBytes: cm.config.quotaBytes,
			Mode:      os.FileMode(1777),
		},
	})

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

	networkingConfig := &network.NetworkingConfig{}
	endpointsConfig := make(map[string]*network.EndpointSettings)

	if len(cm.config.Networks) > 0 {
		endpointsConfig[cm.config.Networks[0]] = &network.EndpointSettings{}
		networkingConfig.EndpointsConfig = endpointsConfig
	}

	resp, err := cm.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, fmt.Sprintf("sshcontainer-%s", cfg.User))
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	containerFields["containerID"] = resp.ID

	if len(cm.config.Networks) > 1 {
		cm.log.WithFields(containerFields).Debug("Connecting to additional networks")
		for _, networkName := range cm.config.Networks[1:] {
			err := cm.client.NetworkConnect(ctx, networkName, resp.ID, &network.EndpointSettings{})
			if err != nil {
				cm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
				return "", fmt.Errorf("failed to connect to network %s: %w", networkName, err)
			}
		}
	}

	cm.log.WithFields(containerFields).Info("Created container")
	return resp.ID, nil
}

func (cm *ContainerManager) ReleaseContainer(username string) {
	cm.log.WithFields(logrus.Fields{
		"username": username,
	}).Debug("Releasing container")
	cm.containersMutex.RLock()
	defer cm.containersMutex.RUnlock()

	if ct, exists := cm.containers[username]; exists {
		ct.mutex.Lock()
		ct.ActiveStreams--
		ct.LastUsed = time.Now()
		ct.mutex.Unlock()
	}
}

func (cm *ContainerManager) AttachToContainer(ctx context.Context, containerID string) (types.HijackedResponse, error) {
	cm.log.WithFields(logrus.Fields{
		"containerID": containerID,
	}).Debug("Attaching to container")
	return cm.client.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
}

func (cm *ContainerManager) ExecInContainer(ctx context.Context, containerID string, env []string, cmd []string, user string, isPty bool) (types.HijackedResponse, string, error) {
	cm.log.WithFields(logrus.Fields{
		"containerID": containerID,
		"env":         env,
		"cmd":         cmd,
	}).Debug("Executing command in container")
	execConfig := container.ExecOptions{
		User:         user,
		Tty:          isPty,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Env:          env,
		Cmd:          cmd,
	}

	execCreateResp, err := cm.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return types.HijackedResponse{}, "", fmt.Errorf("failed to create exec: %w", err)
	}

	execAttachResp, err := cm.client.ContainerExecAttach(ctx, execCreateResp.ID, container.ExecAttachOptions{
		Tty: isPty,
	})
	if err != nil {
		return types.HijackedResponse{}, "", fmt.Errorf("failed to attach to exec: %w", err)
	}

	return execAttachResp, execCreateResp.ID, nil
}

func (cm *ContainerManager) ResizeExec(ctx context.Context, execID string, height, width uint16) error {
	return cm.client.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: uint(height),
		Width:  uint(width),
	})
}

func (cm *ContainerManager) Shutdown() {
	close(cm.shutdownChan)
	cm.CleanUpContainers(context.Background())
}

func (cm *ContainerManager) CleanUpContainers(ctx context.Context) error {
	cm.log.Info("Cleaning up all containers")

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "de.mc8051.sshcontainer=true")

	containers, err := cm.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		username := c.Labels["de.mc8051.sshcontainer.user"]
		if err := cm.removeContainer(ctx, username); err != nil {
			cm.log.WithError(err).Error("Failed to remove container during cleanup")
		}
	}

	return nil
}

func (cm *ContainerManager) removeContainer(ctx context.Context, username string) error {
	if ct, exists := cm.containers[username]; exists {
		cm.log.WithFields(logrus.Fields{
			"username":    username,
			"containerID": ct.ID,
		}).Info("Removing container")
		if err := cm.client.ContainerRemove(ctx, ct.ID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}

		if err := cm.RemoveVFSMount(ctx, ContainerConfig{User: username}); err != nil {
			return fmt.Errorf("failed to remove vfs mount: %w", err)
		}

		delete(cm.containers, username)
	}
	return nil
}

func (cm *ContainerManager) CreateVFSMount(ctx context.Context, cfg ContainerConfig) (string, error) {
	userVFS := path.Join("/mnt/vfs", cfg.User)
	volumeName := fmt.Sprintf("sshcontainer-vfs-%s", cfg.User)

	fields := logrus.Fields{
		"user":        cfg.User,
		"userVFS":     userVFS,
		"blockDevice": cm.blockDevice,
		"volumeName":  volumeName,
	}

	// check if userVFS already exists
	_, err := os.Stat(userVFS)
	if err != nil {
		if os.IsNotExist(err) {
			cm.log.WithFields(fields).Info("Creating user VFS")
			// create VFS using btrfs subvolume create
			if err := exec.Command("btrfs", "subvolume", "create", userVFS).Run(); err != nil {
				return "", fmt.Errorf("failed to create user VFS: %w", err)
			}
			cm.log.WithFields(fields).Info("Created user VFS")
		} else {
			return "", fmt.Errorf("failed to stat user VFS: %w", err)
		}
	}

	// enable quota using btrfs qgroup limit size /volume/subvolume
	if err := exec.Command("btrfs", "qgroup", "limit", cm.config.Quota, userVFS).Run(); err != nil {
		return "", fmt.Errorf("failed to enable quota: %w", err)
	}
	cm.log.WithFields(fields).Info("Updated quota")

	// check if volume already exists
	_, err = cm.client.VolumeInspect(ctx, volumeName)
	if err == nil {
		cm.log.WithFields(fields).Debug("Volume already exists")
		// delete volume
		err = cm.client.VolumeRemove(ctx, volumeName, true)
		if err != nil {
			return "", fmt.Errorf("failed to remove existing volume: %w", err)
		}
		cm.log.WithFields(fields).Info("Removed existing volume")
	}

	cm.log.WithFields(fields).Debug("Creating volume")

	_, err = cm.client.VolumeCreate(ctx, volume.CreateOptions{
		Name:   volumeName,
		Driver: "local",
		DriverOpts: map[string]string{
			"type":   "btrfs",
			"device": cm.blockDevice,
			"o":      fmt.Sprintf("subvol=%s", cfg.User),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create volume: %w", err)
	}

	cm.log.WithFields(fields).Info("Created volume")
	return volumeName, nil
}

func (cm *ContainerManager) RemoveVFSMount(ctx context.Context, cfg ContainerConfig) error {
	volumeName := fmt.Sprintf("sshcontainer-vfs-%s", cfg.User)

	fields := logrus.Fields{
		"user":        cfg.User,
		"blockdevice": cm.blockDevice,
		"volumeName":  volumeName,
	}

	// ignore error explicitly - volume already deleted in removeContainer using RemoveVolumes: true
	// here we just want to make sure it's gone
	_ = cm.client.VolumeRemove(ctx, volumeName, true)

	cm.log.WithFields(fields).Info("Removed volume")
	return nil
}
