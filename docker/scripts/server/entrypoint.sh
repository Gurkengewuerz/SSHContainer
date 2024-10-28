#!/bin/bash

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

# Validate input
if [ -z "$QUOTA" ]; then
    log ERROR "QUOTA environment variable must be set"
    exit 1
fi

# Convert QUOTA to bytes
QUOTA_BYTES=$(convert_size "$QUOTA")
if [ $? -ne 0 ] || [ -z "$QUOTA_BYTES" ]; then
    log ERROR "Invalid QUOTA format. Examples: 1G, 1GB, 1GiB, 512M, 512MB"
    exit 1
fi

# Calculate size in MiB for dd
QUOTA_MB=$(( QUOTA_BYTES / 1024 / 1024 ))

log INFO "Creating VFS file with size: $QUOTA ($QUOTA_BYTES bytes)"

# Create the VFS file
if ! dd if=/dev/zero of="/vfs.img" bs=1M count="$QUOTA_MB" status=none 2>/dev/null; then
    log ERROR "Failed to create VFS template"
    exit 1
fi

log INFO "Formatting filesystem"

# Format the filesystem
if ! mkfs.ext4 -F -q "/vfs.img" 2>/dev/null; then
    log ERROR "Failed to format VFS template"
    exit 1
fi

log INFO "VFS creation completed successfully"

log INFO "Getting my container id"
export CONTAINER_ID=$(cat /proc/self/mountinfo | grep -m1 -oE 'docker/containers/([a-f0-9]+)/' | xargs basename)
log INFO "My container id is $CONTAINER_ID"

exec "$@"
