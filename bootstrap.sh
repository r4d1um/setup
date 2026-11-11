#!/bin/sh
# SPDX-License-Identifier: MIT
# Installs r4d1um's prebuilt toolchains: fetches the r4setup tool (version and SHA256 pinned
# below), which downloads only what's missing, checks every checksum and links the commands
# into ~/.local/bin. Which toolchain lives in which repo is in manifest.json.
#
#   curl -fsSL https://raw.githubusercontent.com/r4d1um/setup/<tag>/bootstrap.sh \
#     | R4SETUP_REF=<tag> sh -s -- avr-gcc fakes          # what tools/session-setup.sh does
#   ./bootstrap.sh avr-gcc                                 # from a checkout: its manifest.json
set -eu
TOOL_VERSION=1.1.0   # pinned by the toolchains workflow
TOOL_SHA256=90a2ed13d8f2a0125c1577b719c5110c5361ce14e89e969b9c35160a88de7d6c
REPO=r4d1um/setup

case "$(uname -s)/$(uname -m)" in
  Linux/x86_64) ;;
  *) echo "bootstrap.sh: the toolchains are built for Linux x86_64 only" >&2; exit 1 ;;
esac
home=${R4SETUP_HOME:-${XDG_DATA_HOME:-$HOME/.local/share}/r4setup}
dir=$home/r4setup/$TOOL_VERSION
if [ "$(cat "$dir/.r4setup-sha256" 2>/dev/null)" != "$TOOL_SHA256" ]; then
  mkdir -p "$home/r4setup"
  tmp=$(mktemp -d "$home/.tmp-bootstrap.XXXXXX")
  trap 'rm -rf "$tmp"' EXIT
  asset=r4setup-$TOOL_VERSION-linux-amd64.tar.gz
  curl -fsSL --retry 3 -o "$tmp/$asset" "${R4SETUP_GITHUB:-https://github.com}/$REPO/releases/download/r4setup-$TOOL_VERSION/$asset"
  echo "$TOOL_SHA256  $tmp/$asset" | sha256sum -c --status \
    || { echo "bootstrap.sh: $asset does not match its pinned checksum" >&2; exit 1; }
  mkdir "$tmp/root"
  tar -xzf "$tmp/$asset" -C "$tmp/root"
  echo "$TOOL_SHA256" >"$tmp/root/.r4setup-sha256"
  rm -rf "$dir"
  mv "$tmp/root" "$dir"
fi

if [ -n "${R4SETUP_REF:-}" ]; then
  manifest=https://raw.githubusercontent.com/$REPO/$R4SETUP_REF/manifest.json
elif [ -f "$(dirname "$0")/manifest.json" ]; then
  manifest=$(cd "$(dirname "$0")" && pwd)/manifest.json
else
  echo "bootstrap.sh: set R4SETUP_REF to a tag of $REPO" >&2; exit 1
fi
R4SETUP_HOME=$home exec "$dir/bin/r4setup" install -manifest "$manifest" r4setup "$@"
