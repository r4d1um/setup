#!/usr/bin/env bash
# ESP8266 GCC 10.3 (earlephilhower/esp-quick-toolchain 3.1.0), the exact compiler of the ESP8266
# Arduino core 3.1.2. Only the compiler with newlib/libstdc++ (GPL & co.), no Espressif SDK, so it
# can be public; libraries build their Arduino-free .a for the ESP8266 with it.
# Upstream sources: https://github.com/earlephilhower/esp-quick-toolchain (SOURCES.txt).
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
asset=x86_64-linux-gnu.xtensa-lx106-elf-e5f9fec.220621.tar.gz
url=https://github.com/earlephilhower/esp-quick-toolchain/releases/download/3.1.0-gcc10.3/$asset
sha=89d21f38a516e34e0bb44df7e30ef6b1bc2f7f125aae3b7986e5cae545a3ed83  # = package_esp8266com_index.json of core 3.1.2
tmp=$(mktemp -d)
curl -fsSL --retry 3 -o "$tmp/$asset" "$url"
echo "$sha  $tmp/$asset" | sha256sum -c --quiet -
tar -xzf "$tmp/$asset" -C "$tmp"
root=$tmp/xtensa-lx106-elf
printf 'esp-quick-toolchain 3.1.0-gcc10.3 (%s), sources: https://github.com/earlephilhower/esp-quick-toolchain/tree/3.1.0-gcc10.3\n' "$url" >"$root/SOURCES.txt"
layout '{"path":["bin"]}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
