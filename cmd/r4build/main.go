// r4build holds the logic the toolchains/*/build.sh scripts share (they stay short glue).
//
//	r4build plan   -repo r4d1um/setup -dir toolchains     # JSON list of toolchains whose release is missing
//	r4build debs   -root DIR pkg=version...              # Ubuntu packages, unpacked relocatable into DIR
//	r4build bundle -root DIR                             # copy host-only libraries in, set $ORIGIN RUNPATHs
//	r4build pack   -name N -version V -root DIR -out OUT # OUT/<name>-<version>-linux-amd64.tar.gz (+ .sha256)
//	r4build unpack -asset FILE -dir DIR                  # for the smoke tests
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/r4d1um/setup/internal/elfdeps"
	"github.com/r4d1um/setup/internal/fetch"
	"github.com/r4d1um/setup/internal/install"
	"github.com/r4d1um/setup/internal/manifest"
	"github.com/r4d1um/setup/internal/tgz"
)

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: r4build plan|debs|bundle|pack|unpack (see cmd/r4build/main.go)"))
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	var err error
	switch os.Args[1] {
	case "plan":
		repo := fs.String("repo", "", "owner/repo whose releases to check")
		dir := fs.String("dir", "toolchains", "directory with <name>/VERSION")
		fs.Parse(os.Args[2:])
		err = plan(*repo, *dir)
	case "debs":
		root := fs.String("root", "", "target directory")
		fs.Parse(os.Args[2:])
		err = debs(*root, fs.Args())
	case "bundle":
		root := fs.String("root", "", "toolchain tree")
		fs.Parse(os.Args[2:])
		var host elfdeps.HostLookup
		if host, err = elfdeps.Ldconfig(); err == nil {
			_, err = elfdeps.Bundle(*root, host, logf)
		}
	case "pack":
		name := fs.String("name", "", "toolchain name")
		version := fs.String("version", "", "toolchain version")
		root := fs.String("root", "", "toolchain tree (with r4setup.json)")
		out := fs.String("out", "", "output directory")
		fs.Parse(os.Args[2:])
		err = pack(*name, *version, *root, *out)
	case "unpack":
		asset := fs.String("asset", "", "toolchain archive")
		dir := fs.String("dir", "", "target directory")
		fs.Parse(os.Args[2:])
		err = unpack(*asset, *dir)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func logf(f string, a ...any) { fmt.Fprintf(os.Stderr, "r4build: "+f+"\n", a...) }

func fail(err error) {
	fmt.Fprintln(os.Stderr, "r4build:", err)
	os.Exit(1)
}

type job struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Timeout int    `json:"timeout"`
}

