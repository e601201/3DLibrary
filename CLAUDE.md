# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 3DLibrary

A web application for managing Blender-authored 3D assets on a local machine. It ships as a **single binary** — a Go HTTP server with the React SPA embedded via `go:embed` — and listens on `127.0.0.1:8765` only (there is no authentication).

## Architecture

### Data flow

```
source/ (source of truth) --[scan: read-only]--> index (SQLite) --> /api/assets --> React SPA
        |                                        ^
        +--[generate: one Blender CLI run]--> cache/ (4 artifacts) --(picked up by next scan)--+
```

**`source/` is the single source of truth.** `cache/` (GLB, thumbnail, extracted metadata, sprite) and `database.db` can be deleted at any time and rebuilt from it. This invariant shapes the whole design — don't break it.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage labels are used as-is (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
