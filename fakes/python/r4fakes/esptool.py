#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Stands in for esptool in the tests: prints what the real one prints for the
commands the server uses. FAKE_ESPTOOL_FAIL=connect makes it fail like a board
that doesn't answer, FAKE_ESPTOOL_FAIL=hang never ends, FAKE_ESPTOOL_FAIL=nostub like Debian's esptool package without its stub
flasher (until --no-stub is given); FAKE_ESPTOOL_LOG gets one line per call; FAKE_ESPTOOL_CHIP is
the chip it finds (default ESP8266EX), and with --chip set to another one it fails like esptool.

Shared fake from r4d1um/setup (toolchain "fakes"); run it as `fake-esptool` or
[sys.executable, <fakes>/python/r4fakes/esptool.py]."""
import os
import re
import sys
import time

args = sys.argv[1:]
if os.environ.get("FAKE_ESPTOOL_LOG"):
    with open(os.environ["FAKE_ESPTOOL_LOG"], "a") as f:
        f.write(" ".join(args) + "\n")
print("esptool.py v4.8.1")
print(f"Serial port {args[args.index('--port') + 1]}")
print("Connecting....")
if os.environ.get("FAKE_ESPTOOL_FAIL") == "hang":  # a port another program holds, a stuck adapter
    sys.stdout.flush()
    time.sleep(600)
if os.environ.get("FAKE_ESPTOOL_FAIL") == "connect":
    print("\nA fatal error occurred: Failed to connect to ESP8266: No serial data received.")
    sys.exit(2)
if os.environ.get("FAKE_ESPTOOL_FAIL") == "nostub" and "--no-stub" not in args:
    print("Chip is ESP8266EX\nUploading stub...")
    print("Traceback (most recent call last):")
    print('  File "/usr/lib/python3/dist-packages/esptool/loader.py", line 159, in __init__')
    print("FileNotFoundError: [Errno 2] No such file or directory: "
          "'/usr/lib/python3/dist-packages/esptool/targets/stub_flasher/stub_flasher_8266.json'")
    sys.exit(1)
chip = os.environ.get("FAKE_ESPTOOL_CHIP", "ESP8266EX")
want = args[args.index("--chip") + 1] if "--chip" in args else "auto"
m = re.match(r"ESP32-([SCHP]\d+)", chip)
chip_id = "esp8266" if chip.startswith("ESP8266") else "esp32" + m.group(1).lower() if m else "esp32"
if want != "auto" and want != chip_id:
    print(f"\nA fatal error occurred: This chip is {chip.split()[0]}, not {want.upper()}. Wrong --chip argument?")
    sys.exit(2)
print(f"Chip is {chip}")
print("Features: WiFi")
print("Crystal is 26MHz")
print("MAC: 48:3f:da:1a:2b:3c")
if "flash_id" in args:
    print("Manufacturer: 20\nDevice: 4016\nDetected flash size: 4MB")
elif "erase_flash" in args and "--no-stub" in args:
    print("Erasing flash (this may take a while)...")
    print("\nA fatal error occurred: ESP8266 ROM does not support function erase_flash.")
    sys.exit(2)
elif "erase_flash" in args:
    print("Erasing flash (this may take a while)...\nChip erase completed successfully in 2.1s")
elif "write_flash" in args:
    image = args[-1]
    size = os.path.getsize(image)
    print(f"Compressed {size} bytes to {size // 2}...")
    for pct in range(0, 101, 5):
        sys.stdout.write(f"\rWriting at 0x{pct * 1000:08x}... ({pct} %)")
        sys.stdout.flush()
        time.sleep(0.005)
    print(f"\nWrote {size} bytes at 0x00000000 in 1.0 seconds")
    print("Hash of data verified.")
print("\nLeaving...\nHard resetting via RTS pin...")
