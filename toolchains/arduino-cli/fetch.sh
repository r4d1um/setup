#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# fetch.sh DIR: puts the pinned arduino-cli binary and its LICENSE.txt (GPL-3.0; checksum checked)
# into DIR (also used by arduino-*).
set -euo pipefail
v=$(sed 's/-r[0-9]*$//' "$(dirname "$0")/VERSION")
dir=${1:?usage: fetch.sh DIR}
tmp=$(mktemp -d)
base=https://github.com/arduino/arduino-cli/releases/download/v$v
curl -fsSL --retry 3 -o "$tmp/arduino-cli_${v}_Linux_64bit.tar.gz" "$base/arduino-cli_${v}_Linux_64bit.tar.gz"
curl -fsSL --retry 3 -o "$tmp/sums" "$base/${v}-checksums.txt"
(cd "$tmp" && grep " arduino-cli_${v}_Linux_64bit.tar.gz\$" sums | sha256sum -c --quiet -)
tar -xzf "$tmp/arduino-cli_${v}_Linux_64bit.tar.gz" -C "$dir" arduino-cli LICENSE.txt
rm -rf "$tmp"
