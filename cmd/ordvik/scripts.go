package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xythian/ordvik"
)

const scriptsUsage = `ordvik scripts: compare a repository's copies of the plugin's scripts with ordvik's.

Usage:
  ordvik scripts check [<repo>] [--diff] [--plugin <dir>]
                                           Say which of scripts/plan, rfc, asks and ordvik
                                           differ from ordvik's; exit 1 if any do
  ordvik scripts update [<repo>] [--plugin <dir>]
                                           Replace the copies that differ with ordvik's

A repository copies these scripts from the plugin (/ordvik:init), and a fix
belongs in ordvik's plugins/ordvik/scripts/, so that every repository gets
it. A copy the repository does not have is not checked. wt is not checked:
a repository extends it.

<repo> defaults to the repository holding the working directory. ordvik's
copies are the ones this binary was built with; --plugin <dir> reads them
from an installed plugin (${CLAUDE_PLUGIN_ROOT}) or an ordvik checkout
instead. In a repository's tests:

  go run github.com/xythian/ordvik/cmd/ordvik@main scripts check

Exit status: 0, 1 when check finds a copy that differs or on errors, 2 on
bad usage.
`

type drift struct {
	name   string
	reason string // "" when the copy is the same as ordvik's
	diff   string
	want   []byte
}

func scriptsCmd(argv []string, stdout, stderr io.Writer) int {
	if len(argv) == 0 || argv[0] == "-h" || argv[0] == "--help" || argv[0] == "help" {
		fmt.Fprint(stdout, scriptsUsage)
		if len(argv) == 0 {
			return 2
		}
		return 0
	}
	cmd := argv[0]
	if cmd != "check" && cmd != "update" {
		fmt.Fprintf(stderr, "Error: unknown command %q.\n\n%s", cmd, scriptsUsage)
		return 2
	}
	var repo, plugin string
	showDiff := false
	for i := 1; i < len(argv); i++ {
		switch a := argv[i]; {
		case a == "--diff" && cmd == "check":
			showDiff = true
		case a == "--plugin":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "Error: --plugin takes a directory.")
				return 2
			}
			i++
			plugin = argv[i]
		case strings.HasPrefix(a, "-") || repo != "":
			fmt.Fprintf(stderr, "Error: unexpected argument %q.\n", a)
			return 2
		default:
			repo = a
		}
	}
	root, err := repoRoot(repo)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	found, err := compareScripts(root, plugin)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if len(found) == 0 {
		fmt.Fprintf(stdout, "%s has none of ordvik's scripts (scripts/%s).\n", root, strings.Join(ordvik.Scripts, ", scripts/"))
		return 0
	}
	var same, differ []string
	for _, d := range found {
		if d.reason == "" {
			same = append(same, "scripts/"+d.name)
			continue
		}
		differ = append(differ, "scripts/"+d.name)
		if cmd == "update" {
			if err := os.WriteFile(filepath.Join(root, "scripts", d.name), d.want, 0o755); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return 1
			}
			if err := os.Chmod(filepath.Join(root, "scripts", d.name), 0o755); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return 1
			}
			fmt.Fprintf(stdout, "updated scripts/%s (it %s)\n", d.name, d.reason)
			continue
		}
		fmt.Fprintf(stdout, "scripts/%s %s\n", d.name, d.reason)
		if showDiff && d.diff != "" {
			fmt.Fprint(stdout, d.diff)
		}
	}
	if cmd == "update" {
		if len(differ) == 0 {
			fmt.Fprintf(stdout, "%s: same as ordvik's.\n", strings.Join(same, ", "))
		}
		return 0
	}
	if len(same) > 0 {
		fmt.Fprintf(stdout, "%s: same as ordvik's.\n", strings.Join(same, ", "))
	}
	if len(differ) == 0 {
		return 0
	}
	fmt.Fprintf(stdout, "\n%s drifted from ordvik's plugins/ordvik/scripts/. Make the change in ordvik, then\n"+
		"`ordvik scripts update` copies ordvik's here (`--diff` shows the differences).\n", strings.Join(differ, ", "))
	return 1
}

// repoRoot is the top of the git work tree holding dir (or the working
// directory), else dir itself.
func repoRoot(repo string) (string, error) {
	start := repo
	if start == "" {
		start = dir
	}
	if fi, err := os.Stat(start); err != nil {
		return "", err
	} else if !fi.IsDir() {
		return "", fmt.Errorf("%s is not a directory", start)
	}
	out, err := exec.Command("git", "-C", start, "rev-parse", "--show-toplevel").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return filepath.Abs(start)
	}
	return strings.TrimSpace(string(out)), nil
}

// pluginScript is ordvik's copy of name: from plugin (a plugin root holding
// scripts/, or an ordvik checkout) when given, else the embedded one.
func pluginScript(plugin, name string) ([]byte, error) {
	if plugin == "" {
		return ordvik.Script(name)
	}
	for _, p := range []string{filepath.Join(plugin, "scripts", name), filepath.Join(plugin, filepath.FromSlash(ordvik.ScriptsDir), name)} {
		b, err := os.ReadFile(p)
		if err == nil {
			return b, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%s has no scripts/%s (give a plugin root or an ordvik checkout)", plugin, name)
}

func compareScripts(root, plugin string) ([]drift, error) {
	var out []drift
	for _, name := range ordvik.Scripts {
		p := filepath.Join(root, "scripts", name)
		fi, err := os.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		have, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		want, err := pluginScript(plugin, name)
		if err != nil {
			return nil, err
		}
		d := drift{name: name, want: want}
		switch {
		case !bytes.Equal(have, want):
			var added, removed int
			d.diff, added, removed = unified(name, want, p)
			d.reason = "differs from ordvik's"
			if d.diff != "" {
				d.reason += fmt.Sprintf(" (+%d -%d lines)", added, removed)
			}
		case fi.Mode()&0o111 == 0:
			d.reason = "is not executable"
		}
		out = append(out, d)
	}
	return out, nil
}

// unified is diff -u from ordvik's copy to the repository's, with the lines
// each side adds; "" when diff is not installed.
func unified(name string, want []byte, have string) (string, int, int) {
	tmp, err := os.CreateTemp("", "ordvik-"+name+"-")
	if err != nil {
		return "", 0, 0
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(want); err != nil {
		tmp.Close()
		return "", 0, 0
	}
	tmp.Close()
	out, err := exec.Command("diff", "-u", "-L", "ordvik/"+ordvik.ScriptsDir+"/"+name, "-L", "scripts/"+name, tmp.Name(), have).Output()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return "", 0, 0
	}
	added, removed := 0, 0
	for _, l := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(l, "+++") || strings.HasPrefix(l, "---"):
		case strings.HasPrefix(l, "+"):
			added++
		case strings.HasPrefix(l, "-"):
			removed++
		}
	}
	return string(out), added, removed
}
