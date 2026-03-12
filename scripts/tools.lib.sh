# -*- mode: bash-ts-mode; sh-basic-offset 2; -*-

_LIB_NAME="${0##*/}"
_LIB_DIR=$( dirname "$(readlink -f -- "${0}")" )
_PROJECT_ROOT="${_LIB_DIR}/.."
_TOOLS_MOD_FILE="${_PROJECT_ROOT}/internal/tools/go.mod"

YQ=$( go tool -n -modfile "${_TOOLS_MOD_FILE}" yq )
KUSTOMIZE=$( go tool -n -modfile "${_TOOLS_MOD_FILE}" kustomize )
HELM=$( go tool -n -modfile "${_TOOLS_MOD_FILE}" helm )

export YQ \
       KUSTOMIZE \
       HELM
