export LANG='en_US.UTF-8'
export LANGUAGE='en_US:en'
export LC_ALL='en_US.UTF-8'
[ -z "$TERM" ] && export TERM=xterm

export PATH=$PATH:/usr/local/go/bin:/go/bin:/usr/local/bin/:$NVM_DIR/versions/node/v$NODE_VERSION/bin
export GOPATH=$HOME/go/$GOVERSION
export NODE_PATH=$HOME/nvm/v$NODE_VERSION/lib/node_modules
export EDITOR=nvim
export VISUAL=nvim

export http_proxy=http://proxy:3128
export https_proxy=http://proxy:3128
export no_proxy=localhost,proxy
export HTTP_PROXY=http://proxy:3128
export HTTPS_PROXY=http://proxy:3128
export NO_PROXY=localhost,proxy
export ftp_proxy=http://proxy:3128

##### Zsh/Oh-my-Zsh Configuration
export ZSH="$HOME/.oh-my-zsh"
export ZSH_COMPDUMP=$ZSH/cache/.zcompdump

# Some good defaults
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'