// Package elfdeps makes a toolchain tree self-contained: every x86-64 ELF file
// gets a RUNPATH ($ORIGIN-relative) to the libraries it needs, and libraries
// that only exist on the build host are copied into <root>/lib/host.
// Libraries every Ubuntu 24.04 install has (glibc, libstdc++, zlib, …) stay outside.
package elfdeps

import (
	"bufio"
	"bytes"
	"debug/elf"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Base are sonames found on every Ubuntu 24.04 system (essential/required packages).
var Base = map[string]bool{
	"libc.so.6": true, "libm.so.6": true, "libpthread.so.0": true, "libdl.so.2": true,
	"librt.so.1": true, "libresolv.so.2": true, "libutil.so.1": true, "ld-linux-x86-64.so.2": true,
	"libgcc_s.so.1": true, "libstdc++.so.6": true, "libz.so.1": true, "libzstd.so.1": true,
	"liblzma.so.5": true, "libbz2.so.1.0": true, "libtinfo.so.6": true, "libcrypt.so.1": true,
	"libmvec.so.1": true, "libanl.so.1": true,
}

// HostLookup finds a soname on the build host ("" if unknown).
type HostLookup func(soname string) string

// Ldconfig returns a lookup backed by `ldconfig -p` (x86-64 entries only).
func Ldconfig() (HostLookup, error) {
	out, err := exec.Command("ldconfig", "-p").Output()
	if err != nil {
		return nil, fmt.Errorf("ldconfig -p: %w", err)
	}
	libs := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		name, rest, ok := strings.Cut(line, " (")
		if !ok || !strings.Contains(rest, "x86-64") {
			continue
		}
		if _, p, ok := strings.Cut(rest, "=> "); ok {
			if _, dup := libs[name]; !dup {
				libs[name] = p
			}
		}
	}
	return func(s string) string { return libs[s] }, nil
}

type elfInfo struct {
	needed []string
	rpath  []string
}

func readELF(p string) (*elfInfo, bool) {
	f, err := elf.Open(p)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	if f.Machine != elf.EM_X86_64 || (f.Type != elf.ET_EXEC && f.Type != elf.ET_DYN) || f.Section(".dynamic") == nil {
		return nil, false
	}
	needed, _ := f.ImportedLibraries()
	info := &elfInfo{needed: needed}
	for _, tag := range []elf.DynTag{elf.DT_RUNPATH, elf.DT_RPATH} {
		vals, _ := f.DynString(tag)
		for _, v := range vals {
			info.rpath = append(info.rpath, strings.Split(v, ":")...)
		}
	}
	return info, true
}

// Bundle fixes up root. patchelf must be installed. It returns the libraries it copied.
func Bundle(root string, host HostLookup, logf func(string, ...any)) ([]string, error) {
	hostDir := filepath.Join(root, "lib", "host")
	var copied []string
	for round := 0; ; round++ {
		if round > 10 {
			return copied, fmt.Errorf("library dependencies don't settle after 10 rounds")
		}
		// which sonames does the tree itself provide, and where
		inTree := map[string]string{}
		var files []string
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Type()&fs.ModeSymlink != 0 && !strings.Contains(d.Name(), ".so") {
				return err
			}
			if strings.Contains(d.Name(), ".so") {
				if _, dup := inTree[d.Name()]; !dup {
					inTree[d.Name()] = filepath.Dir(p)
				}
			}
			if d.Type().IsRegular() {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return copied, err
		}
		sort.Strings(files)
		changed := false
		for _, p := range files {
			info, ok := readELF(p)
			if !ok {
				continue
			}
			var dirs []string
			for _, soname := range info.needed {
				if Base[soname] {
					continue
				}
				dir, ok := inTree[soname]
				if !ok {
					src := host(soname)
					if src == "" {
						return copied, fmt.Errorf("%s needs %s, which neither the tree nor the host has", rel(root, p), soname)
					}
					if err := copyFile(src, filepath.Join(hostDir, soname)); err != nil {
						return copied, err
					}
					logf("bundled %s (needed by %s)", soname, rel(root, p))
					copied = append(copied, soname)
					dir, inTree[soname] = hostDir, hostDir
					changed = true
				}
				dirs = append(dirs, origin(filepath.Dir(p), dir))
			}
			want := mergeRpath(info.rpath, dirs)
			if want != strings.Join(info.rpath, ":") {
				if out, err := exec.Command("patchelf", "--set-rpath", want, p).CombinedOutput(); err != nil {
					return copied, fmt.Errorf("patchelf %s: %v: %s", rel(root, p), err, out)
				}
				changed = true
			}
		}
		if !changed {
			return copied, nil
		}
	}
}

// origin is dir relative to from, as a RUNPATH entry ("$ORIGIN/../lib/host").
// No filepath.Join here: it would fold "$ORIGIN/.." away.
func origin(from, dir string) string {
	r, err := filepath.Rel(from, dir)
	if err != nil || r == "." {
		return "$ORIGIN"
	}
	return "$ORIGIN/" + filepath.ToSlash(r)
}

// mergeRpath keeps the existing entries and appends missing ones, without duplicates.
func mergeRpath(have, add []string) string {
	seen := map[string]bool{}
	var out []string
	for _, e := range append(append([]string{}, have...), add...) {
		e = strings.TrimRight(e, "/")
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	return strings.Join(out, ":")
}

func rel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return r
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
