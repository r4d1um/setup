# Decisions

**2026-11-11 — Repack Ubuntu 24.04 packages for avr-gcc and arm-none-eabi.** Same compilers
as the netbook builders; quick to build; source is on Launchpad (SOURCES.txt). Not Arm's
own arm-none-eabi tarball: its redistribution terms are unclear for a public repo.
arm-none-eabi target libraries are stripped of debug info (3 GB → 0.7 GB unpacked, 170 MB download).

**2026-11-11 — Cross GCCs with crosstool-ng, glibc 2.36.** Ubuntu's cross packages link
against glibc 2.39 (binaries wouldn't run on Debian 12 / Pi OS bookworm) and Debian's armhf
is ARMv7 (no Pi Zero / Pi 1). ct-ng builds self-contained toolchains; armhf is ARMv6 + VFP
hard float, which runs on every Pi with a 32-bit OS.

**2026-11-11 — r4setup in Go, bootstrap.sh only a loader.** The install logic (manifest,
checksums, links, locking) is too much for shell. bootstrap.sh pins the tool's version and
SHA256; the tool is released like any toolchain.

**2026-11-11 — tar.gz, stdlib only.** Sessions can't reach proxy.golang.org, so no
third-party modules (zstd would need one).

**2026-11-11 — Host libraries bundled with $ORIGIN RUNPATHs.** Toolchains carry what Ubuntu
24.04 doesn't have by default (libmpc, libusb, …) in lib/host; glibc, libstdc++ & co. stay outside.

**2026-11-11 — arduino-esp8266 private.** Espressif's SDK libraries are licensed for use
with Espressif chips only; when in doubt, private. Same for the ESP32 core later.

**2026-11-11 — The release workflow queues (cancel-in-progress: false).** A newer push would
otherwise kill an hour-long cross build that has to be redone anyway.

**2026-11-11 — Python tools from PyPI wheels, unpacked by r4build, found through a .pth.**
`r4build wheels` downloads pinned binary wheels (no pip install, no venv) and unpacks them into one
tree; Pillow's extension modules carry the Python version in their names, so one archive serves
CPython 3.11 (the netbook's) to 3.13. r4setup writes a `.pth` into each listed Python's user site
instead of asking projects to set PYTHONPATH, which doesn't survive between shell calls.

**2026-11-11 — Bare Xtensa GCCs public, next to the private cores.** The IoT libraries
(r4d1um/iot-lib-public, a public repo whose CI can't read setup-private) ship an Arduino-free
core as a prebuilt `.a` per target. For the ESP8266 and ESP32 that needs only the compiler, not the
Espressif SDK, so `xtensa-lx106-elf` and `xtensa-esp32-elf` are public (GPL, source pointers in
SOURCES.txt). ESP8266: exactly the core 3.1.2 compiler. ESP32: GCC 8.4 of core 2.0.17 (the netbook's
newest 32-bit core) on purpose; objects from the older GCC also link with core 3.x's GCC 14.
