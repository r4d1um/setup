#!/usr/bin/env bash
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
"$dir/bin/p7" --version
"$dir/bin/p7" --help >/dev/null
