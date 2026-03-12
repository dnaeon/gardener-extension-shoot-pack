#!/usr/bin/env bash

set -e

_SCRIPT_NAME="${0##*/}"
_SCRIPT_DIR=$( dirname "$(readlink -f -- "${0}")" )
_LIBS_DIR="${_SCRIPT_DIR}"

# Set to true in order to enable debugging
DEBUG=${DEBUG:-false}

if [[ "${DEBUG}" == "true" ]]; then
  set -x
fi

# shellcheck source=/dev/null
source "${_LIBS_DIR}/pack.lib.sh"
# shellcheck source=/dev/null
source "${_LIBS_DIR}/logging.lib.sh"

# Prints the usage info of the script
function _usage() {
  cat <<_EOF_
NAME:
  ${_SCRIPT_NAME} - A utility for verifying packs

USAGE:
  ${_SCRIPT_NAME} help
  ${_SCRIPT_NAME} [v|verify] /path/to/pack/dir

EXAMPLES:
  Display usage info:
    $ ${_SCRIPT_NAME} help

  Verify a pack from a given directory:
    $ ${_SCRIPT_NAME} verify /path/to/some/pack

  Verify a pack from a given pack spec file:
    $ ${_SCRIPT_NAME} verify /path/to/some/pack/PACKAGE
_EOF_
}

# Verifies a pack from a given path.
#
# NOTE: This function is meant to called from within a sub-shell.
#
# $1: Path to a pack
function _verify_pack() {
  local _pack_path="${1}"
  local _pack_spec_base_dir=""

  # Sanity checks
  if [[ -z "${_pack_path}" ]]; then
    _msg_error "_verify_pack: no pack dir has been specified" 1
  fi

  _pack_spec_base_dir="$( _get_pack_spec_base_dir "${_pack_path}" )"
  if [[ -z "${_pack_spec_base_dir}" ]]; then
    _msg_error "_verify_pack: unable to find base pack dir for ${_pack_path}" 1
  fi

  # Source and sanity check the pack spec
  local _pack_spec_file="${_pack_spec_base_dir}/${PACK_SPEC_FILE}"
  # shellcheck source=/dev/null
  source "${_pack_spec_file}"

  # Require the presence of certain vars in the pack spec
  for _var in "${PACK_SPEC_REQUIRED_VARS[@]}"; do
    if [ -z "${!_var}" ]; then
      _msg_error "_verify_pack: required var ${_var} is not set in pack spec @ ${_pack_spec_file}" 1
    fi
  done

  # Each pack spec must define a `package()' function
  type -t package >& /dev/null || {
    _msg_error "_verify_pack: ${_pack_spec_file} does not provide a package() function" 1
  }
  _msg_info "package() is defined: OK"

  # Run tests, if a `package_test()' function has been provided by the spec
  type -t package_test >& /dev/null && {
    _msg_info "package_test() is defined: OK"
  }

  # Verify metadata files for the pack
  PACK_DIR="${ASSETS_PKG}/packs/${NAME}/${VERSION}"

  # Verify NAMESPACE metadata file
  local _md_file_namespace="${PACK_DIR}/${PACK_METADATA_NAMESPACE}"
  if [[ ! -f "${_md_file_namespace}" ]]; then
    _msg_error "_verify_pack: Metadata file not found @ ${_md_file_namespace}" 1
  fi
  if [[ "$( cat "${_md_file_namespace}" )" != "${NAMESPACE}" ]]; then
    _msg_error "_verify_pack: NAMESPACE var from spec differs from metadata file @ ${_md_file_namespace}" 1
  fi
  _msg_info "Metadata NAMESPACE file: OK"

  # Verify DESC metadata file
  local _md_file_desc="${PACK_DIR}/${PACK_METADATA_DESC}"
  if [[ ! -f "${_md_file_desc}" ]]; then
    _msg_error "_verify_pack: Metadata file not found @ ${_md_file_desc}" 1
  fi
  if [[ "$( cat "${_md_file_desc}" )" != "${DESCRIPTION}" ]]; then
    _msg_error "_verify_pack: DESCRIPTION var from spec differs from metadata file @ ${_md_file_desc}" 1
  fi
  _msg_info "Metadata DESC file: OK"

  # Verify checksums
  _msg_info "Verifying checksums of pack resources ..."
  local _md_file_sums="${PACK_DIR}/${PACK_METADATA_SUMS}"
  if [[ ! -f "${_md_file_sums}" ]]; then
    _msg_error "_verify_pack: Metadata file not found @ ${_md_file_sums}" 1
  fi
  cd "${ASSETS_PKG}"
  cat "${_md_file_sums}" | sha256sum --check -
  _msg_info "Metadata SUMS file: OK"
  cd "${OLDPWD}"

  _msg_info "Pack ${NAME}@${VERSION} verified successfully"
}

# Main entrypoint
function _main() {
  if [[ $# -ne 2 ]]; then
    _usage
    exit 64  # EX_USAGE
  fi

  local _operation="${1}"
  local _pack_path="${2}"

  case "${_operation}" in
    h|help)
      _usage
      exit 0
      ;;
    v|verify)
      # NOTE: pack verification should be executed from within a sub-shell
      set +e
      (
        set -e
        _verify_pack "${_pack_path}"
      )
      local _rc=$?
      set -e
      if [[ "${_rc}" -ne 0 ]]; then
        _msg_error "Failed to verify pack" "${_rc}"
      fi
      ;;
    *)
      _msg_error "unknown command ${_operation}" 64  # EX_USAGE
      ;;
  esac
}

_main "$@"
