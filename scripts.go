// Package ordvik carries the repository scripts the ordvik plugin installs
// (/ordvik:init copies them to a repository's scripts/), so the CLI can say
// when a repository's copy has drifted from the plugin's.
package ordvik

import (
	"embed"
	"path"
)

// ScriptsDir is where the plugin keeps the scripts, in this module and in
// an installed plugin's root (${CLAUDE_PLUGIN_ROOT}/scripts).
const ScriptsDir = "plugins/ordvik/scripts"

// Scripts are the scripts a repository copies unchanged from the plugin,
// so a fix is made once, in ordvik. ordvik is the wrapper plan and asks
// run in the planning repository's layout. wt is not among them: it is a
// starting point that a repository extends.
var Scripts = []string{"plan", "rfc", "asks", "ordvik"}

//go:embed plugins/ordvik/scripts/plan plugins/ordvik/scripts/rfc plugins/ordvik/scripts/asks plugins/ordvik/scripts/ordvik
var scripts embed.FS

// Script is the plugin's copy of the named script, as of this build.
func Script(name string) ([]byte, error) {
	return scripts.ReadFile(path.Join(ScriptsDir, name))
}
