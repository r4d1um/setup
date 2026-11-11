// Package fetch downloads release assets and manifests from GitHub: public
// assets over plain HTTPS, private ones with the gh CLI (its login or GH_TOKEN).
package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/r4d1um/setup/internal/manifest"
)

// Source gets a toolchain's release asset into the file dst.
type Source interface {
	Download(ctx context.Context, t manifest.Toolchain, dst string) error
}

// GitHub is the real Source.
type GitHub struct {
	Client  *http.Client // nil: a client with a 30 min timeout
	BaseURL string       // "" means https://github.com (tests point it at a fake)
	GH      string       // gh binary, "" means "gh" from PATH
}

func (g GitHub) Download(ctx context.Context, t manifest.Toolchain, dst string) error {
	if t.Private {
		gh := g.GH
		if gh == "" {
			gh = "gh"
		}
		if _, err := exec.LookPath(gh); err != nil {
			return fmt.Errorf("%s is in the private repo %s: it needs the gh CLI, logged in (gh auth login) or with GH_TOKEN", t.Name, t.Repo)
		}
		// REST only (gh api), not "gh release download": that one uses GraphQL, which cloud sessions block
		q := fmt.Sprintf(`.assets[] | select(.name == "%s") | .id`, t.Asset())
		out, err := exec.CommandContext(ctx, gh, "api", "repos/"+t.Repo+"/releases/tags/"+t.Tag(), "-q", q).Output()
		id := strings.TrimSpace(string(out))
		if err != nil || id == "" {
			return fmt.Errorf("release %s of %s (or its asset %s) not found via gh api: %v", t.Tag(), t.Repo, t.Asset(), err)
		}
		f, err := os.Create(dst)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, gh, "api", "-H", "Accept: application/octet-stream", "repos/"+t.Repo+"/releases/assets/"+id)
		cmd.Stdout, cmd.Stderr = f, os.Stderr
		err = cmd.Run()
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("downloading %s from %s: %w", t.Asset(), t.Repo, err)
		}
		return nil
	}
	base := g.BaseURL
	if base == "" {
		base = "https://github.com"
	}
	url := fmt.Sprintf("%s/%s/releases/download/%s/%s", base, t.Repo, t.Tag(), t.Asset())
	return g.get(ctx, url, dst)
}

func (g GitHub) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return &http.Client{Timeout: 30 * time.Minute}
}

func (g GitHub) get(ctx context.Context, url, dst string) error {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		last = g.getOnce(ctx, url, dst)
		if last == nil || ctx.Err() != nil {
			return last
		}
		if he, ok := last.(httpError); ok && he.code >= 400 && he.code < 500 {
			return last // a missing release won't appear on retry
		}
	}
	return last
}

type httpError struct {
	url  string
	code int
}

func (e httpError) Error() string { return fmt.Sprintf("GET %s: HTTP %d", e.url, e.code) }

func (g GitHub) getOnce(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := g.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httpError{url, resp.StatusCode}
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Manifest reads a manifest from a local path or an http(s) URL.
func (g GitHub) Manifest(ctx context.Context, src string) ([]byte, error) {
	if !strings.HasPrefix(src, "https://") && !strings.HasPrefix(src, "http://") {
		return os.ReadFile(src)
	}
	tmp, err := os.CreateTemp("", "r4setup-manifest-*")
	if err != nil {
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := g.get(ctx, src, tmp.Name()); err != nil {
		return nil, err
	}
	return os.ReadFile(tmp.Name())
}

// SHA256 of a file, lowercase hex.
func SHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Verify fails unless the file's SHA256 is want.
func Verify(path, want string) error {
	got, err := SHA256(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s, manifest says %s", path, got, want)
	}
	return nil
}
