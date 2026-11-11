#!/usr/bin/env bash
# Shared test fakes: fake-esptool, fake-optiboot (Python, r4fakes package) and fake-mqtt-broker (Go).
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
mkdir -p "$root/bin" "$root/python"
cp -a "$setup_src/fakes/python/r4fakes" "$root/python/"
(cd "$setup_src" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$root/bin/fake-mqtt-broker" ./cmd/fake-mqtt-broker)
for f in esptool optiboot; do
  # shellcheck disable=SC2016  # the wrapper expands these itself
  printf '#!/bin/sh\nexec python3 "$(dirname "$(readlink -f "$0")")/../python/r4fakes/%s.py" "$@"\n' "$f" >"$root/bin/fake-$f"
  chmod 0755 "$root/bin/fake-$f"
done
layout '{"path":["bin"]}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
