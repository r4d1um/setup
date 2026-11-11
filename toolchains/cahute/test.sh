#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
"$dir/bin/p7" --version
"$dir/bin/p7" --help >/dev/null
# Cahute's licence and, for the bundled libusb (LGPL-2.1), its package, source and copyright
compgen -G "$dir/share/doc/cahute/LICENSE*" >/dev/null
grep -q '^libusb-1.0-0.*https://launchpad.net/ubuntu/+source/libusb-1.0/' "$dir/SOURCES.txt"
test -s "$dir/share/doc/libusb-1.0-0/copyright"
