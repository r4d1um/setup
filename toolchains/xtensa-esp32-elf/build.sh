#!/usr/bin/env bash
# ESP32 GCC 8.4 (Espressif crosstool-NG esp-2021r2-patch5), the compiler of the ESP32 Arduino core
# 2.0.17. Libraries build their Arduino-free ESP32 .a with this older GCC so the archive links with
# core 2.0.17 (netbook) and with core 3.x (GCC 14) alike. GPL & co., no Espressif SDK: public.
# The checksum is checked against Espressif's own board index, which lists this archive.
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
rel=esp-2021r2-patch5
asset=xtensa-esp32-elf-gcc8_4_0-$rel-linux-amd64.tar.gz
url=https://github.com/espressif/crosstool-NG/releases/download/$rel/$asset
sha=8ef14e0409c2011b41e504a30f70d3e35287313a795d1f2462ad2cd0e2052d37
tmp=$(mktemp -d)
curl -fsSL --retry 3 -o "$tmp/$asset" "$url"
echo "$sha  $tmp/$asset" | sha256sum -c --quiet -
curl -fsSL --retry 3 -o "$tmp/index.json" https://espressif.github.io/arduino-esp32/package_esp32_index.json
grep -q "SHA-256:$sha" "$tmp/index.json" || { echo "$asset: checksum not in Espressif's board index" >&2; exit 1; }
tar -xzf "$tmp/$asset" -C "$tmp"
root=$tmp/xtensa-esp32-elf
printf 'Espressif crosstool-NG %s (%s), sources: https://github.com/espressif/crosstool-NG/tree/%s\n' "$rel" "$url" "$rel" >"$root/SOURCES.txt"
layout '{"path":["bin"]}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
