# setup

Prebuilt toolchains and shared test fakes for r4d1um's repos, so a cloud session (or a
GitHub Actions job) spends seconds downloading instead of minutes installing or compiling.

```sh
# in a project: tools/session-setup.sh, which pins a tag of this repo
curl -fsSL https://raw.githubusercontent.com/r4d1um/setup/v1.0.0/bootstrap.sh \
  | R4SETUP_REF=v1.0.0 sh -s -- avr-gcc arduino-cli arduino-avr fakes
```

- **only what's missing:** a toolchain whose release (by SHA256) is already in place is
  skipped, so running it again costs nothing
- **checksums:** every archive's SHA256 is pinned in [`manifest.json`](manifest.json), the
  installer's own in [`bootstrap.sh`](bootstrap.sh); a mismatch installs nothing
- **public or private:** `manifest.json` knows which repo holds each toolchain; private ones
  (in [r4d1um/setup-private](https://github.com/r4d1um/setup-private)) are downloaded with `gh api`
  (REST; `gh release download` uses GraphQL, which cloud sessions block)
- commands are linked into `~/.local/bin`, toolchains live in `~/.local/share/r4setup/<name>/<version>`;
  in Actions their `bin/` directories also go to `$GITHUB_PATH`
- **Python packages:** a toolchain with a `python` entry in its `r4setup.json` gets a
  `r4setup-<name>.pth` in the user site of each listed Python (`~/.local/lib/python3.X/site-packages`,
  or under `$PYTHONUSERBASE`), so its packages import without `pip install`

## Toolchains

| name | what | repo |
|---|---|---|
| `avr-gcc` | avr-gcc 7.3, avr-libc, binutils, avrdude 7.1, simavr (Ubuntu 24.04 packages, relocatable) | public |
| `arm-none-eabi` | GCC 13.2 with newlib/newlib-nano and libstdc++ (Ubuntu 24.04, as in the pico-builder), debug info stripped | public |
| `arduino-cli` | arduino-cli + builtin tools (ctags, discoveries, monitor) + board index | public |
| `arduino-avr` | Arduino AVR core with Arduino's avr-gcc/avrdude | public |
| `arduino-esp8266` | ESP8266 core (Espressif SDK blobs) | **private** |
| `cahute` | Cahute (p7, p7os, CaS) for the fx-9860G, libusb/SDL2 bundled | public |
| `aarch64-linux-gnu` | cross GCC 12.5, glibc 2.36: Pi 3/4/5 with 64-bit Debian 12 / Pi OS bookworm | public |
| `arm-linux-gnueabihf` | cross GCC 12.5, glibc 2.36, **ARMv6** + VFP hard float: Pi Zero / Pi 1 (and every later Pi, 32-bit) | public |
| `i686-linux-gnu` | cross GCC 12.5, glibc 2.36: the netbook (32-bit Debian 12) | public |
| `fakes` | shared test fakes, see below | public |
| `py-console` | pyte + wcwidth, Pillow (CPython 3.11–3.13 in one tree), pyflakes 4.1 and shellcheck 0.11, all from PyPI; `import pyte` works without PYTHONPATH (a `.pth` in the user site) | public |
| `r4setup` | the installer itself | public |

`r4setup list -manifest manifest.json` shows versions and what's installed.

The arduino toolchains fill one data directory, `~/.local/share/r4setup/arduino`, through
symlinks; the `arduino-cli` command is a wrapper that points `ARDUINO_DIRECTORIES_DATA` there.
So `arduino-cli compile -b arduino:avr:uno …` works without `downloads.arduino.cc`.

## Shared fakes (`fakes`)

- `fake-esptool` / `python/r4fakes/esptool.py`: prints what esptool prints; `FAKE_ESPTOOL_FAIL`
  (`connect`, `hang`, `nostub`), `FAKE_ESPTOOL_CHIP`, `FAKE_ESPTOOL_LOG`
- `r4fakes.optiboot.FakeOptiboot` / `fake-optiboot --port-file p --image-out img`: an Arduino
  bootloader (STK500 v1) on a pseudo-terminal, for the real avrdude
- `fake-mqtt-broker`: MQTT 3.1.1 broker (login, QoS 0–2, retained, wildcards, will, keep alive)
  that logs every event as a JSON line; `-refuse 4`, `-drop-after N` for failure tests

In Python tests: `sys.path.insert(0, "<prefix>/python")` with `<prefix>` from
`r4setup prefix -manifest … fakes` (or the real path of `fake-esptool`, two levels up).
Repo-specific fakes stay in each repo's `tests/`.

## Adding or updating a toolchain

1. `toolchains/<name>/VERSION` (upstream version + `-rN`), `build.sh OUTDIR`, `test.sh DIR`,
   optional `TIMEOUT` (minutes, default 30). Shared glue: [`toolchains/common.sh`](toolchains/common.sh);
   logic belongs in `cmd/r4build`.
2. Push. The [toolchains workflow](.github/workflows/toolchains.yml) builds only toolchains
   whose release `<name>-<version>` doesn't exist yet, smoke-tests the packed archive,
   creates the release and pins the checksum in `manifest.json` (and `bootstrap.sh` for `r4setup`).
3. Raise [`VERSION`](VERSION): the tag workflow tags `v<VERSION>`. Then raise the pin in the
   projects' `tools/session-setup.sh`.

Private toolchains: same layout in setup-private; after its release,
`go run ./cmd/r4setup pin -manifest manifest.json -name N -repo r4d1um/setup-private -version V -sha S -private`.

Public or private? Public when everything in it is under a license that allows
redistribution (GPL & co. with the source pointer in `SOURCES.txt` / the sources asset).
Unclear licence, vendor blobs or anything sensitive: private.

Every archive carries a `SOURCES.txt` (where the source of each part is) and the licence texts
(`share/licenses/`, `share/doc/<package>/copyright`). `r4build debs`, `wheels`, `bundle` and
`arduino-sources` write them; an Arduino tool without a rule in
[`internal/sources`](internal/sources/sources.go) or a bundled library no Ubuntu package
owns stops the build.

## Layout

```
bootstrap.sh            loader: fetches r4setup (pinned), runs it
manifest.json           name -> repo, version, sha256 (written by the workflow)
cmd/r4setup             installer: install, list, prefix, pin
cmd/r4build             build helper: plan, debs, bundle, pack, unpack
cmd/fake-mqtt-broker    the MQTT fake
internal/               manifest, fetch, tgz, install, elfdeps, fakebroker, wheel, sources (each with tests)
fakes/python/r4fakes    Python fakes
toolchains/<name>/      VERSION, build.sh, test.sh (+ defconfig for the cross GCCs)
```

## License

The code in this repo (installer, build helper, fakes, scripts) is [MIT](LICENSE).
The toolchains in the releases keep their own licences (GPL, LGPL, BSD, …): see
`SOURCES.txt` and the licence files inside each archive.
