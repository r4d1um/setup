// Package install puts toolchains from the manifest into place: download only
// what's missing, check the SHA256, unpack under a temporary name and rename,
// then link the commands into a bin directory that's on PATH.
//
// Every toolchain archive carries an r4setup.json at its root:
//
//	{"path": ["bin"], "links": {"arduino/packages/arduino": "packages/arduino"}}
//
// path: directories whose commands get linked (and added to GITHUB_PATH in Actions).
// links: symlinks <home>/<key> -> <toolchain>/<value>, so several toolchains can
// fill one shared tree (arduino-cli's data directory).
// python: a directory of Python packages; for each listed version a
// <user site>/r4setup-<name>.pth points that Python at it, so `import pyte` just works
// (the user site is ~/.local/lib/pythonX.Y/site-packages, or under $PYTHONUSERBASE).
//
//	{"path": ["bin"], "python": {"dir": "python", "versions": ["3.11", "3.12"]}}
package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/r4d1um/setup/internal/fetch"
	"github.com/r4d1um/setup/internal/manifest"
	"github.com/r4d1um/setup/internal/tgz"
)

const (
	LayoutFile = "r4setup.json"
	markerFile = ".r4setup-sha256"
)

type Layout struct {
	Path   []string          `json:"path,omitempty"`
	Links  map[string]string `json:"links,omitempty"`
	Python *PythonLayout     `json:"python,omitempty"`
}

type PythonLayout struct {
	Dir      string   `json:"dir"`
	Versions []string `json:"versions"`
}

type Installer struct {
	Home       string // toolchains go to <Home>/<name>/<version>
	BinDir     string // commands are linked here; "" links nothing
	GitHubPath string // Actions' $GITHUB_PATH file; "" outside Actions
	PythonBase string // Python's user base (~/.local): .pth files go to its site-packages; "" writes none
	Source     fetch.Source
	Log        func(format string, args ...any)
}

func (in *Installer) logf(format string, args ...any) {
	if in.Log != nil {
		in.Log(format, args...)
	}
}

// Dir is where a toolchain version lives.
func (in *Installer) Dir(t manifest.Toolchain) string {
	return filepath.Join(in.Home, t.Name, t.Version)
}

// Installed reports whether exactly this release (by checksum) is in place.
func (in *Installer) Installed(t manifest.Toolchain) bool {
	b, err := os.ReadFile(filepath.Join(in.Dir(t), markerFile))
	return err == nil && strings.TrimSpace(string(b)) == t.SHA256
}

// Install installs the named toolchains; already installed ones only get their links checked.
func (in *Installer) Install(ctx context.Context, m *manifest.Manifest, names []string) error {
	if len(names) == 0 {
		return errors.New("no toolchain named")
	}
	var list []manifest.Toolchain
	for _, n := range names {
		t, ok := m.Find(n)
		if !ok {
			return fmt.Errorf("unknown toolchain %q (r4setup list shows them)", n)
		}
		if t.SHA256 == "" {
			return fmt.Errorf("%s %s has no release pinned in the manifest yet", t.Name, t.Version)
		}
		list = append(list, t)
	}
	if err := os.MkdirAll(in.Home, 0o755); err != nil {
		return err
	}
	unlock, err := lock(filepath.Join(in.Home, ".lock"))
	if err != nil {
		return err
	}
	defer unlock()
	for _, t := range list {
		if in.Installed(t) {
			in.logf("%s %s: already installed", t.Name, t.Version)
		} else if err := in.fetch(ctx, t); err != nil {
			return err
		}
		if err := in.activate(t); err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
	}
	return in.pruneDanglingLinks()
}

