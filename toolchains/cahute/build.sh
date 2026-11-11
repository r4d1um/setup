#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Cahute (p7, p7os, CaS, ...) for the Casio fx-9860G, built from source; libusb goes into the
# tree. p7screen (screen streaming, needs SDL2 and a desktop) is left out.
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
src=$(mktemp -d)
apt_install cmake pkg-config python3 python3-toml libusb-1.0-0-dev libsdl2-dev patchelf
git clone -q --depth 1 --branch "${version%-r*}" https://gitlab.com/cahuteproject/cahute.git "$src/cahute"
cmake -S "$src/cahute" -B "$src/build" -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr >/dev/null
cmake --build "$src/build" -j"$(nproc)"
DESTDIR=$src/stage cmake --install "$src/build" --strip
cp -a "$src/stage/usr/." "$root/"
rm -rf "$root/bin/p7screen" "$root/lib/udev" "$root/lib/pkgconfig" "$root/include" "$root/share/man"
mkdir -p "$root/share/doc/cahute"
cp "$src/cahute/LICENSE"* "$root/share/doc/cahute/"   # no licence file: stop, don't ship without
printf 'Cahute %s, source: https://gitlab.com/cahuteproject/cahute/-/tree/%s\n' "${version%-r*}" "${version%-r*}" >"$root/SOURCES.txt"
r4build bundle -root "$root"   # also adds libusb & co. (LGPL) to SOURCES.txt
layout '{"path":["bin"]}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
