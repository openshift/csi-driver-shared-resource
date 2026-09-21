#!/usr/bin/env bash

# release.sh renders the config/ kustomize tree into a single release.yaml
# manifest at the repository root.
#
# Usage:
#   hack/release.sh [-h|--help]

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_DIR="${REPO_ROOT}/config"
RELEASE_FILE="${REPO_ROOT}/release.yaml"

die() {
	echo "error: $*" >&2
	exit 1
}

case "${1:-}" in
-h | --help)
	sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
	exit 0
	;;
"") ;;
*)
	die "unexpected argument: $1 (see --help)"
	;;
esac

[[ -f "${CONFIG_DIR}/kustomization.yaml" ]] || die "missing ${CONFIG_DIR}/kustomization.yaml"

if command -v kustomize >/dev/null 2>&1; then
	KUSTOMIZE=(kustomize build)
elif command -v kubectl >/dev/null 2>&1; then
	KUSTOMIZE=(kubectl kustomize)
else
	die "neither 'kustomize' nor 'kubectl' found on PATH"
fi

echo "==> Rendering ${CONFIG_DIR} into ${RELEASE_FILE}"
rm -f "${RELEASE_FILE}"
"${KUSTOMIZE[@]}" "${CONFIG_DIR}" >"${RELEASE_FILE}"
echo "    wrote $(grep -c '^kind:' "${RELEASE_FILE}") objects"
