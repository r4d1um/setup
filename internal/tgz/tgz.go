// SPDX-License-Identifier: MIT

// Package tgz packs a directory into a reproducible .tar.gz and unpacks one
// safely (no paths or links that leave the target directory).
package tgz

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Pack writes root's contents as tar.gz: sorted, owner 0, fixed mtime, hard links kept
// (GCC installs the same driver under several names).
func Pack(root string, w io.Writer) error {
	gz, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
	gz.ModTime = time.Unix(0, 0)
	tw := tar.NewWriter(gz)
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != root {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	type ino struct{ dev, ino uint64 }
	seen := map[ino]string{}
	mtime := time.Unix(0, 0)
	for _, p := range paths {
		rel := filepath.ToSlash(strings.TrimPrefix(p, root+string(filepath.Separator)))
		fi, err := os.Lstat(p)
		if err != nil {
			return err
		}
		h := &tar.Header{Name: rel, Mode: int64(fi.Mode().Perm()), ModTime: mtime, Format: tar.FormatPAX}
		switch {
		case fi.IsDir():
			h.Typeflag, h.Name = tar.TypeDir, rel+"/"
		case fi.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			h.Typeflag, h.Linkname = tar.TypeSymlink, target
		case fi.Mode().IsRegular():
			if dev, in, ok := inode(fi); ok {
				if first, dup := seen[ino{dev, in}]; dup {
					h.Typeflag, h.Linkname = tar.TypeLink, first
					break
				}
				seen[ino{dev, in}] = rel
			}
			h.Typeflag, h.Size = tar.TypeReg, fi.Size()
		default:
			return fmt.Errorf("%s: unsupported file type %v", rel, fi.Mode().Type())
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if h.Typeflag == tar.TypeReg {
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, f)
			f.Close()
			if err != nil {
				return err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// inside reports whether name (slash separated, relative) stays below the root.
func inside(name string) bool {
	c := path.Clean(name)
	return c != ".." && !strings.HasPrefix(c, "../") && !path.IsAbs(c)
}

// Extract unpacks a tar.gz into dir (which must exist). Absolute paths, "..", links
// pointing outside dir and device files are refused.
func Extract(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if !inside(h.Name) {
			return fmt.Errorf("refusing %q: outside the target", h.Name)
		}
		dst := filepath.Join(dir, filepath.FromSlash(h.Name))
		mode := fs.FileMode(h.Mode).Perm()
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, mode|0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			if path.IsAbs(h.Linkname) || !inside(path.Join(path.Dir(h.Name), h.Linkname)) {
				return fmt.Errorf("refusing link %q -> %q: points outside the target", h.Name, h.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, dst); err != nil {
				return err
			}
		case tar.TypeLink:
			if !inside(h.Linkname) {
				return fmt.Errorf("refusing hard link %q -> %q", h.Name, h.Linkname)
			}
			if err := os.Link(filepath.Join(dir, filepath.FromSlash(h.Linkname)), dst); err != nil {
				return err
			}
		default:
			return fmt.Errorf("refusing %q: unsupported entry type %q", h.Name, h.Typeflag)
		}
	}
}
