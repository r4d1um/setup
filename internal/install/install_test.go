// SPDX-License-Identifier: MIT

package install

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r4d1um/setup/internal/fetch"
	"github.com/r4d1um/setup/internal/manifest"
	"github.com/r4d1um/setup/internal/tgz"
)

// fakeSource serves prebuilt archives from memory and counts downloads.
type fakeSource struct {
	assets map[string][]byte
	calls  int
}

func (f *fakeSource) Download(_ context.Context, t manifest.Toolchain, dst string) error {
	f.calls++
	return os.WriteFile(dst, f.assets[t.Tag()], 0o644)
}

// archive builds a toolchain tree with the given files (path -> content; "x:" prefix = executable).
func archive(t *testing.T, files map[string]string) ([]byte, string) {
	t.Helper()
	root := t.TempDir()
	for p, c := range files {
		mode := os.FileMode(0o644)
		if strings.HasPrefix(c, "x:") {
			mode, c = 0o755, c[2:]
		}
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), mode); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := tgz.Pack(root, &buf); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "a")
	os.WriteFile(f, buf.Bytes(), 0o644)
	sum, _ := fetch.SHA256(f)
	return buf.Bytes(), sum
}

func setup(t *testing.T) (*Installer, *fakeSource, *manifest.Manifest) {
	t.Helper()
	v1, s1 := archive(t, map[string]string{
		"bin/avr-gcc": "x:#!/bin/sh\necho v1\n",
		"bin/notes":   "not executable",
		LayoutFile:    `{"path":["bin"],"links":{"arduino/packages/arduino":"packages/arduino"}}`,
		"packages/arduino/hardware/avr/installed.json": "{}",
	})
	v2, s2 := archive(t, map[string]string{"bin/avr-gcc": "x:#!/bin/sh\necho v2\n"})
	src := &fakeSource{assets: map[string][]byte{"avr-gcc-1": v1, "avr-gcc-2": v2}}
	m := &manifest.Manifest{Schema: 1, Toolchains: []manifest.Toolchain{
		{Name: "avr-gcc", Repo: "r4d1um/setup", Version: "1", SHA256: s1},
		{Name: "unreleased", Repo: "r4d1um/setup", Version: "1"},
	}}
	_ = s2
	home := t.TempDir()
	in := &Installer{Home: filepath.Join(home, "share"), BinDir: filepath.Join(home, "bin"),
		GitHubPath: filepath.Join(home, "github_path"), Source: src}
	m.Toolchains = append(m.Toolchains, manifest.Toolchain{Name: "v2", Repo: "r4d1um/setup", Version: "2", SHA256: s2})
	return in, src, m
}

func TestInstallLinksAndIsIdempotent(t *testing.T) {
	in, src, m := setup(t)
	ctx := context.Background()
	if err := in.Install(ctx, m, []string{"avr-gcc"}); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(in.BinDir, "avr-gcc"))
	if err != nil || target != filepath.Join(in.Home, "avr-gcc", "1", "bin", "avr-gcc") {
		t.Fatalf("command link: %q %v", target, err)
	}
	if _, err := os.Lstat(filepath.Join(in.BinDir, "notes")); err == nil {
		t.Fatal("linked a file that isn't executable")
	}
	if _, err := os.Stat(filepath.Join(in.Home, "arduino/packages/arduino/hardware/avr/installed.json")); err != nil {
		t.Fatalf("shared-tree link: %v", err)
	}
	gp, _ := os.ReadFile(in.GitHubPath)
	if strings.TrimSpace(string(gp)) != filepath.Join(in.Home, "avr-gcc", "1", "bin") {
		t.Fatalf("GITHUB_PATH: %q", gp)
	}
	if err := in.Install(ctx, m, []string{"avr-gcc"}); err != nil {
		t.Fatal(err)
	}
	if src.calls != 1 {
		t.Fatalf("downloaded %d times, want 1", src.calls)
	}
	if gp2, _ := os.ReadFile(in.GitHubPath); !bytes.Equal(gp, gp2) {
		t.Fatal("GITHUB_PATH got a duplicate line")
	}
}

