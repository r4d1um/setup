# shellcheck shell=bash disable=SC2034  # name/version/root are used by the scripts sourcing this
# SPDX-License-Identifier: MIT
# Sourced by toolchains/<name>/build.sh and test.sh (also from r4d1um/setup-private,
# which checks this repo out): short glue only, the logic lives in cmd/r4build.
#   build.sh OUTDIR   -> OUTDIR/<name>-<version>-linux-amd64.tar.gz (+ .sha256, extra assets)
#   test.sh  DIR      -> smoke test of the unpacked archive in DIR
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[1]}")" && pwd)
name=$(basename "$here")
version=$(cat "$here/VERSION")
setup_src=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
r4build() { (cd "$setup_src" && go run ./cmd/r4build "$@"); }
# layout '{"path":["bin"]}' writes the toolchain's r4setup.json into $root
layout() { printf '%s\n' "$1" >"${root:?}/r4setup.json"; }
# apt_install pkg... : install on the build host (root in containers, sudo on runners)
apt_install() {
  local s=()
  [[ $(id -u) == 0 ]] || s=(sudo)
  "${s[@]}" apt-get update -qq
  DEBIAN_FRONTEND=noninteractive "${s[@]}" apt-get install -y -qq --no-install-recommends "$@" >/dev/null
}
