package elfdeps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A program in the tree needs a library that only the "host" has: Bundle copies it
// into lib/host and sets a RUNPATH, so the program runs without LD_LIBRARY_PATH.
func TestBundleCopiesHostOnlyLibrary(t *testing.T) {
	for _, tool := range []string{"gcc", "patchelf"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	host, root := t.TempDir(), t.TempDir()
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(host, "greet.c"), `const char *greet(void) { return "hello from the host lib"; }`)
	write(filepath.Join(host, "main.c"), `#include <stdio.h>
const char *greet(void);
int main(void) { puts(greet()); return 0; }`)
	run := func(name string, args ...string) {
		if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	lib := filepath.Join(host, "libgreet.so.1")
	run("gcc", "-shared", "-fPIC", "-Wl,-soname,libgreet.so.1", "-o", lib, filepath.Join(host, "greet.c"))
	os.MkdirAll(filepath.Join(root, "bin"), 0o755)
	exe := filepath.Join(root, "bin", "hello")
	run("gcc", "-o", exe, filepath.Join(host, "main.c"), lib)

	lookup := func(s string) string {
		if s == "libgreet.so.1" {
			return lib
		}
		return ""
	}
	copied, err := Bundle(root, lookup, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 1 || copied[0] != "libgreet.so.1" {
		t.Fatalf("copied %v", copied)
	}
	os.Remove(lib) // the "host" no longer has it
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "hello from the host lib") {
		t.Fatalf("run: %v %s", err, out)
	}
	// a second run changes nothing
	if again, err := Bundle(root, lookup, t.Logf); err != nil || len(again) != 0 {
		t.Fatalf("second run: %v %v", again, err)
	}
}

func TestBundleFailsOnUnknownLibrary(t *testing.T) {
	for _, tool := range []string{"gcc", "patchelf"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	dir, root := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.c"), []byte("int x(void){return 1;}"), 0o644)
	os.WriteFile(filepath.Join(dir, "m.c"), []byte("int x(void); int main(void){return x();}"), 0o644)
	lib := filepath.Join(dir, "libx.so.9")
	exec.Command("gcc", "-shared", "-fPIC", "-Wl,-soname,libx.so.9", "-o", lib, filepath.Join(dir, "x.c")).Run()
	exec.Command("gcc", "-o", filepath.Join(root, "m"), filepath.Join(dir, "m.c"), lib).Run()
	if _, err := Bundle(root, func(string) string { return "" }, t.Logf); err == nil || !strings.Contains(err.Error(), "libx.so.9") {
		t.Fatalf("err = %v", err)
	}
}

func TestOriginAndMergeRpath(t *testing.T) {
	if got := origin("/t/libexec/gcc/avr/7", "/t/lib/host"); got != "$ORIGIN/../../../../lib/host" {
		t.Fatal(got)
	}
	if got := origin("/t/lib", "/t/lib"); got != "$ORIGIN" {
		t.Fatal(got)
	}
	if got := mergeRpath([]string{"$ORIGIN/../lib"}, []string{"$ORIGIN/../lib/", "$ORIGIN/../lib/host"}); got != "$ORIGIN/../lib:$ORIGIN/../lib/host" {
		t.Fatal(got)
	}
}
