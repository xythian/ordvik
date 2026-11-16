package planning

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// A shallow clone makes every ask look as new as the clone, so Fetch
// deepens it.
func TestFetchUnshallows(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	gitIn(t, root, "init", "-q", "-b", "main", origin)
	for _, m := range []string{"one", "two", "three"} {
		gitIn(t, origin, "-c", "user.name=T", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", m)
	}
	clone := filepath.Join(root, "clone")
	gitIn(t, root, "clone", "-q", "--depth", "1", "file://"+origin, clone)
	g := Git{Root: clone}
	if !g.Shallow() {
		t.Fatal("the test clone should start shallow")
	}
	if err := g.Fetch(); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if g.Shallow() {
		t.Error("Fetch left the clone shallow")
	}
	// A full clone fetches as before.
	if err := g.Fetch(); err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
}
