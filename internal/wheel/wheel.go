// SPDX-License-Identifier: MIT

// Package wheel unpacks Python wheels into a toolchain tree without pip:
//
//	<name>.data/scripts/X           -> bin/X (executable)
//	<name>.data/purelib|platlib/... -> python/...
//	<name>.data/(headers|data)/...  -> left out
//	everything else                 -> python/...
//
// Several wheels can go into one tree, also the same package built for several Python
// versions: their extension modules have different names (_imaging.cpython-311-…so), the
// rest is identical. A file that is already there with other content is an error, except
// inside *.dist-info/ (WHEEL, RECORD differ per build), where the first wheel wins.
package wheel

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Info names the distribution a wheel holds.
type Info struct {
	Name    string // as in the file name: "pillow", "shellcheck_py"
	Version string
}

// ParseName reads name and version from a wheel's file name
// (name-version(-build)?-python-abi-platform.whl).
func ParseName(file string) (Info, error) {
	base := strings.TrimSuffix(filepath.Base(file), ".whl")
	parts := strings.Split(base, "-")
	if len(parts) < 5 || len(parts) > 6 || base == filepath.Base(file) {
		return Info{}, fmt.Errorf("%s: not a wheel file name", file)
	}
	return Info{Name: parts[0], Version: parts[1]}, nil
}

// Target says where a member of a wheel goes in the tree ("" = left out).
func Target(member string) (string, error) {
	if strings.HasSuffix(member, "/") {
		return "", nil // directories come with their files
	}
	clean := path.Clean(member)
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%q leaves the tree", member)
	}
	first, rest, _ := strings.Cut(clean, "/")
	if strings.HasSuffix(first, ".data") {
		kind, sub, ok := strings.Cut(rest, "/")
		if !ok {
			return "", nil
		}
		switch kind {
		case "scripts":
			return path.Join("bin", sub), nil
		case "purelib", "platlib":
			return path.Join("python", sub), nil
		default:
			return "", nil
		}
	}
	return path.Join("python", clean), nil
}

// Merge unpacks the wheel at file into root.
func Merge(root, file string) error {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		rel, err := Target(f.Name)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
		if rel == "" {
			continue
		}
		if err := put(root, rel, f); err != nil {
			return fmt.Errorf("%s: %s: %w", filepath.Base(file), f.Name, err)
		}
	}
	return nil
}

func put(root, rel string, f *zip.File) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return err
	}
	dst := filepath.Join(root, filepath.FromSlash(rel))
	if old, err := os.ReadFile(dst); err == nil {
		if bytes.Equal(old, data) || inDistInfo(rel) {
			return nil
		}
		return fmt.Errorf("%s is already there with other content", rel)
	}
	mode := os.FileMode(0o644)
	if strings.HasPrefix(rel, "bin/") || f.Mode()&0o111 != 0 {
		mode = 0o755
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".wheel-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func inDistInfo(rel string) bool {
	for _, p := range strings.Split(rel, "/") {
		if strings.HasSuffix(p, ".dist-info") {
			return true
		}
	}
	return false
}
