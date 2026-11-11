#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
[[ "$("$dir/bin/r4setup" version)" == "$version" ]]
"$dir/bin/r4setup" list -manifest "$setup_src/manifest.json" >/dev/null
