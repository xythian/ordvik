#!/bin/bash
# SessionStart hook for Claude Code cloud sessions: commit as Ken with the
# attribution hooks on. A cloud container's default git identity is Claude,
# and its harness asks for Claude trailers; Ken is the author of every
# change (scripts/check-attribution). Local sessions are left alone.
set -euo pipefail

if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

cd "${CLAUDE_PROJECT_DIR:-$(dirname "$0")/../..}"
git config user.name "Ken Fox"
git config user.email fox@xythian.com
git config core.hooksPath scripts/git-hooks
