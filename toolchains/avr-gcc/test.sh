#!/usr/bin/env bash
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
tmp=$(mktemp -d)
cat >"$tmp/blink.c" <<'C'
#include <avr/io.h>
#include <util/delay.h>
int main(void) { DDRB |= _BV(PB5); for (;;) { PORTB ^= _BV(PB5); _delay_ms(500); } }
C
"$dir/bin/avr-gcc" -mmcu=atmega328p -DF_CPU=16000000UL -Os -o "$tmp/blink.elf" "$tmp/blink.c"
"$dir/bin/avr-objcopy" -O ihex "$tmp/blink.elf" "$tmp/blink.hex"
"$dir/bin/avr-size" "$tmp/blink.elf"
# avrdude writes the program through the repo's fake Arduino bootloader
python3 "$setup_src/fakes/python/r4fakes/optiboot.py" --port-file "$tmp/port" --image-out "$tmp/image" &
boot=$!
for _ in $(seq 50); do [[ -s "$tmp/port" ]] && break; sleep 0.1; done
"$dir/bin/avrdude" -p atmega328p -c arduino -P "$(cat "$tmp/port")" -b 115200 -D -U "flash:w:$tmp/blink.hex:i"
kill "$boot"
wait "$boot" || true
"$dir/bin/avr-objcopy" -I ihex -O binary "$tmp/blink.hex" "$tmp/blink.bin"
cmp -n "$(stat -c %s "$tmp/blink.bin")" "$tmp/blink.bin" "$tmp/image"  # avrdude pads to full pages
cores=$("$dir/bin/simavr" --list-cores 2>&1 || true)
grep -q atmega328p <<<"$cores"
