#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# The Arduino AVR core (Uno, Nano, ...) with its avr-gcc, avrdude and arduinoOTA, installed by
# arduino-cli; r4setup links it into <R4SETUP_HOME>/arduino/packages/arduino.
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
tmp=$(mktemp -d)
"$here/../arduino-cli/fetch.sh" "$tmp"
export ARDUINO_DIRECTORIES_DATA=$tmp/data ARDUINO_DIRECTORIES_DOWNLOADS=$tmp/dl ARDUINO_DIRECTORIES_USER=$tmp/user
"$tmp/arduino-cli" core update-index
"$tmp/arduino-cli" core install "arduino:avr@${version%-r*}"
mkdir -p "$root/packages"
mv "$tmp/data/packages/arduino" "$root/packages/"
# LGPL core, GPL tools: SOURCES.txt with the source of each, licence texts in share/licenses
r4build arduino-sources -root "$root" -packages "$root/packages"
layout '{"links":{"arduino/packages/arduino":"packages/arduino"}}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
