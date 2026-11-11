// SPDX-License-Identifier: MIT

// Package sources says where the source code of what a toolchain archive redistributes is,
// and under which licences: the lines of SOURCES.txt and the licence texts that go next to
// the binaries. GPL & co. allow redistribution only with both, so an unknown Arduino tool is
// an error instead of an archive without them. No I/O here: r4build reads dpkg and the tree.
package sources

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// CommonLicense maps the SPDX ids used here to the texts in /usr/share/common-licenses
// (base-files, on every Debian/Ubuntu build host).
var CommonLicense = map[string]string{
	"GPL-2.0":  "GPL-2",
	"GPL-3.0":  "GPL-3",
	"LGPL-2.1": "LGPL-2.1",
	"LGPL-3.0": "LGPL-3",
}

// Item is one Arduino package directory: packages/<Vendor>/<Kind>/<Name>/<Version>,
// Kind "tools" or "hardware". The CLI itself is Vendor "arduino", Kind "cli".
type Item struct {
	Vendor, Kind, Name, Version string
}

// Entry is an Item with its licences and where its source is.
type Entry struct {
	Item
	Licenses []string // SPDX ids, each also a key of CommonLicense
	Source   string   // URL of the exact version where one exists
	Note     string   // extra pointers (upstream, parts under other licences)
}

type rule struct {
	licenses []string
	source   func(v string) string
	note     string
}

func gh(repo, tagPrefix string) func(string) string {
	return func(v string) string { return "https://github.com/" + repo + "/tree/" + tagPrefix + v }
}

// arduino knows every tool arduino-cli and the AVR core bring along. Key: vendor/kind/name.
var arduino = map[string]rule{
	"arduino/cli/arduino-cli":        {[]string{"GPL-3.0"}, gh("arduino/arduino-cli", "v"), ""},
	"builtin/tools/ctags":            {[]string{"GPL-2.0"}, gh("arduino/ctags", ""), ""},
	"builtin/tools/serial-discovery": {[]string{"GPL-3.0"}, gh("arduino/serial-discovery", "v"), ""},
	"builtin/tools/mdns-discovery":   {[]string{"GPL-3.0"}, gh("arduino/mdns-discovery", "v"), ""},
	"builtin/tools/serial-monitor":   {[]string{"GPL-3.0"}, gh("arduino/serial-monitor", "v"), ""},
	"builtin/tools/dfu-discovery":    {[]string{"GPL-3.0"}, gh("arduino/dfu-discovery", "v"), ""},
	"arduino/hardware/avr":           {[]string{"LGPL-2.1"}, gh("arduino/ArduinoCore-avr", ""), ""},
	"arduino/tools/arduinoOTA":       {[]string{"GPL-3.0"}, gh("arduino/arduinoOTA", ""), ""},
	"arduino/tools/avr-gcc": {[]string{"GPL-3.0"}, gh("arduino/toolchain-avr", ""),
		"GCC and binutils GPL-3.0; avr-libc under its modified BSD licence, " +
			"https://github.com/avrdudes/avr-libc/blob/main/LICENSE"},
	"arduino/tools/avrdude": {[]string{"GPL-2.0"}, avrdude, ""},
}

// avrdude: Arduino builds 6.3 with avrdude-build-script, 7 and later with avrdude-packing;
// neither tags every build, so the link is the upstream release plus Arduino's build repo.
func avrdude(v string) string {
	up := strings.SplitN(v, "-", 2)[0] // "6.3.0-arduino17" -> "6.3.0"
	parts := strings.Split(up, ".")
	if len(parts) >= 2 {
		up = parts[0] + "." + parts[1]
	}
	build := "arduino/avrdude-packing"
	if parts[0] == "6" {
		build = "arduino/avrdude-build-script"
	}
	return "https://github.com/avrdudes/avrdude/tree/v" + up + " (Arduino's build: https://github.com/" + build + ")"
}

// ParseArduino turns paths relative to a packages directory ("builtin/tools/ctags/5.8-arduino11")
// into Items. Anything not four levels deep is an error: the caller lists exactly those dirs.
func ParseArduino(rel []string) ([]Item, error) {
	var items []Item
	for _, r := range rel {
		p := strings.Split(path.Clean(r), "/")
		if len(p) != 4 || (p[1] != "tools" && p[1] != "hardware") {
			return nil, fmt.Errorf("%s: want <vendor>/tools|hardware/<name>/<version>", r)
		}
		items = append(items, Item{Vendor: p[0], Kind: p[1], Name: p[2], Version: p[3]})
	}
	return items, nil
}

// Arduino describes the items; an item without a rule is an error, so a new tool can't
// slip into a public release without its source pointer.
func Arduino(items []Item) ([]Entry, error) {
	var out []Entry
	for _, it := range items {
		r, ok := arduino[it.Vendor+"/"+it.Kind+"/"+it.Name]
		if !ok {
			return nil, fmt.Errorf("%s/%s/%s %s: no licence/source rule in internal/sources, add one", it.Vendor, it.Kind, it.Name, it.Version)
		}
		out = append(out, Entry{Item: it, Licenses: r.licenses, Source: r.source(it.Version), Note: r.note})
	}
	sort.Slice(out, func(a, b int) bool {
		return out[a].Vendor+out[a].Kind+out[a].Name < out[b].Vendor+out[b].Kind+out[b].Name
	})
	return out, nil
}

// Licenses lists the distinct SPDX ids of the entries, sorted.
func Licenses(entries []Entry) []string {
	seen := map[string]bool{}
	var ids []string
	for _, e := range entries {
		for _, l := range e.Licenses {
			if !seen[l] {
				seen[l] = true
				ids = append(ids, l)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// FormatArduino is the SOURCES.txt section for the entries.
func FormatArduino(entries []Entry) string {
	var b strings.Builder
	b.WriteString("Arduino software in this archive; licence texts in share/licenses/, the projects'\n")
	b.WriteString("own licence files (where they ship one) in share/doc/<name>/.\n\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "%s %s (%s)  %s\n", e.Name, e.Version, strings.Join(e.Licenses, ", "), e.Source)
		if e.Note != "" {
			fmt.Fprintf(&b, "    %s\n", e.Note)
		}
	}
	return b.String()
}

// DebLine is one SOURCES.txt line for an Ubuntu package (binary package, its version, and
// the source package it was built from).
func DebLine(pkg, version, src, srcVersion string) string {
	if src == "" {
		src = pkg
	}
	if srcVersion == "" {
		srcVersion = version
	}
	return fmt.Sprintf("%s %s  https://launchpad.net/ubuntu/+source/%s/%s", pkg, version, src, srcVersion)
}

// ParseDpkgSearch reads `dpkg-query -S PATH` output ("libusb-1.0-0:amd64: /usr/lib/...")
// and returns the package ("libusb-1.0-0:amd64"). Diversion lines (Ubuntu's t64 transition
// leaves some on /lib paths) are skipped; no owner or several owners are errors.
func ParseDpkgSearch(out string) (string, error) {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l != "" && !strings.HasPrefix(l, "diversion ") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 {
		return "", fmt.Errorf("dpkg-query -S: want one owner, got %q", out)
	}
	pkg, _, ok := strings.Cut(lines[0], ": ")
	if !ok || strings.Contains(pkg, ",") {
		return "", fmt.Errorf("dpkg-query -S: want one owner, got %q", lines[0])
	}
	return pkg, nil
}
