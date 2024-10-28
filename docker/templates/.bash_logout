# ~/.bash_logout: executed by bash(1) when login shell exits.

# Clear the screen for security's sake when logging out
clear
reset

# Log logout to syslog
logger -p auth.info -t "user-session" "User logged out: $USER"
