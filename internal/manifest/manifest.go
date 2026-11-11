// Package manifest reads and writes manifest.json: which toolchain lives in
// which repo, at which version, and the SHA256 its release asset must have.
//
// Contract (fields are only ever added, never renamed):
//
//	{"schema": 1, "toolchains": [{"name": "avr-gcc", "repo": "r4d1um/setup",
//	  "version": "7.3.0-r1", "sha256": "…", "private": false}]}
//
// The release tag is "<name>-<version>", the asset "<name>-<version>-linux-amd64.tar.gz".
package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
)

const Schema = 1

type Toolchain struct {
	Name    string `json:"name"`
	Repo    string `json:"repo"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Private bool   `json:"private,omitempty"`
}

type Manifest struct {
	Schema     int         `json:"schema"`
	Toolchains []Toolchain `json:"toolchains"`
}

var (
	nameRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	versionRe = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+~_-]*$`)
	repoRe    = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)
	shaRe     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Tag is the GitHub release tag that carries the toolchain.
func (t Toolchain) Tag() string { return t.Name + "-" + t.Version }

// Asset is the file name of the release asset.
func (t Toolchain) Asset() string { return AssetName(t.Name, t.Version) }

// AssetName is the asset file name for a toolchain version (used by the builders too).
func AssetName(name, version string) string { return name + "-" + version + "-linux-amd64.tar.gz" }

// Check reports whether the entry is well formed. An empty sha256 means "not released yet".
func (t Toolchain) Check() error {
	switch {
	case !nameRe.MatchString(t.Name):
		return fmt.Errorf("bad toolchain name %q", t.Name)
	case !versionRe.MatchString(t.Version):
		return fmt.Errorf("%s: bad version %q", t.Name, t.Version)
	case !repoRe.MatchString(t.Repo):
		return fmt.Errorf("%s: bad repo %q", t.Name, t.Repo)
	case t.SHA256 != "" && !shaRe.MatchString(t.SHA256):
		return fmt.Errorf("%s: sha256 must be 64 lowercase hex digits", t.Name)
	}
	return nil
}

// Parse reads and checks a manifest.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if m.Schema != Schema {
		return nil, fmt.Errorf("manifest: schema %d, this r4setup knows %d (update bootstrap.sh's tool version)", m.Schema, Schema)
	}
	seen := map[string]bool{}
	for _, t := range m.Toolchains {
		if err := t.Check(); err != nil {
			return nil, fmt.Errorf("manifest: %w", err)
		}
		if seen[t.Name] {
			return nil, fmt.Errorf("manifest: %s listed twice", t.Name)
		}
		seen[t.Name] = true
	}
	return &m, nil
}

// Find returns the entry for name.
func (m *Manifest) Find(name string) (Toolchain, bool) {
	for _, t := range m.Toolchains {
		if t.Name == name {
			return t, true
		}
	}
	return Toolchain{}, false
}

// Pin sets (or adds) an entry, keeping the list sorted by name.
func (m *Manifest) Pin(t Toolchain) error {
	if err := t.Check(); err != nil {
		return err
	}
	if t.SHA256 == "" {
		return fmt.Errorf("%s: pin needs a sha256", t.Name)
	}
	for i := range m.Toolchains {
		if m.Toolchains[i].Name == t.Name {
			m.Toolchains[i] = t
			return nil
		}
	}
	m.Toolchains = append(m.Toolchains, t)
	sort.Slice(m.Toolchains, func(i, j int) bool { return m.Toolchains[i].Name < m.Toolchains[j].Name })
	return nil
}

// Marshal renders the manifest the way it is committed (2-space indent, trailing newline).
func (m *Manifest) Marshal() []byte {
	if m.Toolchains == nil {
		m.Toolchains = []Toolchain{}
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return append(b, '\n')
}
