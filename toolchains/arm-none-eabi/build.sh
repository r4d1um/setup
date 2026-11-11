#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# arm-none-eabi GCC 13.2 with newlib (+ nano) and libstdc++ from Ubuntu 24.04, relocatable;
# the same compiler the netbook's pico-builder uses (Pico SDK, C and C++).
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
pkgs=(gcc-arm-none-eabi=15:13.2.rel1-2 binutils-arm-none-eabi=2.42-1ubuntu1+23
      libnewlib-arm-none-eabi=4.4.0.20231231-2 libnewlib-dev=4.4.0.20231231-2
      libstdc++-arm-none-eabi-newlib=15:13.2.rel1-2+26 libstdc++-arm-none-eabi-dev=15:13.2.rel1-2+26)
apt_install "${pkgs[@]}" patchelf
r4build debs -root "$root" "${pkgs[@]}"
# what Debian's alternatives (postinst) would link: newlib as the default lib and include
ln -s newlib "$root/lib/arm-none-eabi/lib"
ln -s ../../include/newlib "$root/lib/arm-none-eabi/include"
# debug info in the target libraries is 4/5 of the size (3 GB -> 0.7 GB); one file at a time,
# the libraries are hard-linked between multilib directories
find "$root/lib/arm-none-eabi/newlib" "$root/lib/gcc/arm-none-eabi" -type f \( -name '*.a' -o -name '*.o' \) \
  -exec "$root/bin/arm-none-eabi-strip" --strip-debug {} +
r4build bundle -root "$root"
layout '{"path":["bin"]}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
