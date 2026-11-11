#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
"$dir/libexec/arduino-cli" version
ARDUINO_DIRECTORIES_DATA=$dir/data "$dir/bin/arduino-cli" core list
ctags=("$dir/data/packages/builtin/tools/ctags/"*/ctags)
"${ctags[0]}" --version | grep -q "Exuberant Ctags"
test -s "$dir/data/library_index.json"
# what the GPL asks for: source pointer per tool and the licence texts
grep -q '^arduino-cli .* (GPL-3.0)  https://github.com/arduino/arduino-cli/tree/v' "$dir/SOURCES.txt"
grep -q '^ctags .* (GPL-2.0)' "$dir/SOURCES.txt"
test -s "$dir/share/doc/arduino-cli/LICENSE.txt"
test -s "$dir/share/licenses/GPL-3.0.txt"
test -s "$dir/share/licenses/GPL-2.0.txt"
