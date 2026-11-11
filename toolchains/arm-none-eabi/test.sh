#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
tmp=$(mktemp -d)
cat >"$tmp/main.cpp" <<'C'
#include <cstdio>
#include <vector>
int main() { std::vector<int> v{1, 2, 3}; std::printf("%d\n", v[2]); return 0; }
C
for cpu in cortex-m0plus cortex-m33; do  # RP2040, RP2350
  "$dir/bin/arm-none-eabi-g++" -mcpu=$cpu -mthumb -Os --specs=nano.specs --specs=nosys.specs \
    -fno-exceptions -o "$tmp/main-$cpu.elf" "$tmp/main.cpp"
  "$dir/bin/arm-none-eabi-size" "$tmp/main-$cpu.elf"
done
"$dir/bin/arm-none-eabi-gcc" -mcpu=cortex-m0plus -mthumb -print-file-name=libc_nano.a | grep -q "^$dir"
# every bundled host library has its package, source and copyright (LGPL/GPL)
grep -q '^libmpc3:amd64 .*https://launchpad.net/ubuntu/+source/mpclib3/' "$dir/SOURCES.txt"
test -s "$dir/share/doc/libmpc3/copyright"