func TestChecksumMismatchInstallsNothing(t *testing.T) {
	in, src, m := setup(t)
	src.assets["avr-gcc-1"] = append([]byte{}, src.assets["avr-gcc-1"]...)
	src.assets["avr-gcc-1"][len(src.assets["avr-gcc-1"])-1] ^= 0xff
	err := in.Install(context.Background(), m, []string{"avr-gcc"})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(in.Dir(m.Toolchains[0])); err == nil {
		t.Fatal("a toolchain with a bad checksum was put in place")
	}
	left, _ := filepath.Glob(filepath.Join(in.Home, ".tmp-*"))
	if len(left) != 0 {
		t.Fatalf("temp dirs left: %v", left)
	}
}

func TestNewVersionReplacesOldAndItsLinks(t *testing.T) {
	in, _, m := setup(t)
	ctx := context.Background()
	if err := in.Install(ctx, m, []string{"avr-gcc"}); err != nil {
		t.Fatal(err)
	}
	// the manifest moves avr-gcc to version 2 (archive without bin/notes or links)
	v2, _ := m.Find("v2")
	m.Toolchains[0].Version, m.Toolchains[0].SHA256 = "2", v2.SHA256
	in.Source.(*fakeSource).assets["avr-gcc-2"] = in.Source.(*fakeSource).assets["avr-gcc-2"]
	if err := in.Install(ctx, m, []string{"avr-gcc"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(in.Home, "avr-gcc", "1")); err == nil {
		t.Fatal("old version kept")
	}
	if target, _ := os.Readlink(filepath.Join(in.BinDir, "avr-gcc")); !strings.Contains(target, "/avr-gcc/2/") {
		t.Fatalf("link not moved: %s", target)
	}
}

func TestRefusesUnknownUnreleasedAndForeignCommands(t *testing.T) {
	in, _, m := setup(t)
	ctx := context.Background()
	if err := in.Install(ctx, m, []string{"nope"}); err == nil {
		t.Fatal("unknown toolchain installed")
	}
	if err := in.Install(ctx, m, []string{"unreleased"}); err == nil {
		t.Fatal("unpinned toolchain installed")
	}
	os.MkdirAll(in.BinDir, 0o755)
	os.WriteFile(filepath.Join(in.BinDir, "avr-gcc"), []byte("mine"), 0o755)
	if err := in.Install(ctx, m, []string{"avr-gcc"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(in.BinDir, "avr-gcc")); string(b) != "mine" {
		t.Fatal("overwrote a command that wasn't ours")
	}
}

func TestPythonPathFiles(t *testing.T) {
	arc, sum := archive(t, map[string]string{
		"python/pyte/__init__.py": "",
		LayoutFile:                `{"path":["bin"],"python":{"dir":"python","versions":["3.11","3.12"]}}`,
	})
	bad, badSum := archive(t, map[string]string{LayoutFile: `{"python":{"dir":"../x","versions":["3.11"]}}`})
	src := &fakeSource{assets: map[string][]byte{"py-console-1": arc, "evil-1": bad}}
	m := &manifest.Manifest{Schema: 1, Toolchains: []manifest.Toolchain{
		{Name: "py-console", Repo: "r4d1um/setup", Version: "1", SHA256: sum},
		{Name: "evil", Repo: "r4d1um/setup", Version: "1", SHA256: badSum},
	}}
	home := t.TempDir()
	in := &Installer{Home: filepath.Join(home, "share"), PythonBase: filepath.Join(home, "local"), Source: src}
	for i := 0; i < 2; i++ {
		if err := in.Install(context.Background(), m, []string{"py-console"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []string{"3.11", "3.12"} {
		b, err := os.ReadFile(filepath.Join(home, "local/lib/python"+v+"/site-packages/r4setup-py-console.pth"))
		if err != nil || strings.TrimSpace(string(b)) != filepath.Join(in.Home, "py-console", "1", "python") {
			t.Fatalf("%s: %q %v", v, b, err)
		}
	}
	if err := in.Install(context.Background(), m, []string{"evil"}); err == nil {
		t.Fatal("python dir outside the toolchain accepted")
	}
}
