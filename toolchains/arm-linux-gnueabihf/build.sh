#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Cross GCC for Debian 12 / Pi OS bookworm targets, built with crosstool-ng from defconfig
# (GCC 12, glibc 2.36, so programs run on those systems). The source tarballs are attached
# to the release as <name>-<version>-sources.tar (GPL).
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
ctng=crosstool-ng-1.29.0
work=$(mktemp -d)
apt_install gperf bison flex texinfo help2man gawk libncurses-dev libtool-bin automake autoconf \
  unzip rsync patchelf xz-utils bzip2 wget
git clone -q --depth 1 --branch "$ctng" https://github.com/crosstool-ng/crosstool-ng "$work/ctng"
(cd "$work/ctng" && ./bootstrap >/dev/null && ./configure --enable-local >/dev/null && make -j"$(nproc)" >/dev/null)
mkdir -p "$work/build" "$work/tarballs"
cp "$here/defconfig" "$work/build/defconfig"
cd "$work/build" || exit 1
"$work/ctng/ct-ng" defconfig
tuple=$("$work/ctng/ct-ng" show-tuple)
[[ "$tuple" == "$name" ]] || { echo "defconfig gives $tuple, expected $name" >&2; exit 1; }
export CT_PREFIX=$work/x-tools CT_LOCAL_TARBALLS_DIR=$work/tarballs
sed -i "s|^CT_LOCAL_TARBALLS_DIR=.*|CT_LOCAL_TARBALLS_DIR=\"$work/tarballs\"|; s|^CT_PREFIX_DIR=.*|CT_PREFIX_DIR=\"$work/x-tools/$name\"|" .config
"$work/ctng/ct-ng" "build.$(nproc)" || { tail -n 80 build.log >&2; exit 1; }
root=$work/x-tools/$name
chmod -R u+w "$root"
rm -f "$root/build.log.bz2"
printf 'Built with %s from toolchains/%s/defconfig; the source tarballs are the release asset %s-%s-sources.tar\n' \
  "$ctng" "$name" "$name" "$version" >"$root/SOURCES.txt"
layout '{"path":["bin"]}'
r4build bundle -root "$root"
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
tar -cf "$out/$name-$version-sources.tar" -C "$work/tarballs" .
