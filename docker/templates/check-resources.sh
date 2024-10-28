#!/bin/bash

echo "Resource Usage for $USER:"
echo "------------------------"
echo "Memory Usage:"
free -h

echo -e "\nCPU Usage:"
top -b -n 1 -u "$USER" | head -n 12

echo -e "\nDisk Usage:"
df -h /workspaces/"$USER"

echo -e "\nProcess Count:"
ps -u "$USER" | wc -l
