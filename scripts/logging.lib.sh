# -*- mode: bash-ts-mode; sh-basic-offset 2; -*-

_SCRIPT_NAME="${0##*/}"

# Refer to the ANSI escape codes table for more details.
# https://en.wikipedia.org/wiki/ANSI_escape_code
_RED='\033[0;31m'
_YELLOW='\033[0;33m'
_GREEN='\033[0;32m'
_CYAN='\033[0;36m'
_NO_COLOR='\033[0m'

# Set this to false in order to disable timestamps in log messages.
LOG_WITH_TIMESTAMP="true"

# Display an INFO message
#
# $1: Message to display
function _msg_info() {
  local _msg="${1}"
  local _ts="$( date +%Y-%m-%d-%T.%3N)"

  if [[ "${LOG_WITH_TIMESTAMP}" == "true" ]]; then
    echo -e "[${_ts}] ${_SCRIPT_NAME} | ${_GREEN}INFO${_NO_COLOR}: ${_msg}"
  else
    echo -e "${_SCRIPT_NAME} | ${_GREEN}INFO${_NO_COLOR}: ${_msg}"
  fi
}

# Display a WARN message
# $1: Message to display
function _msg_warn() {
  local _msg="${1}"
  local _ts="$( date +%Y-%m-%d-%T.%3N)"

  if [[ "${LOG_WITH_TIMESTAMP}" == "true" ]]; then
    echo -e "[$( date +%Y-%m-%d-%T.%3N)] | ${_SCRIPT_NAME} ${_YELLOW}WARN${_NO_COLOR}: ${_msg}"
  else
    echo -e "${_SCRIPT_NAME} | ${_YELLOW}WARN${_NO_COLOR}: ${_msg}"
  fi
}

# Display an ERROR message
#
# $1: Message to display
# $2: Exit code
function _msg_error() {
  local _msg="${1}"
  local _rc=${2}
  local _ts="$( date +%Y-%m-%d-%T.%3N)"

  if [[ "${LOG_WITH_TIMESTAMP}" == "true" ]]; then
    echo -e "[$( date +%Y-%m-%d-%T.%3N)] ${_SCRIPT_NAME} | ${_RED}ERROR${_NO_COLOR}: ${_msg}"
  else
    echo -e "${_SCRIPT_NAME} | ${_RED}ERROR${_NO_COLOR}: ${_msg}"
  fi

  if [[ ${_rc} -ne 0 ]]; then
    exit ${_rc}
  fi
}
