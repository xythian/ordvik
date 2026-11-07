package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xythian/ordvik"
)

func runScripts(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(append([]string{"scripts"}, args...), &out, &errb)
	return code, out.String() + errb.String()
}

func installScript(t *testing.T, root, name string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", name), body, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "scripts", name), mode); err != nil {
		t.Fatal(err)
	}
}

func TestScriptsCheckFindsDrift(t *testing.T) {
	root := t.TempDir()
	plan, _ := ordvik.Script("plan")
	rfc, _ := ordvik.Script("rfc")
	installScript(t, root, "plan", plan, 0o755)
	installScript(t, root, "rfc", append(append([]byte{}, rfc...), "# a local fix\n"...), 0o755)

	code, out := runScripts(t, "check", root)
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	if !strings.Contains(out, "scripts/rfc differs from ordvik's (+1 -0 lines)") ||
		!strings.Contains(out, "scripts/plan: same as ordvik's.") {
		t.Fatalf("output:\n%s", out)
	}
	if strings.Contains(out, "scripts/asks") && !strings.Contains(out, "none of") {
		t.Fatalf("an absent script was checked:\n%s", out)
	}

	code, out = runScripts(t, "check", root, "--diff")
	if code != 1 || !strings.Contains(out, "+# a local fix") || !strings.Contains(out, "--- ordvik/plugins/ordvik/scripts/rfc") {
		t.Fatalf("--diff: exit %d:\n%s", code, out)
	}

	code, out = runScripts(t, "update", root)
	if code != 0 || !strings.Contains(out, "updated scripts/rfc") {
		t.Fatalf("update: exit %d:\n%s", code, out)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "scripts", "rfc")); !bytes.Equal(got, rfc) {
		t.Fatal("update did not write ordvik's rfc")
	}
	if code, out = runScripts(t, "check", root); code != 0 {
		t.Fatalf("check after update: exit %d:\n%s", code, out)
	}
}

func TestScriptsCheckModeAndNone(t *testing.T) {
	root := t.TempDir()
	if code, out := runScripts(t, "check", root); code != 0 || !strings.Contains(out, "none of ordvik's scripts") {
		t.Fatalf("no scripts: exit %d:\n%s", code, out)
	}
	asks, _ := ordvik.Script("asks")
	installScript(t, root, "asks", asks, 0o644)
	if code, out := runScripts(t, "check", root); code != 1 || !strings.Contains(out, "scripts/asks is not executable") {
		t.Fatalf("mode: exit %d:\n%s", code, out)
	}
	runScripts(t, "update", root)
	if fi, _ := os.Stat(filepath.Join(root, "scripts", "asks")); fi.Mode()&0o111 == 0 {
		t.Fatal("update left scripts/asks not executable")
	}
}

func TestScriptsCheckAgainstAPluginDir(t *testing.T) {
	root, plugin := t.TempDir(), t.TempDir()
	installScript(t, root, "ordvik", []byte("#!/bin/sh\necho old\n"), 0o755)
	installScript(t, plugin, "ordvik", []byte("#!/bin/sh\necho old\n"), 0o755)
	if code, out := runScripts(t, "check", root, "--plugin", plugin); code != 0 {
		t.Fatalf("plugin root: exit %d:\n%s", code, out)
	}
	// An ordvik checkout: this module's own copies.
	if code, out := runScripts(t, "check", root, "--plugin", "../.."); code != 1 {
		t.Fatalf("checkout: exit %d:\n%s", code, out)
	}
	if code, _ := runScripts(t, "check", root, "--plugin", t.TempDir()); code != 1 {
		t.Fatal("a directory with no scripts was accepted")
	}
	if code, _ := runScripts(t, "check", root, "--bogus"); code != 2 {
		t.Fatal("bad usage did not exit 2")
	}
}
