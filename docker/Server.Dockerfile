FROM golang:1.23-bookworm as builder

# Create and change to the app directory.
WORKDIR /app

# Retrieve application dependencies.
# This allows the container build to reuse cached dependencies.
# Expecting to copy go.mod and if present go.sum.
COPY go.* ./
RUN go mod download

# Copy local code to the container image.
COPY . ./

# Build the binary.
RUN go build -v -o server ./cmd/server/main.go

FROM debian:bookworm-slim
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates \
    bc \
    coreutils \
    e2fsprogs && \
    rm -rf /var/lib/apt/lists/*

COPY --chmod=750 docker/scripts/server/entrypoint.sh /entrypoint.sh
RUN mkdir -p /vfs && chmod 650 /vfs
RUN mkdir -p /workspaces && chmod 650 /workspaces

# Copy the binary to the production image from the builder stage.
COPY --from=builder /app/server /server

# Run the web service on container startup.
CMD ["/server"]
ENTRYPOINT ["/entrypoint.sh"]
