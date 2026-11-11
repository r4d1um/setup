# CLAUDE.md — r4d1um/setup

Central, prebuilt toolchains + shared test fakes for all of r4d1um's repos. Read README.md
first; decisions and their reasons are in DECISIONS.md.

## Map

- `bootstrap.sh` — POSIX sh loader, keep it short (pins `TOOL_VERSION`/`TOOL_SHA256`, written by CI)
- `manifest.json` — contract between this repo and every project: fields only added, never renamed
  (`schema` bumps otherwise). Written by `r4setup pin` / the toolchains workflow, not by hand.
- `cmd/r4setup` — installer (install/list/prefix/pin); `cmd/r4build` — build helper used by
  `toolchains/*/build.sh`; `cmd/fake-mqtt-broker`
- `internal/{manifest,fetch,tgz,install,elfdeps,fakebroker,wheel}` — logic, each package tested;
  I/O (HTTP, gh, apt, pip download, patchelf) only in fetch / r4build / elfdeps
- `toolchains/<name>/` — `VERSION`, `build.sh`, `test.sh`, optional `TIMEOUT`, `defconfig`
- `fakes/python/r4fakes` — shared Python fakes

## Rules

- This repo is **public**: nothing with an unclear licence, vendor blobs or anything
  sensitive — that goes to r4d1um/setup-private (when in doubt: private).
- A toolchain change = new `VERSION` (`-rN` for a repack); the workflow then builds only that one.
- Shell only as short glue (`toolchains/common.sh`); branching/JSON/>50 lines go to Go.
- No third-party Go modules: sessions can't reach proxy.golang.org (stdlib only, gzip not zstd).
- Writes others read go to a temp name, then rename (manifest, installs, links, addr files).
- Actions: Node 24 actions (checkout@v7, setup-go@v7, upload-artifact@v7, download-artifact@v8),
  `timeout-minutes` on every job, paths filters. The build workflow queues instead of
  cancelling (an hour of cross-GCC work would be lost).
- After a release: raise `VERSION` here (the tag workflow creates `v<VERSION>`; sessions can't push tags),
  then raise `R4SETUP_REF` in the projects' `tools/session-setup.sh`.

## Testing here

`go vet ./... && go test -race ./...`, `shellcheck -x bootstrap.sh toolchains/common.sh toolchains/*/*.sh`.
Go is preinstalled in sessions; this repo needs no session-setup itself.
Repack toolchains (avr-gcc, arm-none-eabi) build locally too (archive.ubuntu.com is reachable);
arduino, cahute and the cross GCCs need hosts sessions can't reach — they build in Actions only.
