#!/usr/bin/env bash
# Cahute (p7, p7os, CaS, ...) for the Casio fx-9860G, built from source; libusb and SDL2 go
# into the tree, so the host needs neither.
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
src=$(mktemp -d)
apt_install cmake pkg-config python3 python3-toml libusb-1.0-0-dev libsdl2-dev patchelf
git clone -q --depth 1 --branch "${version%-r*}" https://gitlab.com/cahuteproject/cahute.git "$src/cahute"
cmake -S "$src/cahute" -B "$src/build" -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/ >/dev/null
cmake --build "$src/build" -j"$(nproc)"
DESTDIR=$root cmake --install "$src/build" --strip
rm -rf "$root/lib/udev" "$root/lib/pkgconfig" "$root/include" "$root/share/man"
mkdir -p "$root/share/doc/cahute"
cp "$src/cahute/LICENSE"* "$root/share/doc/cahute/" 2>/dev/null || true
printf 'Cahute %s, source: https://gitlab.com/cahuteproject/cahute/-/tree/%s\n' "${version%-r*}" "${version%-r*}" >"$root/SOURCES.txt"
r4build bundle -root "$root"
layout '{"path":["bin"]}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
