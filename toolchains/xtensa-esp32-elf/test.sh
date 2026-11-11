#!/usr/bin/env bash
# C and C++ with the ESP32 core's flags, into an archive
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
tmp=$(mktemp -d)
printf 'int add(int a, int b) { return a + b; }\n' >"$tmp/a.c"
printf 'struct S { int v; int get() const { return v; } };\nint twice(int x) { S s{x}; return 2 * s.get(); }\n' >"$tmp/b.cpp"
f=(-Os -mlongcalls -mtext-section-literals -falign-functions=4 -ffunction-sections -fdata-sections)
"$dir/bin/xtensa-esp32-elf-gcc" "${f[@]}" -c "$tmp/a.c" -o "$tmp/a.o"
"$dir/bin/xtensa-esp32-elf-g++" "${f[@]}" -std=gnu++17 -fno-exceptions -fno-rtti -c "$tmp/b.cpp" -o "$tmp/b.o"
"$dir/bin/xtensa-esp32-elf-gcc-ar" rcsD "$tmp/libt.a" "$tmp/a.o" "$tmp/b.o"
"$dir/bin/xtensa-esp32-elf-gcc" --version | grep -q ' 8\.4\.0'
"$dir/bin/xtensa-esp32-elf-objdump" -f "$tmp/a.o" | grep -q 'xtensa'
