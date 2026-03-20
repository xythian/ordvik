# Claude Instructions

Ordvik is a Claude Code plugin that packages a planning model, RFC management, dispatch documents, and reporting as a distributable toolkit.

## Context

- **Design**: the skills under `plugins/ordvik/skills/` and their `references/` are the specification.
- **Plugin format**: Claude Code plugin with `.claude-plugin/plugin.json` manifest and `skills/` directory

## Repo Structure

```
ordvik/
├── .claude-plugin/
│   └── marketplace.json          # Plugin registry (repo is a marketplace)
├── plugins/
│   └── ordvik/
│       ├── .claude-plugin/
│       │   └── plugin.json       # Plugin manifest
│       └── skills/
│           ├── init/SKILL.md
│           ├── rfc/SKILL.md + references/
│           ├── dispatch/SKILL.md
│           ├── status/SKILL.md
│           ├── report/SKILL.md
│           ├── planning/SKILL.md
│           ├── verify/SKILL.md
│           └── garden/SKILL.md
│       └── backlog.md
├── CLAUDE.md
└── README.md
```

## Development

This repo is a Claude Code marketplace. The ordvik plugin lives in `plugins/ordvik/`. Repo-level files (CLAUDE.md, README.md, docs/) are not part of the plugin distribution.

The marketplace format supports multiple plugins — future plugins can be added as siblings under `plugins/`.
