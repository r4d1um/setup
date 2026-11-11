// SPDX-License-Identifier: MIT

package manifest

import (
	"strings"
	"testing"
)

const sha = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestParseAndFind(t *testing.T) {
	m, err := Parse([]byte(`{"schema":1,"toolchains":[
	 {"name":"avr-gcc","repo":"r4d1um/setup","version":"7.3.0-r1","sha256":"` + sha + `"},
	 {"name":"arduino-esp8266","repo":"r4d1um/setup-private","version":"3.1.2-r1","sha256":"","private":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	tc, ok := m.Find("avr-gcc")
	if !ok || tc.Tag() != "avr-gcc-7.3.0-r1" || tc.Asset() != "avr-gcc-7.3.0-r1-linux-amd64.tar.gz" {
		t.Fatalf("got %+v", tc)
	}
	if p, _ := m.Find("arduino-esp8266"); !p.Private {
		t.Fatal("private flag lost")
	}
	if _, ok := m.Find("nope"); ok {
		t.Fatal("found a toolchain that is not there")
	}
}

func TestParseRejects(t *testing.T) {
	for name, in := range map[string]string{
		"schema":    `{"schema":2,"toolchains":[]}`,
		"twice":     `{"schema":1,"toolchains":[{"name":"a","repo":"o/r","version":"1"},{"name":"a","repo":"o/r","version":"2"}]}`,
		"bad sha":   `{"schema":1,"toolchains":[{"name":"a","repo":"o/r","version":"1","sha256":"ABC"}]}`,
		"traversal": `{"schema":1,"toolchains":[{"name":"../a","repo":"o/r","version":"1"}]}`,
		"version":   `{"schema":1,"toolchains":[{"name":"a","repo":"o/r","version":"1/../../x"}]}`,
		"repo":      `{"schema":1,"toolchains":[{"name":"a","repo":"https://x","version":"1"}]}`,
		"json":      `{`,
	} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPinKeepsSortedAndRoundTrips(t *testing.T) {
	m := &Manifest{Schema: Schema}
	for _, n := range []string{"zeta", "alpha"} {
		if err := m.Pin(Toolchain{Name: n, Repo: "r4d1um/setup", Version: "1", SHA256: sha}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Pin(Toolchain{Name: "zeta", Repo: "r4d1um/setup", Version: "2", SHA256: sha}); err != nil {
		t.Fatal(err)
	}
	if err := m.Pin(Toolchain{Name: "nosha", Repo: "r4d1um/setup", Version: "1"}); err == nil {
		t.Fatal("pinned without sha")
	}
	out := m.Marshal()
	if !strings.HasSuffix(string(out), "}\n") || strings.Index(string(out), "alpha") > strings.Index(string(out), "zeta") {
		t.Fatalf("not sorted / no newline:\n%s", out)
	}
	back, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if z, _ := back.Find("zeta"); z.Version != "2" || len(back.Toolchains) != 2 {
		t.Fatalf("round trip: %+v", back.Toolchains)
	}
}
