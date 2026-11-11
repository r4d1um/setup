// SPDX-License-Identifier: MIT

package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r4d1um/setup/internal/manifest"
)

func TestPublicDownloadAndVerify(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/r4d1um/setup/releases/download/avr-gcc-1/avr-gcc-1-linux-amd64.tar.gz" {
			w.Write([]byte("payload"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := GitHub{BaseURL: srv.URL}
	dst := filepath.Join(t.TempDir(), "a")
	tc := manifest.Toolchain{Name: "avr-gcc", Repo: "r4d1um/setup", Version: "1"}
	if err := g.Download(context.Background(), tc, dst); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dst, "239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dst, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong checksum accepted")
	}
	// a missing release is not retried
	paths = nil
	tc.Version = "2"
	if err := g.Download(context.Background(), tc, dst); err == nil || len(paths) != 1 {
		t.Fatalf("err=%v, %d requests", err, len(paths))
	}
}

func TestPrivateNeedsGh(t *testing.T) {
	g := GitHub{GH: "/nonexistent/gh"}
	err := g.Download(context.Background(), manifest.Toolchain{Name: "x", Repo: "r4d1um/setup-private", Version: "1", Private: true}, filepath.Join(t.TempDir(), "a"))
	if err == nil || !strings.Contains(err.Error(), "gh CLI") {
		t.Fatalf("err = %v", err)
	}
}

func TestPrivateUsesGhAPI(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	gh := filepath.Join(dir, "gh")
	// fake gh: answers the release lookup with an asset id, the asset request with its bytes
	os.WriteFile(gh, []byte(`#!/bin/sh
echo "$@" >> `+log+`
case "$*" in
  *releases/tags/*) echo 4711 ;;
  *releases/assets/4711*) printf data ;;
  *) exit 1 ;;
esac
`), 0o755)
	dst := filepath.Join(dir, "asset")
	tc := manifest.Toolchain{Name: "arduino-esp8266", Repo: "r4d1um/setup-private", Version: "3.1.2-r1", Private: true}
	if err := (GitHub{GH: gh}).Download(context.Background(), tc, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "data" {
		t.Fatalf("asset %q", b)
	}
	args, _ := os.ReadFile(log)
	want := `api repos/r4d1um/setup-private/releases/tags/arduino-esp8266-3.1.2-r1 -q .assets[] | select(.name == "arduino-esp8266-3.1.2-r1-linux-amd64.tar.gz") | .id
api -H Accept: application/octet-stream repos/r4d1um/setup-private/releases/assets/4711
`
	if string(args) != want {
		t.Fatalf("gh calls:\n%s", args)
	}
	tc.Version = "9"
	os.WriteFile(gh, []byte("#!/bin/sh\nexit 0\n"), 0o755) // no such asset: empty answer
	if err := (GitHub{GH: gh}).Download(context.Background(), tc, dst); err == nil {
		t.Fatal("missing asset not reported")
	}
}