func (in *Installer) fetch(ctx context.Context, t manifest.Toolchain) error {
	in.logf("%s %s: downloading %s from %s", t.Name, t.Version, t.Asset(), t.Repo)
	tmp, err := os.MkdirTemp(in.Home, ".tmp-"+t.Name+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	asset := filepath.Join(tmp, t.Asset())
	if err := in.Source.Download(ctx, t, asset); err != nil {
		return err
	}
	if err := fetch.Verify(asset, t.SHA256); err != nil {
		return err
	}
	root := filepath.Join(tmp, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		return err
	}
	f, err := os.Open(asset)
	if err != nil {
		return err
	}
	err = tgz.Extract(f, root)
	f.Close()
	if err != nil {
		return fmt.Errorf("%s: unpacking: %w", t.Name, err)
	}
	if err := os.WriteFile(filepath.Join(root, markerFile), []byte(t.SHA256+"\n"), 0o644); err != nil {
		return err
	}
	dir := in.Dir(t)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil { // a half-broken earlier attempt
		return err
	}
	if err := os.Rename(root, dir); err != nil {
		return err
	}
	// other versions of the same toolchain are no longer wanted
	others, _ := os.ReadDir(filepath.Dir(dir))
	for _, o := range others {
		if o.Name() != t.Version {
			os.RemoveAll(filepath.Join(filepath.Dir(dir), o.Name()))
		}
	}
	in.logf("%s %s: installed in %s", t.Name, t.Version, dir)
	return nil
}

func (in *Installer) layout(dir string) (Layout, error) {
	var l Layout
	b, err := os.ReadFile(filepath.Join(dir, LayoutFile))
	if errors.Is(err, fs.ErrNotExist) {
		if fi, err := os.Stat(filepath.Join(dir, "bin")); err == nil && fi.IsDir() {
			l.Path = []string{"bin"}
		}
		return l, nil
	}
	if err != nil {
		return l, err
	}
	if err := json.Unmarshal(b, &l); err != nil {
		return l, fmt.Errorf("%s: %w", LayoutFile, err)
	}
	return l, nil
}

func relInside(p string) bool {
	c := filepath.Clean(p)
	return c != "." && c != ".." && !filepath.IsAbs(c) && !strings.HasPrefix(c, ".."+string(filepath.Separator))
}

func (in *Installer) activate(t manifest.Toolchain) error {
	dir := in.Dir(t)
	l, err := in.layout(dir)
	if err != nil {
		return err
	}
	for _, p := range l.Path {
		if !relInside(p) {
			return fmt.Errorf("path entry %q leaves the toolchain", p)
		}
		bindir := filepath.Join(dir, p)
		if in.BinDir != "" {
			if err := in.linkCommands(bindir); err != nil {
				return err
			}
		}
		if in.GitHubPath != "" {
			if err := appendLine(in.GitHubPath, bindir); err != nil {
				return err
			}
		}
	}
	for k, v := range l.Links {
		if !relInside(k) || !relInside(v) {
			return fmt.Errorf("link %q -> %q leaves its directory", k, v)
		}
		if err := replaceSymlink(filepath.Join(dir, v), filepath.Join(in.Home, k)); err != nil {
			return err
		}
	}
	if l.Python != nil && in.PythonBase != "" {
		if !relInside(l.Python.Dir) {
			return fmt.Errorf("python dir %q leaves the toolchain", l.Python.Dir)
		}
		for _, v := range l.Python.Versions {
			if !pyVersionRe.MatchString(v) {
				return fmt.Errorf("bad python version %q", v)
			}
			site := filepath.Join(in.PythonBase, "lib", "python"+v, "site-packages")
			if err := writeFile(filepath.Join(site, "r4setup-"+t.Name+".pth"), filepath.Join(dir, l.Python.Dir)+"\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

var pyVersionRe = regexp.MustCompile(`^3\.[0-9]+$`)

// writeFile writes content under a temporary name and renames it into place
// (unchanged files are left alone).
func writeFile(dst, content string) error {
	if b, err := os.ReadFile(dst); err == nil && string(b) == content {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".r4setup-tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func (in *Installer) linkCommands(bindir string) error {
	entries, err := os.ReadDir(bindir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(in.BinDir, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		src := filepath.Join(bindir, e.Name())
		fi, err := os.Stat(src)
		if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			continue
		}
		dst := filepath.Join(in.BinDir, e.Name())
		if cur, err := os.Readlink(dst); err == nil {
			if cur == src {
				continue
			}
			if !strings.HasPrefix(cur, in.Home+string(filepath.Separator)) {
				in.logf("warning: %s already exists and isn't from r4setup, left alone", dst)
				continue
			}
		} else if _, err := os.Lstat(dst); err == nil {
			in.logf("warning: %s already exists and isn't from r4setup, left alone", dst)
			continue
		}
		if err := replaceSymlink(src, dst); err != nil {
			return err
		}
	}
	return nil
}

// pruneDanglingLinks drops our links whose target went away with an old version.
func (in *Installer) pruneDanglingLinks() error {
	if in.BinDir == "" {
		return nil
	}
	entries, err := os.ReadDir(in.BinDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		p := filepath.Join(in.BinDir, e.Name())
		cur, err := os.Readlink(p)
		if err != nil || !strings.HasPrefix(cur, in.Home+string(filepath.Separator)) {
			continue
		}
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			os.Remove(p)
		}
	}
	return nil
}

// replaceSymlink points dst at target, atomically (temp name, then rename).
func replaceSymlink(target, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".r4setup-tmp"
	os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func appendLine(file, line string) error {
	b, _ := os.ReadFile(file)
	for _, l := range strings.Split(string(b), "\n") {
		if l == line {
			return nil
		}
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, line)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