// plan lists the toolchains under dir whose "<name>-<version>" release doesn't exist yet.
func plan(repo, dir string) error {
	out, err := exec.Command("gh", "release", "list", "-R", repo, "--limit", "1000", "--json", "tagName", "-q", ".[].tagName").Output()
	if err != nil {
		return fmt.Errorf("gh release list: %w", err)
	}
	have := map[string]bool{}
	for _, t := range strings.Fields(string(out)) {
		have[t] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	jobs := []job{}
	for _, e := range entries {
		vb, err := os.ReadFile(filepath.Join(dir, e.Name(), "VERSION"))
		if !e.IsDir() || err != nil {
			continue
		}
		j := job{Name: e.Name(), Version: strings.TrimSpace(string(vb)), Timeout: 30}
		if err := (manifest.Toolchain{Name: j.Name, Version: j.Version, Repo: repo}).Check(); err != nil {
			return err
		}
		if tb, err := os.ReadFile(filepath.Join(dir, e.Name(), "TIMEOUT")); err == nil {
			if j.Timeout, err = strconv.Atoi(strings.TrimSpace(string(tb))); err != nil {
				return fmt.Errorf("%s/TIMEOUT: %w", e.Name(), err)
			}
		}
		if !have[j.Name+"-"+j.Version] {
			jobs = append(jobs, j)
		}
	}
	sort.Slice(jobs, func(a, b int) bool { return jobs[a].Name < jobs[b].Name })
	b, _ := json.Marshal(jobs)
	fmt.Println(string(b))
	return nil
}

// debs downloads Ubuntu packages (exact versions) and unpacks them into root with usr/ folded
// away: root/bin, root/lib, … GCC finds its parts relative to its own location, so the tree
// works wherever it is unpacked. Copyright files stay (share/doc/*/copyright), other docs go,
// and SOURCES.txt says where each package's source is.
func debs(root string, pkgs []string) error {
	if root == "" || len(pkgs) == 0 {
		return fmt.Errorf("usage: r4build debs -root DIR pkg=version...")
	}
	tmp, err := os.MkdirTemp("", "r4build-debs-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	cmd := exec.Command("apt-get", append([]string{"download", "-q"}, pkgs...)...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = tmp, os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get download: %w", err)
	}
	files, _ := filepath.Glob(filepath.Join(tmp, "*.deb"))
	if len(files) != len(pkgs) {
		return fmt.Errorf("asked for %d packages, got %d .deb files", len(pkgs), len(files))
	}
	stage := filepath.Join(tmp, "stage")
	var sources bytes.Buffer
	sources.WriteString("Unpacked from these Ubuntu 24.04 (noble) packages; their source code is at the links.\n\n")
	sort.Strings(files)
	for _, f := range files {
		if out, err := exec.Command("dpkg-deb", "-x", f, stage).CombinedOutput(); err != nil {
			return fmt.Errorf("dpkg-deb -x %s: %v %s", f, err, out)
		}
		info, err := exec.Command("dpkg-deb", "-f", f, "Package", "Version", "Source").Output()
		if err != nil {
			return err
		}
		fields := map[string]string{}
		for _, l := range strings.Split(string(info), "\n") {
			if k, v, ok := strings.Cut(l, ": "); ok {
				fields[k] = v
			}
		}
		src, srcVer := fields["Source"], fields["Version"]
		if src == "" {
			src = fields["Package"]
		}
		if s, v, ok := strings.Cut(src, " ("); ok { // "gcc-avr (1:7.3.0...)"
			src, srcVer = s, strings.TrimSuffix(v, ")")
		}
		fmt.Fprintf(&sources, "%s %s  https://launchpad.net/ubuntu/+source/%s/%s\n", fields["Package"], fields["Version"], src, srcVer)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for _, sub := range []string{"usr", "etc"} {
		from := filepath.Join(stage, sub)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		to := root
		if sub == "etc" {
			to = filepath.Join(root, "etc")
			os.MkdirAll(to, 0o755)
		}
		if out, err := exec.Command("cp", "-a", from+"/.", to+"/").CombinedOutput(); err != nil {
			return fmt.Errorf("cp: %v %s", err, out)
		}
	}
	for _, junk := range []string{"share/man", "share/info", "share/lintian", "share/bug"} {
		os.RemoveAll(filepath.Join(root, junk))
	}
	docs, _ := filepath.Glob(filepath.Join(root, "share/doc/*/*"))
	for _, d := range docs {
		if filepath.Base(d) != "copyright" {
			os.RemoveAll(d)
		}
	}
	return os.WriteFile(filepath.Join(root, "SOURCES.txt"), sources.Bytes(), 0o644)
}

func pack(name, version, root, out string) error {
	if err := (manifest.Toolchain{Name: name, Version: version, Repo: "o/r"}).Check(); err != nil {
		return err
	}
	if b, err := os.ReadFile(filepath.Join(root, install.LayoutFile)); err != nil {
		return fmt.Errorf("%s needs a %s: %w", root, install.LayoutFile, err)
	} else if err := json.Unmarshal(b, &install.Layout{}); err != nil {
		return fmt.Errorf("%s: %w", install.LayoutFile, err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(out, manifest.AssetName(name, version))
	f, err := os.Create(dst + ".tmp")
	if err != nil {
		return err
	}
	err = tgz.Pack(root, f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(dst+".tmp", dst); err != nil {
		return err
	}
	sum, err := fetch.SHA256(dst)
	if err != nil {
		return err
	}
	fi, _ := os.Stat(dst)
	logf("%s: %d MB, sha256 %s", filepath.Base(dst), fi.Size()>>20, sum)
	return os.WriteFile(dst+".sha256", []byte(sum+"  "+filepath.Base(dst)+"\n"), 0o644)
}

func unpack(asset, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Open(asset)
	if err != nil {
		return err
	}
	defer f.Close()
	return tgz.Extract(f, dir)
}
