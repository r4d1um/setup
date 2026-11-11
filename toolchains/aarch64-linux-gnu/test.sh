#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# hello world in C and C++ for the target; nothing may need a glibc newer than 2.36
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
tmp=$(mktemp -d)
printf '#include <stdio.h>\n#include <stdlib.h>\nint main(int c, char **v) { printf("%%ld\\n", strtol(c > 1 ? v[1] : "42", 0, 10)); return 0; }\n' >"$tmp/hello.c"
printf '#include <iostream>\n#include <string>\nint main() { std::cout << std::to_string(42) << std::endl; }\n' >"$tmp/hello.cpp"
"$dir/bin/$name-gcc" -O2 -D_GNU_SOURCE -o "$tmp/hello" "$tmp/hello.c"
"$dir/bin/$name-g++" -O2 -o "$tmp/hellocpp" "$tmp/hello.cpp"
for b in hello hellocpp; do
  file "$tmp/$b"
  if "$dir/bin/$name-objdump" -T "$tmp/$b" | grep -oE 'GLIBC_2\.(3[7-9]|[4-9][0-9])' ; then
    echo "$b needs a glibc newer than 2.36" >&2; exit 1
  fi
done
case $name in
  arm-linux-gnueabihf) "$dir/bin/$name-readelf" -A "$tmp/hello" | grep -q "Tag_CPU_arch: v6" ;;   # Pi Zero / Pi 1
  aarch64-linux-gnu)   file "$tmp/hello" | grep -q "ARM aarch64" ;;
  i686-linux-gnu)      file "$tmp/hello" | grep -q "Intel 80386"
                       sysroot=$dir/$name/sysroot  # runs here too, with the sysroot's loader
                       "$sysroot/lib/ld-linux.so.2" --library-path "$sysroot/lib" "$tmp/hello" 7 | grep -qx 7 ;;
esac
