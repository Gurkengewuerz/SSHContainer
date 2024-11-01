#!/bin/bash
shopt -s dotglob

# First, copy all files including hidden ones
if ! rsync -r --chown="${CREATING_USER}:${CREATING_USER}" /templates/ "$CREATING_WORKSPACE"; then
    echo "Failed to copy template files to workspace"
    exit 1
fi

chmod 777 "$CREATING_WORKSPACE"

# Allow all traffic to Docker's DNS (127.0.0.11) regardless of port
iptables -A OUTPUT -m owner --uid-owner $CREATING_USER -d 127.0.0.11 -j ACCEPT

# Your existing proxy and reject rules
iptables -A OUTPUT -m owner --uid-owner $CREATING_USER -p tcp --dport 3128 -j ACCEPT
iptables -A OUTPUT -m owner --uid-owner $CREATING_USER -j REJECT
ip6tables -A OUTPUT -m owner --uid-owner $CREATING_USER -j REJECT

cd "$CREATING_WORKSPACE"
exec runuser $CREATING_USER -c "$@"
#exec "$@"
