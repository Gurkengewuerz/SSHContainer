#!/usr/bin/env bash

stty sane

# wait until /tmp/.init exists
while [ ! -f /tmp/.init ]; do
    echo "Waiting for initialization complete..."
    sleep 1
done

exec "$@"
