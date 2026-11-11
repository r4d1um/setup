#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# avr-gcc 7.3 + avr-libc + binutils, avrdude and simavr from Ubuntu 24.04, relocatable.
# A new package version means a new VERSION (7.3.0-r2, ...).
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
pkgs=(gcc-avr=1:7.3.0+Atmel3.7.0-1 avr-libc=1:2.0.0+Atmel3.7.0-1 binutils-avr=2.26.20160125+Atmel3.7.0-2
      avrdude=7.1+dfsg-3build2 simavr=1.6+dfsg-3build2 libsimavr2=1.6+dfsg-3build2)
apt_install "${pkgs[@]}" patchelf   # the host needs their libraries, bundle copies them in
r4build debs -root "$root" "${pkgs[@]}"
# avrdude reads /etc/avrdude.conf; the wrapper hands it the one in the tree
mkdir -p "$root/libexec"
mv "$root/bin/avrdude" "$root/libexec/avrdude"
# shellcheck disable=SC2016  # the wrapper expands these itself
printf '#!/bin/sh\nhere=$(dirname "$(readlink -f "$0")")\nexec "$here/../libexec/avrdude" -C "$here/../etc/avrdude.conf" "$@"\n' >"$root/bin/avrdude"
chmod 0755 "$root/bin/avrdude"
r4build bundle -root "$root"
layout '{"path":["bin"]}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
