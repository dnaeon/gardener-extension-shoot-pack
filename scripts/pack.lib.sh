# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# shellcheck shell=bash

_LIB_NAME="${0##*/}"
_LIB_DIR=$( dirname "$(readlink -f -- "${0}")" )
_PROJECT_ROOT="$( dirname "${_LIB_DIR}" )"

# shellcheck source=/dev/null
source "${_LIB_DIR}/logging.lib.sh"

# Export common vars for use by clients

export PACK_METADATA_DESC="DESC"
export PACK_METADATA_SUMS="SUMS"
export PACK_SPEC_FILE="PACKAGE"
export PACK_RESOURCES_GLOB="*.yaml"
export ASSETS_PKG="${_PROJECT_ROOT}/pkg/assets"
export PACK_SPEC_REQUIRED_VARS=(
    NAME
    VERSION
    DESCRIPTION
)

export PACK_SPEC_NAME_REGEX='^[[:alnum:]._+@-]*$'
export PACK_SPEC_VERSION_REGEX='^[[:alnum:]._+@-]*$'

# The namespace in which packs get installed is `kube-system', since this is the
# system namespace used by components deployed by Gardener in shoot clusters.
#
# https://gardener.cloud/docs/getting-started/ca-components/#managedresources
# https://github.com/gardener/gardener/issues/14342
# https://github.com/gardener/gardener/pull/14335
export PACK_NAMESPACE="kube-system"

# Returns the base dir of a pack spec
#
# $1: Path to a pack spec file or directory containing a pack spec
function _get_pack_spec_base_dir() {
  local _path="${1}"
  local _base_dir=""

  if [[ -z "${_path}" ]]; then
    _msg_error "_get_pack_base_dir: empty pack dir specified" 1
  fi

  if [[ "$( basename "${_path}" )" == "${PACK_SPEC_FILE}" ]]; then
    # We were called with a path to a pack file
    _base_dir="$( dirname "${_path}" )"
  elif [[ -f "${_path}/${PACK_SPEC_FILE}" ]]; then
    # We were called with a path, which contains a pack file
    _base_dir="${_path}"
  else
    _msg_error "_get_pack_base_dir: unable to find a pack spec at path ${_path}" 1
  fi

  realpath "${_base_dir}"
}
