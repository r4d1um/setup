// SPDX-License-Identifier: MIT

// r4setup installs r4d1um's prebuilt toolchains (see manifest.json) and pins new releases.
//
//	r4setup install -manifest <path|url> <toolchain>...
//	r4setup list    -manifest <path|url>
//	r4setup prefix  -manifest <path|url> <toolchain>     # where it is installed
//	r4setup pin     -manifest manifest.json -name N -repo R -version V (-sha S | -asset FILE) [-private]
//	r4setup version
//
// Normally started by bootstrap.sh. Environment: R4SETUP_HOME (default
// ~/.local/share/r4setup), R4SETUP_BIN (default ~/.local/bin), R4SETUP_MANIFEST,
// R4SETUP_GITHUB (default https://github.com; tests use a local server).
// In GitHub Actions the toolchains' bin directories also go to $GITHUB_PATH.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/r4d1um/setup/internal/fetch"
	"github.com/r4d1um/setup/internal/install"
	"github.com/r4d1um/setup/internal/manifest"
)

var version = "dev" // set with -ldflags "-X main.version=..."

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "r4setup:", err)
		os.Exit(1)
	}
}

func usage() error {
	return fmt.Errorf("usage: r4setup install|list|prefix|pin|version (see the comment at the top of cmd/r4setup/main.go)")
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cmd, args := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	manifestSrc := fs.String("manifest", os.Getenv("R4SETUP_MANIFEST"), "manifest.json path or URL")
	switch cmd {
	case "version":
		fmt.Println(version)
		return nil
	case "install", "list", "prefix":
		if err := fs.Parse(args); err != nil {
			return err
		}
		m, err := loadManifest(ctx, *manifestSrc)
		if err != nil {
			return err
		}
		in := installer()
		switch cmd {
		case "install":
			if err := in.Install(ctx, m, fs.Args()); err != nil {
				return err
			}
			if in.BinDir != "" && !onPath(in.BinDir) {
				fmt.Fprintf(os.Stderr, "r4setup: %s is not on PATH; add it:  export PATH=\"%s:$PATH\"\n", in.BinDir, in.BinDir)
			}
		case "list":
			for _, t := range m.Toolchains {
				state := "not installed"
				switch {
				case t.SHA256 == "":
					state = "no release yet"
				case in.Installed(t):
					state = "installed"
				}
				where := "public"
				if t.Private {
					where = "private"
				}
				fmt.Printf("%-22s %-20s %-22s %-8s %s\n", t.Name, t.Version, t.Repo, where, state)
			}
		case "prefix":
			if fs.NArg() != 1 {
				return fmt.Errorf("usage: r4setup prefix <toolchain>")
			}
			t, ok := m.Find(fs.Arg(0))
			if !ok || !in.Installed(t) {
				return fmt.Errorf("%s is not installed", fs.Arg(0))
			}
			fmt.Println(in.Dir(t))
		}
		return nil
	case "pin":
		name := fs.String("name", "", "toolchain name")
		repo := fs.String("repo", "", "owner/repo holding the release")
		ver := fs.String("version", "", "toolchain version")
		sha := fs.String("sha", "", "sha256 of the asset")
		asset := fs.String("asset", "", "the asset file (sha256 computed from it)")
		private := fs.Bool("private", false, "the repo is private (download with gh)")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if *asset != "" {
			s, err := fetch.SHA256(*asset)
			if err != nil {
				return err
			}
			*sha = s
		}
		return pin(*manifestSrc, manifest.Toolchain{Name: *name, Repo: *repo, Version: *ver, SHA256: *sha, Private: *private})
	}
	return usage()
}

func loadManifest(ctx context.Context, src string) (*manifest.Manifest, error) {
	if src == "" {
		return nil, fmt.Errorf("no manifest: pass -manifest or set R4SETUP_MANIFEST (bootstrap.sh does this)")
	}
	data, err := fetch.GitHub{}.Manifest(ctx, src)
	if err != nil {
		return nil, err
	}
	return manifest.Parse(data)
}

func pin(path string, t manifest.Toolchain) error {
	if path == "" || strings.Contains(path, "://") {
		return fmt.Errorf("pin needs a local -manifest file")
	}
	m := &manifest.Manifest{Schema: manifest.Schema}
	if data, err := os.ReadFile(path); err == nil {
		if m, err = manifest.Parse(data); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := m.Pin(t); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, m.Marshal(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	fmt.Printf("pinned %s %s (%s) in %s\n", t.Name, t.Version, t.SHA256, path)
	return nil
}

func installer() *install.Installer {
	home, _ := os.UserHomeDir()
	in := &install.Installer{
		Home:   os.Getenv("R4SETUP_HOME"),
		BinDir: os.Getenv("R4SETUP_BIN"),
		Source: fetch.GitHub{BaseURL: os.Getenv("R4SETUP_GITHUB")}, // tests point this at a fake
		Log:    func(f string, a ...any) { fmt.Fprintf(os.Stderr, "r4setup: "+f+"\n", a...) },
	}
	if in.Home == "" {
		data := os.Getenv("XDG_DATA_HOME")
		if data == "" {
			data = filepath.Join(home, ".local", "share")
		}
		in.Home = filepath.Join(data, "r4setup")
	}
	if in.BinDir == "" {
		in.BinDir = filepath.Join(home, ".local", "bin")
	}
	if in.PythonBase = os.Getenv("PYTHONUSERBASE"); in.PythonBase == "" {
		in.PythonBase = filepath.Join(home, ".local")
	}
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		in.GitHubPath = os.Getenv("GITHUB_PATH")
	}
	return in
}

func onPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(p) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}
