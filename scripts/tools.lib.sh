# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# shellcheck shell=bash

_LIB_NAME="${0##*/}"
_LIB_DIR=$( dirname "$(readlink -f -- "${0}")" )
_PROJECT_ROOT="${_LIB_DIR}/.."
_TOOLS_MOD_FILE="${_PROJECT_ROOT}/internal/tools/go.mod"

YQ=$( go tool -n -modfile "${_TOOLS_MOD_FILE}" yq )
KUSTOMIZE=$( go tool -n -modfile "${_TOOLS_MOD_FILE}" kustomize )
HELM=$( go tool -n -modfile "${_TOOLS_MOD_FILE}" helm )
KUBECTL_SLICE=$( go tool -n -modfile "${_TOOLS_MOD_FILE}" kubectl-slice )
KUBECONFORM=$( go tool -n -modfile "${_TOOLS_MOD_FILE}" kubeconform )

export YQ \
       KUSTOMIZE \
       HELM \
       KUBECTL_SLICE \
       KUBECONFORM
