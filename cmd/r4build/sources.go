// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/r4d1um/setup/internal/elfdeps"
	"github.com/r4d1um/setup/internal/sources"
)

const commonLicenses = "/usr/share/common-licenses"

// appendSources adds a section to root/SOURCES.txt (created if missing).
func appendSources(root, section string) error {
	p := filepath.Join(root, "SOURCES.txt")
	old, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(old) > 0 {
		old = append(old, '\n')
	}
	return os.WriteFile(p, append(old, section...), 0o644)
}

// bundleSources records where each library bundle copied into lib/host comes from: the
// Ubuntu package that owns it (SOURCES.txt) and that package's copyright file
// (share/doc/<package>/copyright). A library no package owns stops the build.
func bundleSources(root string, host elfdeps.HostLookup, sonames []string) error {
	if len(sonames) == 0 {
		return nil
	}
	pkgs := map[string]string{} // package -> SOURCES.txt line
	for _, so := range sonames {
		p := host(so)
		pkg, err := owner(p)
		if err != nil {
			return fmt.Errorf("%s (%s): %w", so, p, err)
		}
		if _, done := pkgs[pkg]; done {
			continue
		}
		out, err := exec.Command("dpkg-query", "-W", "-f", "${Version}\t${source:Package}\t${source:Version}", pkg).Output()
		if err != nil {
			return fmt.Errorf("dpkg-query -W %s: %w", pkg, err)
		}
		f := strings.Split(string(out), "\t")
		if len(f) != 3 {
			return fmt.Errorf("dpkg-query -W %s: %q", pkg, out)
		}
		pkgs[pkg] = sources.DebLine(pkg, f[0], f[1], f[2])
		name, _, _ := strings.Cut(pkg, ":")
		if err := copyFile(filepath.Join("/usr/share/doc", name, "copyright"), filepath.Join(root, "share/doc", name, "copyright")); err != nil {
			return fmt.Errorf("%s has no copyright file: %w", pkg, err)
		}
	}
	var lines []string
	for _, l := range pkgs {
		lines = append(lines, l)
	}
	sort.Strings(lines)
	return appendSources(root, "Host libraries in lib/host, copied from these Ubuntu 24.04 packages;\n"+
		"licence in share/doc/<package>/copyright, source at the links.\n\n"+strings.Join(lines, "\n")+"\n")
}

// owner finds the package that installed p, trying the path ldconfig gave, its real path and
// both sides of the /usr merge (dpkg knows a file under one of them only).
func owner(p string) (string, error) {
	try := []string{p}
	if real, err := filepath.EvalSymlinks(p); err == nil && real != p {
		try = append(try, real)
	}
	for _, c := range append([]string(nil), try...) {
		if strings.HasPrefix(c, "/usr/") {
			try = append(try, strings.TrimPrefix(c, "/usr"))
		} else {
			try = append(try, "/usr"+c)
		}
	}
	var last error
	for _, c := range try {
		out, err := exec.Command("dpkg-query", "-S", c).Output()
		if err != nil {
			last = fmt.Errorf("dpkg-query -S %s: %w", c, err)
			continue
		}
		pkg, err := sources.ParseDpkgSearch(string(out))
		if err != nil {
			last = err
			continue
		}
		return pkg, nil
	}
	return "", last
}

// arduinoSources describes every <vendor>/tools|hardware/<name>/<version> under packages
// (plus arduino-cli itself when cli is set): SOURCES.txt, the licence texts in share/licenses
// and each project's own licence files in share/doc/<name>.
func arduinoSources(root, packages, cli string) error {
	if root == "" || packages == "" {
		return fmt.Errorf("usage: r4build arduino-sources -root DIR -packages DIR [-cli VERSION]")
	}
	dirs, err := filepath.Glob(filepath.Join(packages, "*", "*", "*", "*"))
	if err != nil {
		return err
	}
	var rel []string
	for _, d := range dirs {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			continue
		}
		r, _ := filepath.Rel(packages, d)
		rel = append(rel, filepath.ToSlash(r))
	}
	items, err := sources.ParseArduino(rel)
	if err != nil {
		return err
	}
	if cli != "" {
		items = append(items, sources.Item{Vendor: "arduino", Kind: "cli", Name: "arduino-cli", Version: cli})
	}
	if len(items) == 0 {
		return fmt.Errorf("%s: no Arduino tools found", packages)
	}
	entries, err := sources.Arduino(items)
	if err != nil {
		return err
	}
	for _, id := range sources.Licenses(entries) {
		if err := copyFile(filepath.Join(commonLicenses, sources.CommonLicense[id]), filepath.Join(root, "share/licenses", id+".txt")); err != nil {
			return fmt.Errorf("licence text %s: %w", id, err)
		}
	}
	for _, e := range entries {
		if e.Kind == "cli" {
			continue // its LICENSE.txt comes from the release tarball (build.sh)
		}
		dir := filepath.Join(packages, e.Vendor, e.Kind, e.Name, e.Version)
		files, _ := filepath.Glob(filepath.Join(dir, "*"))
		for _, f := range files {
			b := strings.ToUpper(filepath.Base(f))
			if !strings.HasPrefix(b, "LICENSE") && !strings.HasPrefix(b, "COPYING") {
				continue
			}
			if st, err := os.Stat(f); err != nil || !st.Mode().IsRegular() {
				continue
			}
			if err := copyFile(f, filepath.Join(root, "share/doc", e.Name, filepath.Base(f))); err != nil {
				return err
			}
		}
	}
	return appendSources(root, sources.FormatArduino(entries))
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}
