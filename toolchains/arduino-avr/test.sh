#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# compiles Blink against the unpacked core, the way the arduino data directory links it in
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
tmp=$(mktemp -d)
"$here/../arduino-cli/fetch.sh" "$tmp"
mkdir -p "$tmp/data/packages" "$tmp/sketch/Blink"
ln -s "$dir/packages/arduino" "$tmp/data/packages/arduino"
export ARDUINO_DIRECTORIES_DATA=$tmp/data ARDUINO_DIRECTORIES_DOWNLOADS=$tmp/dl ARDUINO_DIRECTORIES_USER=$tmp/user
printf 'void setup() { pinMode(LED_BUILTIN, OUTPUT); }\nvoid loop() { digitalWrite(LED_BUILTIN, !digitalRead(LED_BUILTIN)); delay(500); }\n' >"$tmp/sketch/Blink/Blink.ino"
"$tmp/arduino-cli" core update-index
"$tmp/arduino-cli" compile --fqbn arduino:avr:uno --output-dir "$tmp/build" "$tmp/sketch/Blink"
test -s "$tmp/build/Blink.ino.hex"
# what the (L)GPL asks for: source pointer per part and the licence texts
grep -q "^avr ${version%-r*} (LGPL-2.1)  https://github.com/arduino/ArduinoCore-avr/tree/" "$dir/SOURCES.txt"
grep -q '^avr-gcc .* (GPL-3.0)' "$dir/SOURCES.txt"
grep -q '^avrdude .* (GPL-2.0)' "$dir/SOURCES.txt"
for l in LGPL-2.1 GPL-3.0 GPL-2.0; do test -s "$dir/share/licenses/$l.txt"; done
