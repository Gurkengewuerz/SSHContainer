# Standard bash configuration
[ -z "$PS1" ] && return

source $HOME/.profile

# Enable programmable completion
if [ -f /etc/bash_completion ]; then
    . /etc/bash_completion
fi

# Customize prompt
PS1='\[\033[01;32m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '
