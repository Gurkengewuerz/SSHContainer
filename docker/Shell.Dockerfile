# Base image
FROM ubuntu:22.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install required packages
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    rsync \
    iptables \
    sudo \
    curl \
    wget \
    git \
    unzip \
    build-essential \
    libcurl4-openssl-dev \
    libssl-dev \
    libxml2-dev \
    zsh \
    locales && \
    rm -rf /var/lib/apt/lists/*

RUN locale-gen en_US.UTF-8

ENV CREATING_USER=user
ENV CREATING_WORKSPACE=/workspace

RUN mkdir -p $CREATING_WORKSPACE && \
    chmod 666 $CREATING_WORKSPACE

RUN useradd -M -s /bin/zsh -d $CREATING_WORKSPACE "$CREATING_USER"

# Set NVM environment variables
ENV NVM_DIR=/usr/local/nvm
ENV NODE_VERSION=23.1.0

# install nvm
RUN mkdir -p $NVM_DIR
RUN curl --silent -o- https://raw.githubusercontent.com/nvm-sh/nvm/refs/heads/master/install.sh | bash

# install node and npm
RUN echo "source $NVM_DIR/nvm.sh && \
    nvm install $NODE_VERSION && \
    nvm alias default $NODE_VERSION && \
    nvm use default" | bash

ENV NODE_PATH=$NVM_DIR/v$NODE_VERSION/lib/node_modules
ENV PATH=$NVM_DIR/versions/node/v$NODE_VERSION/bin:$PATH

ENV GOVERSION=1.22.1
# Install latest Go version
RUN curl -OL https://go.dev/dl/go${GOVERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GOVERSION}.linux-amd64.tar.gz && \
    rm go${GOVERSION}.linux-amd64.tar.gz

# Set Go environment variables
ENV GOPATH=/go
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 755 "$GOPATH"

# Install Neovim
RUN curl -LO https://github.com/neovim/neovim/releases/latest/download/nvim-linux64.tar.gz && \
    tar xzf nvim-linux64.tar.gz -C /opt && \
    ln -s /opt/nvim-linux64/bin/nvim /usr/local/bin/nvim && \
    rm nvim-linux64.tar.gz

RUN pip --no-cache-dir install neovim ruff-lsp numpy pandas matplotlib scipy sympy

COPY scripts/shell/entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh

COPY --chmod=750 templates/ /templates

CMD ["/bin/zsh"]
ENTRYPOINT ["/entrypoint.sh"]
