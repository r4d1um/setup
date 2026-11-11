#!/usr/bin/env bash
# C and C++ with the ESP8266 core's flags, into an archive
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
tmp=$(mktemp -d)
printf 'int add(int a, int b) { return a + b; }\n' >"$tmp/a.c"
printf 'struct S { int v; int get() const { return v; } };\nint twice(int x) { S s{x}; return 2 * s.get(); }\n' >"$tmp/b.cpp"
f=(-Os -mlongcalls -mtext-section-literals -falign-functions=4 -ffunction-sections -fdata-sections)
"$dir/bin/xtensa-lx106-elf-gcc" "${f[@]}" -c "$tmp/a.c" -o "$tmp/a.o"
"$dir/bin/xtensa-lx106-elf-g++" "${f[@]}" -std=gnu++17 -fno-exceptions -fno-rtti -c "$tmp/b.cpp" -o "$tmp/b.o"
"$dir/bin/xtensa-lx106-elf-gcc-ar" rcsD "$tmp/libt.a" "$tmp/a.o" "$tmp/b.o"
"$dir/bin/xtensa-lx106-elf-gcc" --version | grep -q ' 10\.3\.'
"$dir/bin/xtensa-lx106-elf-objdump" -f "$tmp/a.o" | grep -q 'xtensa'
