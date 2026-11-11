package tgz

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripKeepsLinksAndModes(t *testing.T) {
	src := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(src, "bin"), 0o755))
	must(t, os.WriteFile(filepath.Join(src, "bin", "gcc"), []byte("#!/bin/sh\n"), 0o755))
	must(t, os.Link(filepath.Join(src, "bin", "gcc"), filepath.Join(src, "bin", "cc")))
	must(t, os.Symlink("gcc", filepath.Join(src, "bin", "c99")))
	must(t, os.WriteFile(filepath.Join(src, "README"), []byte("hi"), 0o644))

	var a, b bytes.Buffer
	must(t, Pack(src, &a))
	must(t, Pack(src, &b))
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("two packs of the same tree differ")
	}
	dst := t.TempDir()
	must(t, Extract(bytes.NewReader(a.Bytes()), dst))
	fi, err := os.Stat(filepath.Join(dst, "bin", "cc"))
	must(t, err)
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v", fi.Mode())
	}
	gi, _ := os.Stat(filepath.Join(dst, "bin", "gcc"))
	if !os.SameFile(fi, gi) {
		t.Fatal("hard link became a copy")
	}
	if l, _ := os.Readlink(filepath.Join(dst, "bin", "c99")); l != "gcc" {
		t.Fatalf("symlink %q", l)
	}
}

func TestExtractRefusesEscapes(t *testing.T) {
	for name, h := range map[string]*tar.Header{
		"dotdot":   {Name: "../evil", Typeflag: tar.TypeReg, Mode: 0o644},
		"absolute": {Name: "/etc/evil", Typeflag: tar.TypeReg, Mode: 0o644},
		"symlink":  {Name: "lib/x", Typeflag: tar.TypeSymlink, Linkname: "../../etc"},
		"abs link": {Name: "x", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		"device":   {Name: "dev", Typeflag: tar.TypeChar},
	} {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		must(t, tw.WriteHeader(h))
		tw.Close()
		gz.Close()
		if err := Extract(&buf, t.TempDir()); err == nil {
			t.Errorf("%s: extracted", name)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
