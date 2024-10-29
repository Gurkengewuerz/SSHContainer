export LANG='en_US.UTF-8'
export LANGUAGE='en_US:en'
export LC_ALL='en_US.UTF-8'
[ -z "$TERM" ] && export TERM=xterm

export GOVERSION=1.22.1
export NODE_VERSION=23.1.0
export NVM_DIR=/usr/local/nvm

export PATH=$PATH:/usr/local/go/bin:/go/bin:/usr/local/bin/:$NVM_DIR/versions/node/v$NODE_VERSION/bin
export GOPATH=$HOME/go/$GOVERSION
export NODE_PATH=$HOME/nvm/v$NODE_VERSION/lib/node_modules
export EDITOR=nvim
export VISUAL=nvim

if [ -z "${IS_DEV_ENV}" ]; then
export http_proxy=http://proxy:3128
export https_proxy=http://proxy:3128
export no_proxy=localhost,proxy
export HTTP_PROXY=http://proxy:3128
export HTTPS_PROXY=http://proxy:3128
export NO_PROXY=localhost,proxy
export ftp_proxy=http://proxy:3128
fi


##### Zsh/Oh-my-Zsh Configuration
export skip_global_compinit=1
export ZSH="$HOME/.oh-my-zsh"
export ZSH_COMPDUMP=$ZSH/cache/.zcompdump

# Some good defaults
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'
