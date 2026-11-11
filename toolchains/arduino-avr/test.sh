#!/usr/bin/env bash
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
