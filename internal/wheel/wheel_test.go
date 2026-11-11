package wheel

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeWheel(t *testing.T, dir, name string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for n, c := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(c))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return p
}

func TestParseName(t *testing.T) {
	for file, want := range map[string]Info{
		"pillow-13.0.0-cp311-cp311-manylinux_2_28_x86_64.whl":          {"pillow", "13.0.0"},
		"pyte-0.8.2-py3-none-any.whl":                                  {"pyte", "0.8.2"},
		"/x/shellcheck_py-0.11.0.1-py2.py3-none-manylinux1_x86_64.whl": {"shellcheck_py", "0.11.0.1"},
		"pkg-1.0-1build-py3-none-any.whl":                              {"pkg", "1.0"},
	} {
		got, err := ParseName(file)
		if err != nil || got != want {
			t.Errorf("%s: %+v %v", file, got, err)
		}
	}
	for _, bad := range []string{"pyte-0.8.2.tar.gz", "a-b.whl", "a-b-c-d-e-f-g.whl"} {
		if _, err := ParseName(bad); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}

func TestTarget(t *testing.T) {
	for member, want := range map[string]string{
		"PIL/Image.py":                    "python/PIL/Image.py",
		"pillow-13.0.0.dist-info/LICENSE": "python/pillow-13.0.0.dist-info/LICENSE",
		"sc-0.1.data/scripts/shellcheck":  "bin/shellcheck",
		"x-1.data/purelib/x/__init__.py":  "python/x/__init__.py",
		"x-1.data/platlib/x/_c.so":        "python/x/_c.so",
		"x-1.data/headers/x.h":            "",
		"x-1.data/data/share/man/x.1":     "",
		"PIL/":                            "",
	} {
		got, err := Target(member)
		if err != nil || got != want {
			t.Errorf("%s: %q %v, want %q", member, got, err, want)
		}
	}
	for _, bad := range []string{"../etc/passwd", "/etc/passwd", "a/../../b"} {
		if _, err := Target(bad); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}

func TestMergeSeveralPythons(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	w11 := makeWheel(t, dir, "p-1-cp311-cp311-linux_x86_64.whl", map[string]string{
		"P/__init__.py": "x", "P/_c.cpython-311.so": "311", "p-1.dist-info/WHEEL": "cp311",
	})
	w12 := makeWheel(t, dir, "p-1-cp312-cp312-linux_x86_64.whl", map[string]string{
		"P/__init__.py": "x", "P/_c.cpython-312.so": "312", "p-1.dist-info/WHEEL": "cp312",
	})
	sc := makeWheel(t, dir, "sc-1-py3-none-any.whl", map[string]string{"sc-1.data/scripts/sc": "#!/bin/sh\n"})
	for _, w := range []string{w11, w12, sc, w11} {
		if err := Merge(root, w); err != nil {
			t.Fatal(err)
		}
	}
	for rel, want := range map[string]string{
		"python/P/_c.cpython-311.so": "311", "python/P/_c.cpython-312.so": "312",
		"python/p-1.dist-info/WHEEL": "cp311", "bin/sc": "#!/bin/sh\n",
	} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || string(b) != want {
			t.Errorf("%s: %q %v", rel, b, err)
		}
	}
	if fi, _ := os.Stat(filepath.Join(root, "bin/sc")); fi.Mode()&0o111 == 0 {
		t.Error("script not executable")
	}
	clash := makeWheel(t, dir, "p-2-py3-none-any.whl", map[string]string{"P/__init__.py": "other"})
	if err := Merge(root, clash); err == nil || !strings.Contains(err.Error(), "other content") {
		t.Fatalf("clash: %v", err)
	}
	evil := makeWheel(t, dir, "e-1-py3-none-any.whl", map[string]string{"../x": "x"})
	if err := Merge(root, evil); err == nil {
		t.Fatal("wrote outside the tree")
	}
}
