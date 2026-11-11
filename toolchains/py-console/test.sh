#!/usr/bin/env bash
# Every Python the tree is built for (that this host has) imports the packages and draws
# a terminal screen through pyte into a PNG; pyflakes and shellcheck each find a real mistake.
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
tmp=$(mktemp -d)
ran=0
for py in python3.11 python3.12 python3.13; do
  command -v "$py" >/dev/null || continue
  PYTHONPATH="$dir/python" "$py" - "$tmp/$py.png" <<'PY'
import sys, pyte, wcwidth
from PIL import Image, ImageDraw
screen = pyte.Screen(20, 2)
pyte.Stream(screen).feed("\x1b[1;32mok\x1b[0m ✓")
assert screen.display[0].startswith("ok ✓"), screen.display
assert wcwidth.wcswidth("✓") == 1
img = Image.new("RGB", (160, 32))
ImageDraw.Draw(img).text((2, 2), screen.display[0], fill=(0, 255, 0))
img.save(sys.argv[1])
assert Image.open(sys.argv[1]).size == (160, 32)
print(sys.version.split()[0], "Pillow", Image.__version__)
PY
  # pyflakes (as tools/fix-imports.py uses it) reports the unused import and the undefined name
  printf 'import os\nprint(json)\n' >"$tmp/bad.py"
  out=$(PYTHONPATH="$dir/python" "$py" -m pyflakes "$tmp/bad.py" || true)
  if ! grep -q "'os' imported but unused" <<<"$out" || ! grep -q "undefined name 'json'" <<<"$out"; then
    echo "$py: pyflakes missed a mistake: $out" >&2; exit 1
  fi
  ran=$((ran + 1))
done
[[ $ran -gt 0 ]] || { echo "no python3.11-3.13 on this host" >&2; exit 1; }
"$dir/bin/shellcheck" --version | grep -q 'version: 0.11.0'
# shellcheck disable=SC2016  # the unquoted $1 is the mistake it must find
printf '#!/bin/sh\necho $1\n' >"$tmp/bad.sh"
if "$dir/bin/shellcheck" "$tmp/bad.sh" >/dev/null; then echo "shellcheck missed SC2086" >&2; exit 1; fi
grep -q koalaman "$dir/SOURCES.txt"
