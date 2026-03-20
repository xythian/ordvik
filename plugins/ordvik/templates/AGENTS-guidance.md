## Codebase Exploration

When searching or exploring the codebase, prefer built-in tools over shell commands:

- **Find files**: Use `Glob` (not `find` or `ls -R`)
- **Search content**: Use `Grep` (not `grep`, `rg`, `awk`)
- **Read files**: Use `Read` (not `cat`, `head`, `tail`, `sed`)
- **Edit files**: Use `Edit` (not `sed -i`, `awk`, `perl -pi`)
- **Write files**: Use `Write` (not `echo >`, `cat <<EOF >`)
- **Complex exploration**: Use the `Agent` tool with `subagent_type=Explore`

Only fall back to shell commands when you need something these tools genuinely can't do (e.g., `wc -l`, `file`, `stat`, process-related commands). Built-in tools are faster, safer, and don't require per-command approval.

## Subagent Communication

Subagents return results as text directly to the parent agent via the Agent tool's return value. Do NOT write files to `/tmp/` or anywhere else to pass data between agents. Instead, include all relevant findings in your text output — that IS the result the parent receives.

## Build & Test

Always use project build/test scripts rather than bare language commands:

```bash
# Do this:
./scripts/test.sh
./scripts/build.sh

# Not this:
go test ./...
cargo build
npm run build
```

The scripts handle build tags, cross-compilation, test fixtures, and other project-specific concerns. Bare commands may miss build constraints or produce incorrect results.

For test tiers:
- **Fast tier** (default): `./scripts/test.sh` — use during iterative development
- **Full tier**: `./scripts/test.sh full` — use before declaring work complete
- **Integration tests**: `./scripts/test.sh list` to discover available suites
