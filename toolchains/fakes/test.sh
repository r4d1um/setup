#!/usr/bin/env bash
source "$(dirname "$0")/../common.sh"
dir=${1:?usage: test.sh DIR}
tmp=$(mktemp -d)
"$dir/bin/fake-esptool" --port /dev/null flash_id | grep -q "Detected flash size"
PYTHONPATH="$dir/python" python3 -c "from r4fakes.optiboot import FakeOptiboot; b = FakeOptiboot(); print(b.port); b.close()"
"$dir/bin/fake-mqtt-broker" -listen 127.0.0.1:0 -addr-file "$tmp/addr" -log "$tmp/log" &
pid=$!
for _ in $(seq 50); do [[ -s "$tmp/addr" ]] && break; sleep 0.1; done
kill "$pid"
wait "$pid" || true
grep -q 127.0.0.1 "$tmp/addr"
