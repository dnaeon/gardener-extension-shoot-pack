#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

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
  ${_SCRIPT_NAME} - A utility for building packs

USAGE:
  ${_SCRIPT_NAME} help
  ${_SCRIPT_NAME} [b|build] /path/to/pack/dir

EXAMPLES:
  Display usage info:
    $ ${_SCRIPT_NAME} help

  Build a pack from a given directory:
    $ ${_SCRIPT_NAME} build /path/to/some/pack

  Build a pack from a given pack spec file:
    $ ${_SCRIPT_NAME} build /path/to/some/pack/PACKAGE
_EOF_
}

# Builds a pack from a given path.
#
# NOTE: This function is meant to be called from within a sub-shell.
#
# $1: Path to a pack
function _build_pack() {
  local _pack_path="${1}"
  local _pack_spec_base_dir=""

  # Sanity checks
  if [[ -z "${_pack_path}" ]]; then
    _msg_error "_build_pack: no pack dir has been specified" 1
  fi

  _pack_spec_base_dir="$( _get_pack_spec_base_dir "${_pack_path}" )"
  if [[ -z "${_pack_spec_base_dir}" ]]; then
    _msg_error "_build_pack: unable to find base pack dir for ${_pack_path}" 1
  fi

  # Pack build steps
  # shellcheck source=/dev/null
  source "${_LIBS_DIR}/tools.lib.sh"
  # shellcheck source=/dev/null
  source "${_LIBS_DIR}/pack.lib.sh"

  # Source and sanity check the pack spec
  local _pack_spec_file="${_pack_spec_base_dir}/${PACK_SPEC_FILE}"
  # shellcheck source=/dev/null
  source "${_pack_spec_file}"

  # Require the presence of certain vars in the pack spec
  for _var in "${PACK_SPEC_REQUIRED_VARS[@]}"; do
    if [[ -z "${!_var}" ]]; then
      _msg_error "_build_pack: required var ${_var} is not set in pack spec @ ${_pack_spec_file}" 1
    fi
  done

  # Each pack spec must define a `package()' function
  type -t package >& /dev/null || {
    _msg_error "_build_pack: ${_pack_spec_file} does not provide a package() function" 1
  }

  _msg_info "Setting up environment for pack ${NAME}@${VERSION} ..."
  SRC_DIR="${_pack_spec_base_dir}"  # Source directory points to the pack spec dir
  PACK_DIR="${ASSETS_PKG}/packs/${NAME}/${VERSION}"
  mkdir -p "${PACK_DIR}"
  PACK_DIR=$( realpath "${PACK_DIR}" )

  # Make SRC_DIR and PACK_DIR available to package() functions
  export SRC_DIR PACK_DIR

  # Clean up old resources, if any.
  find "${PACK_DIR}" \
       -type f \
       -iname "${PACK_RESOURCES_GLOB}" \
       -delete

  # Package it up
  _msg_info "Building pack ${NAME}@${VERSION} ..."
  package

  # Generate metadata files for the pack
  _msg_info "Generating metadata files for pack ${NAME}@${VERSION} ..."
  echo "${NAMESPACE}" > "${PACK_DIR}/${PACK_METADATA_NAMESPACE}"
  echo "${DESCRIPTION}" > "${PACK_DIR}/${PACK_METADATA_DESC}"

  # Re-create sums file
  if [[ -f "${PACK_DIR}/${PACK_METADATA_SUMS}" ]]; then
    rm -f "${PACK_DIR}/${PACK_METADATA_SUMS}"
  fi

  cd "${ASSETS_PKG}"
  find "packs/${NAME}/${VERSION}" \
       -type f \
       -iname "${PACK_RESOURCES_GLOB}" \
       -exec sha256sum {} \; | tee -a "${PACK_DIR}/${PACK_METADATA_SUMS}"
  cd "${OLDPWD}"
  _msg_info "Pack ${NAME}@${VERSION} built successfully"
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
    b|build)
      # NOTE: pack building should be executed from within a sub-shell
      set +e
      (
        set -e
        _build_pack "${_pack_path}"
      )
      local _rc=$?
      set -e
      if [[ "${_rc}" -ne 0 ]]; then
        _msg_error "Failed to build pack" "${_rc}"
      fi
      ;;
    *)
      _msg_error "unknown command ${_operation}" 64  # EX_USAGE
      ;;
  esac
}

_main "$@"
