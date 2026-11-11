#!/usr/bin/env bash
# arduino-cli + its builtin tools (ctags, serial/mdns discovery, serial monitor) and the board
# index, so compiling needs nothing from downloads.arduino.cc. The cores come as their own
# toolchains (arduino-avr, arduino-esp8266, ...); r4setup links everything into one data
# directory, <R4SETUP_HOME>/arduino, which the bin/arduino-cli wrapper uses.
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
tmp=$(mktemp -d)
mkdir -p "$root/bin" "$root/libexec" "$root/data"
"$here/fetch.sh" "$root/libexec"
export ARDUINO_DIRECTORIES_DATA=$root/data ARDUINO_DIRECTORIES_DOWNLOADS=$tmp/dl ARDUINO_DIRECTORIES_USER=$tmp/user
"$root/libexec/arduino-cli" core update-index
"$root/libexec/arduino-cli" core install arduino:avr  # brings the builtin tools along
rm -rf "$root/data/packages/arduino"                 # the core itself is the arduino-avr toolchain
test -x "$root/data/packages/builtin/tools/ctags/"*/ctags
cat >"$root/bin/arduino-cli" <<'SH'
#!/bin/sh
# arduino-cli with the data directory r4setup assembles: <R4SETUP_HOME>/arduino
here=$(dirname "$(readlink -f "$0")")
data=$(cd "$here/../../.." && pwd)/arduino
export ARDUINO_DIRECTORIES_DATA="${ARDUINO_DIRECTORIES_DATA:-$data}"
export ARDUINO_DIRECTORIES_DOWNLOADS="${ARDUINO_DIRECTORIES_DOWNLOADS:-$data/staging}"
exec "$here/../libexec/arduino-cli" "$@"
SH
chmod 0755 "$root/bin/arduino-cli"
layout '{"path":["bin"],"links":{"arduino/package_index.json":"data/package_index.json","arduino/packages/builtin":"data/packages/builtin"}}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
