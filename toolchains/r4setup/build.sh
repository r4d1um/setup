#!/usr/bin/env bash
# The installer itself; bootstrap.sh fetches this archive (version + sha256 pinned in it).
source "$(dirname "$0")/../common.sh"
out=${1:?usage: build.sh OUTDIR}
root=$(mktemp -d)
(cd "$setup_src" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$root/bin/r4setup" ./cmd/r4setup)
layout '{"path":["bin"]}'
r4build pack -name "$name" -version "$version" -root "$root" -out "$out"
