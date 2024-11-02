#!/usr/bin/env bash

# Convert human-readable sizes to bytes
# Supports formats like 1G, 1GB, 1GiB, 512M, 512MB, etc.
convert_size() {
    local size=$1
    local value=$(echo "$size" | sed 's/[^0-9.]//g')
    local unit=$(echo "$size" | sed 's/[0-9.]//g' | tr '[:lower:]' '[:upper:]')

    case "$unit" in
        B|BYTES) echo "${value}" ;;
        K|KB|KIB) echo "$( printf "%.0f" "$(echo "${value} * 1024" | bc)" )" ;;
        M|MB|MIB) echo "$( printf "%.0f" "$(echo "${value} * 1024 * 1024" | bc)" )" ;;
        G|GB|GIB) echo "$( printf "%.0f" "$(echo "${value} * 1024 * 1024 * 1024" | bc)" )" ;;
        T|TB|TIB) echo "$( printf "%.0f" "$(echo "${value} * 1024 * 1024 * 1024 * 1024" | bc)" )" ;;
        *) echo "Invalid size format: $size" >&2; return 1 ;;
    esac
}

# Pretty logging function
log() {
    local level=$1
    shift
    local color_start=""
    local color_end="\033[0m"

    case "$level" in
        INFO)  color_start="\033[0;32m" ;; # Green
        WARN)  color_start="\033[0;33m" ;; # Yellow
        ERROR) color_start="\033[0;31m" ;; # Red
    esac

    echo -e "${color_start}[$(date '+%Y-%m-%d %H:%M:%S')] ${level}: $*${color_end}"
}

cleanup() {
    log INFO "Received signal, starting cleanup..."

    # Kill the child process if it exists
    if [ -n "$CHILD_PID" ] && kill -0 $CHILD_PID 2>/dev/null; then
        log INFO "Stopping child process (PID: $CHILD_PID)"
        kill -TERM $CHILD_PID 2>/dev/null
        wait $CHILD_PID 2>/dev/null
    fi

    log INFO "Unmounting VFS image"
    umount "$MOUNTPOINT" 2>/dev/null
    losetup -d "$BLOCK_DEVICE" 2>/dev/null
    log INFO "VFS image unmounted"

    log INFO "Exiting"
    exit 0
}

# Validate input
if [ -z "$QUOTA" ]; then
    log ERROR "QUOTA environment variable must be set"
    exit 1
fi

if [ -z "$PARTITION_SIZE" ]; then
    log ERROR "PARTITION_SIZE environment variable must be set"
    exit 1
fi

# Convert QUOTA to bytes
QUOTA_BYTES=$(convert_size "$PARTITION_SIZE")
if [ $? -ne 0 ] || [ -z "$QUOTA_BYTES" ]; then
    log ERROR "Invalid QUOTA format. Examples: 1G, 1GB, 1GiB, 512M, 512MB"
    exit 1
fi

# Calculate size in MiB for dd
QUOTA_MB=$(( QUOTA_BYTES / 1024 / 1024 ))

VFS_IMG="/vfs/vfs.img"

# check if $VFS_IMG not exists
if [ -f "$VFS_IMG" ]; then
    log INFO "VFS file already exists, skipping creation"
else
    log INFO "VFS file $VFS_IMG does not exist, creating"
    log INFO "Creating VFS file with size: $PARTITION_SIZE ($QUOTA_BYTES bytes)"

    # Create the VFS file
    if ! dd if=/dev/zero of="$VFS_IMG" bs=1M count="$QUOTA_MB" status=none 2>/dev/null; then
        log ERROR "Failed to create VFS"
        exit 1
    fi

    log INFO "Formatting filesystem"

    # Format the filesystem
    if ! mkfs.btrfs -f -q "$VFS_IMG" 2>/dev/null; then
        log ERROR "Failed to format VFS"
        exit 1
    fi

    log INFO "VFS creation completed successfully"
fi

# Mount the VFS
log INFO "Mounting VFS image"
export BLOCK_DEVICE="$(losetup -f)"
losetup $BLOCK_DEVICE "$VFS_IMG"
log INFO "VFS image mounted at $BLOCK_DEVICE"

# Create the mountpoint
export MOUNTPOINT="/mnt/vfs"
mkdir -p "$MOUNTPOINT"
mount -t btrfs "$BLOCK_DEVICE" "$MOUNTPOINT"
log INFO "VFS image mounted at $MOUNTPOINT"

log INFO "Setting up quota"
btrfs quota enable "$MOUNTPOINT"

log INFO "Getting my container id"
export CONTAINER_ID=$(cat /proc/self/mountinfo | grep -m1 -oE 'docker/containers/([a-f0-9]+)/' | xargs basename)
log INFO "My container id is $CONTAINER_ID"

# Set up signal handling
trap cleanup SIGTERM SIGINT SIGQUIT SIGHUP

# Instead of exec, run the command in the background and wait
"$@" &
CHILD_PID=$!

# Wait for the child process
wait $CHILD_PID
