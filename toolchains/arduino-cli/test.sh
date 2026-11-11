#!/usr/bin/env bash
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
"$dir/libexec/arduino-cli" version
ARDUINO_DIRECTORIES_DATA=$dir/data "$dir/bin/arduino-cli" core list
ctags=("$dir/data/packages/builtin/tools/ctags/"*/ctags)
"${ctags[0]}" --version | grep -q "Exuberant Ctags"
test -s "$dir/data/library_index.json"
