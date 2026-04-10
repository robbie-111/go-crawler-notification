# AGENTS.md

## Quick Start
- This is a single-module Go desktop app built with Fyne; the process entrypoint is `main.go`, which creates the UI in `internal/ui/app.go`.
- Use `go run .` to launch the app locally. This opens a GUI window, so headless verification should prefer package-level commands.
- Verified repo-wide checks: `go test ./...` and `go build ./...`.

## Structure That Matters
- `internal/ui/app.go`: wires the whole app together, owns input validation, button behavior, and monitor lifecycle.
- `internal/monitor/monitor.go`: polling loop and alert logic; runs one immediate check before the ticker loop.
- `internal/crawler/crawler.go`: decides between raw fetch and HTML parsing. It does a `HEAD` request first, then either plain HTTP GET or Colly HTML scraping.
- `internal/state/state.go`: persists version history to `monitor_state.json` in the repo root.
- `internal/normalize/url.go`: rewrites GitHub `blob` URLs to `raw.githubusercontent.com` before fetch and before state keys are stored.

## Gotchas
- Version-change alerts are stateful. `monitor_state.json` suppresses repeat "new version" notifications after the first seen version is stored; reset or remove that file if you need a clean version-detection test.
- Keyword alerts are edge-triggered in memory (`lastMatched` in `Runner.loop`), so repeated matches do not re-alert until the keyword disappears once.
- If both detection checkboxes are off, the UI intentionally rejects start/test actions.
- There are currently no Go test files in the repo, so `go test ./...` only verifies compilation.

## Editing Notes
- Keep changes inside existing packages unless the wiring truly changes; this repo is small and centered around `ui -> monitor -> crawler/state/version`.
- Be careful with `monitor_state.json`: it is runtime data, not code, and changes there affect alert behavior during manual verification.
