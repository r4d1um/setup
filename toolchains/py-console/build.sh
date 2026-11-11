#!/usr/bin/env bash
# Python tools for terminal pictures and shell checks (docker32bit-netbook's console/preview.py
# and its checks): pyte + wcwidth, Pillow (one tree for CPython 3.11–3.13: the extension
# modules carry the version in their names), pyflakes (tools/fix-imports.py) and shellcheck
# (static binary from shellcheck-py).
# Every file comes from PyPI; a new package version means a new VERSION (1.0.0 -> 1.1.0, ...).
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
pythons=3.11,3.12,3.13
r4build wheels -root "$root" -python "$pythons" \
  pyte==0.8.2 wcwidth==0.9.4 pillow==13.0.0 pyflakes==4.1.0 shellcheck-py==0.11.0.1
# the binary inside shellcheck-py is GPL-3.0 ShellCheck: point to its real source too
cat >>"$root/SOURCES.txt" <<'TXT'

bin/shellcheck is ShellCheck 0.11.0 (GPL-3.0), source: https://github.com/koalaman/shellcheck/tree/v0.11.0
TXT
layout '{"path":["bin"],"python":{"dir":"python","versions":["3.11","3.12","3.13"]}}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
