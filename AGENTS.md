# AGENTS.md

Primary engineering instructions for AI coding agents (Claude Code, OpenAI Codex, Gemini / Antigravity) working on **ProjectHub**.

## 1. Project Context

- **Name:** ProjectHub
- **Shared-Memory Project ID:** `ProjectHub` (use `Project:ProjectHub`, `Service:ProjectHub:<service>`, `Module:ProjectHub:<module>`, `Decision:ProjectHub:<decision>`, `Integration:ProjectHub:<integration>`, `Bug:ProjectHub:<bug>` in the `shared-memory` MCP knowledge graph)
- **Purpose:** Lightweight, local-first Linux desktop project manager inspired by the Projects view of JetBrains Toolbox, with an independent visual identity.
- **Repository:** `https://github.com/semelyanov86/ProjectHub` (MIT). Current version: see `VERSION`.
- **Target Platforms:**
  - Ubuntu 24.04 LTS (build baseline) & Ubuntu 26.04 LTS
  - GNOME desktop environment (Wayland and X11)
  - Primary architecture: `linux/amd64` (`linux/arm64` optional in future releases)
- **Supported Editors & Tools:**
  - Editors: Sublime Text (`subl`) and Zed (`zed`, `zeditor`); default: Zed
  - Terminal: Ghostty or a configured custom terminal
  - System file manager (`gio open`, then `xdg-open`, or a configured executable)
  - Local Git repository inspection
  - System tray through a StatusNotifierItem host (Ubuntu AppIndicators extension, KDE, …); optional
- **Technology Stack:**
  - **Backend:** Go 1.26+, Wails 3 (`github.com/wailsapp/wails/v3` v3.1.0, GTK4 + WebKitGTK 6.0), standard Go libraries wherever practical; `github.com/godbus/dbus/v5` (already a Wails dependency) only for tray-host detection
  - **Frontend:** Vue 3 (`<script setup lang="ts">`), strict TypeScript, Vite, Tailwind CSS v4, Pinia, `@lucide/vue`
  - **Tooling:** Go Task (`Taskfile.yml`), Git, `golangci-lint` v2 (`.golangci.yml`, `gofumpt`, `goimports`, `govet`, `gosec`, `staticcheck`), `govulncheck`, ESLint, Prettier (`prettier-plugin-tailwindcss`), Vitest (`@vitest/coverage-v8`), Go testing (`go test`)
  - **Packaging:** `wails3 tool package` (nFPM) → `dist/projecthub_<version>_<arch>.deb`; GitHub Actions for CI and tagged releases
  - **Storage:** Versioned JSON files following the XDG Base Directory Specification (`~/.config/projecthub/projects.json` and `~/.config/projecthub/settings.json`). No database, no cloud sync, no authentication, no telemetry.

### Architectural Overview

```text
ProjectHub/
├── main.go                    # Wails app bootstrap: window, tray, close hook, single instance, --version/--help
├── VERSION                    # Single source of the version (embedded into the binary, passed to the package)
├── Taskfile.yml               # Unified Go Task entrypoints integrated with Wails 3 build tasks
├── build/
│   ├── config.yml, Taskfile.yml       # Wails build config and shared tasks
│   ├── appicon.png, icons/            # Icon sources (projecthub.svg, projecthub-small.svg) and tray.png
│   └── linux/
│       ├── Taskfile.yml               # Wails Linux build/packaging tasks (PIE production build, stage + nFPM)
│       ├── nfpm/nfpm.yaml             # Debian package definition (contents, Depends)
│       ├── share/                     # Files installed below /usr/share (desktop entry, metainfo, icons, man, doc)
│       ├── verify-deb.sh              # Package verification used by `task deb`
│       ├── smoke-test.sh, smoke/      # Docker install + GUI smoke tests (X11, Wayland, tray)
│       └── appimage/                  # Wails AppImage helper (not a release artifact)
├── scripts/                   # doctor, version check, icon rendering, release notes
├── docs/                      # RELEASING.md, screenshots
├── .github/workflows/         # ci.yml (PRs/pushes), release.yml (tags v*)
├── internal/
│   ├── app/                   # Version, config/home directory and tray availability for the UI
│   ├── config/                # XDG JSON v2 schemas, migration, transactions, import/export, settings service
│   ├── desktop/               # ID-based Wails facade, native dialogs/drops, events, launch and transfer coordination
│   ├── project/               # Domain validation and transactional catalog mutations
│   ├── editor/                # Supported editor catalog and lookup helpers
│   ├── launcher/              # Directory validation, executable detection, launch arguments and presets
│   ├── process/               # Runner abstraction, bounded runs, detached starts, environment sanitising
│   ├── git/                   # Bounded read-only status inspection and porcelain-v2 parsing
│   └── tray/                  # Tray menu model (pure), Wails SystemTray adapter, StatusNotifierWatcher detection
└── frontend/
    └── src/
        ├── assets/            # Global CSS, Tailwind v4 import, light/dark theme tokens
        ├── components/        # Focused UI components and reusable ui/ primitives
        ├── composables/       # Reactive composables (e.g. theme resolution)
        ├── services/          # Typed wrapper around generated Wails 3 bindings
        ├── stores/            # App/settings, projects, Git inspections, tools, notifications
        ├── lib/               # Filtering, display helpers, and error messages
        └── views/             # Top-level views
```

- **Backend responsibilities:** Filesystem operations, XDG JSON persistence, validation, spawning external apps safely, Git inspection, tray, window lifecycle.
- **Frontend responsibilities:** UI rendering, filtering, Pinia state management, user interactions, theme switching.
- **Frontend API:** Components and stores use only `frontend/src/services/backend.ts` to call `desktop.Service`, `config.Service`, and `app.Service`. The desktop facade coordinates project operations by ID; catalog, launcher, process runner, Git inspector and tray are not bound services. Shared TypeScript domain types come from generated bindings through this wrapper.
- **Events (Go → UI):** Typed events are registered statically with `application.RegisterEvent` in `init()` functions so Wails generates their types: `projects:dropped` (`DropResult`, `internal/desktop/dialogs.go`), `app:open-settings` (void), `app:notice` (`Notice`) and `projects:changed` (void) in `internal/desktop/events.go`. Preserve the static registration.
- **Native drops:** `desktop.DropHandler` accepts native file drops only and is not bound to JavaScript.
- **Tray and window lifecycle (`main.go`, `internal/tray`):** The tray menu is rebuilt from the catalog after every successful `projects.json` write (`config.Store.OnProjectsChanged` → `tray.Refresh`). The `WindowClosing` hook (registered with `RegisterHook`, which runs before Wails destroys the window) saves the window size and hides the window only when `tray.HideOnClose(settings.closeBehavior, tray.HostAvailable)` is true; otherwise the close quits. While quitting (tray Quit, `OnShutdown`) closes are never cancelled. A second launch (Wails `SingleInstance`) shows and focuses the existing window. Never leave the app running hidden without a tray host. dbusmenu labels must escape `_` as `__` (`tray.MenuLabel`).
- **Wails 3 bindings:** Generated TypeScript bindings in `frontend/bindings/` (gitignored, regenerated during build via `wails3 generate bindings -ts -i`). Do not edit files in `frontend/bindings/` manually.

## 2. Development Conventions

- **Idiomatic Go:** Standard formatting (`gofumpt` / `goimports` via `golangci-lint fmt`), small single-purpose packages, explicit error wrapping (`fmt.Errorf("...: %w", err)`), sentinel errors where callers branch on failure kinds (`ErrCorruptedConfig`, `ErrInvalidConfig`, etc.). All Go changes must pass `.golangci.yml` checks (`golangci-lint run . ./internal/...`).
- **Toolchain PATH:** Put `$(go env GOPATH)/bin` first on `PATH` for `wails3` and `golangci-lint` v2; use a linter build compatible with the installed Go version (CI uses golangci-lint v2.15.0 built with the same Go).
- **Strict TypeScript:** Keep `strict: true`, `noUnusedLocals: true`, `noUnusedParameters: true`, and `noImplicitAny: true`. Avoid `any`.
- **Vue Composition API:** Always use `<script setup lang="ts">`. Keep components small, focused, and easy to test.
- **Separation of Concerns:** Never place OS/filesystem/process execution logic in the frontend, and never embed UI state concerns in backend packages.
- **No Overengineering:** Avoid unnecessary repository abstractions, redundant interfaces, or extra dependencies when Go stdlib or Vue/Pinia primitives suffice.
- **User-facing errors:** Launch failures are returned as `desktop.LaunchError` with a short, actionable message (the notification title already names the project); the launcher error stays wrapped for `errors.Is`.
- **Configuration Safety:**
  - All JSON writes must be atomic (write to temp file in same directory -> `Sync` -> `Close` -> `Rename` -> fsync the directory).
  - Never silently overwrite corrupted JSON files; surface descriptive errors and preserve user data on disk.
  - Catalog mutations must use `Store.UpdateProjects` locked read-modify-write transactions through `project.Catalog`; replacement imports use `Store.ReplaceProjectsWithBackup`. Do not implement load-then-save mutation sequences.
  - `Store.EnsureFiles` creates missing `settings.json`/`projects.json` with defaults at startup and never modifies existing files (corrupted ones included).
  - Loading schema v1 migrates in memory only; the next save writes v2. Preserve this no-write-on-load behavior.
  - Maintain versioned schemas (`schemaVersion`) and preserve existing user configuration during upgrades. Purely additive optional fields with a default applied on load (like `closeBehavior`) do not need a schema bump; anything that changes the meaning of existing data does.
  - Document new settings fields in `README.md` and in `internal/config/settings.go`.

## 3. Security Requirements

- **Never construct shell commands (`sh -c`, `bash -c`) from project paths or user metadata.**
- **Always use `exec.Command(binary, arg1, arg2, ...)` with separate arguments.**
- **Launch routing:** All application launches must go through `internal/launcher` and `internal/process.Runner`. Keep bounded `Run` calls and detached `Start` calls behind the runner abstraction. Tests must use fake runners and must never start real desktop applications.
- **Read-only Git:** Keep inspection read-only, retain `core.fsmonitor=false`, the filter-driver neutralisation (`internal/git/filters.go`), `GIT_ALLOW_PROTOCOL=`/`GIT_NO_LAZY_FETCH=1` and the other safety flags, sanitize inherited `GIT_*` variables, and preserve timeout/concurrency bounds. Never execute repository-configured monitor hooks, filters or transports; `TestInspectorNeverRunsRepositoryFilters` must keep passing.
- **Bounded filesystem checks:** Directory existence checks for inspection run concurrently under a timeout (`internal/desktop/inspect.go`), so unresponsive network mounts cannot block a refresh.
- **Native file access:** Dialogs and file paths used to read imports or write exports stay in Go. Never accept JavaScript-supplied paths for reading or writing files; use native dialogs and prepared-import tokens. Project directory metadata is validated in Go before use.
- **Validate project paths before launching any external application** (reject empty paths, NUL bytes, relative paths, and non-directory targets using `launcher.ValidateProjectDir`).
- **Handle paths containing spaces and special characters safely** by passing raw validated paths as discrete OS arguments, never shell-quoted strings.
- **Never execute arbitrary scripts or hooks from project metadata.**
- **No `sudo` or elevated privileges at runtime.** Only `task install`/`task uninstall` use `sudo apt`, and both prompt first.
- **No network calls** unless explicitly requested and approved as a feature. D-Bus on the session bus (single instance, tray) is local IPC.

## 4. Packaging Rules

- `VERSION` is the canonical version: the binary embeds it and `task deb` passes it to the package. A version change must also update the release metadata files listed in `docs/RELEASING.md` (changelogs, man page, AppStream, `build/config.yml`, `frontend/package*.json`); `task version:check` fails until they all match.
- The package installs only below `/usr`: `/usr/bin/projecthub`, `/usr/share/applications/io.github.semelyanov86.projecthub.desktop` (file id = GTK application id, needed for Wayland matching; `StartupWMClass=projecthub` for X11), `/usr/share/metainfo/…`, hicolor icons, man page, copyright and changelog. Never install into home directories, `/usr/local` or `/etc/skel`; configuration is created by the application at runtime.
- No maintainer scripts: dpkg triggers refresh the desktop database, icon cache and man index.
- `Depends` in `build/linux/nfpm/nfpm.yaml` must cover `dpkg-shlibdeps` of the binary built on Ubuntu 24.04; `build/linux/verify-deb.sh` fails otherwise. Confirm package names on Ubuntu 26.04 before changing them.
- Builds must stay reproducible: `-trimpath`, `npm ci`, `gzip -n`, `SOURCE_DATE_EPOCH`.
- Regenerate icons with `task icons` after changing `build/icons/*.svg`; the PNGs are committed.

## 5. Testing Requirements

- Write unit tests for all backend business logic, validation, and storage invariants (`go test -race ./...`).
- Write Vitest tests for frontend stores, composables, and essential UI interactions (`npm --prefix frontend run test`).
- Tray host detection is tested against a private `dbus-daemon` (skipped when it is not installed); never touch the developer's session bus in tests.
- Always run `task check` (version consistency, formatting check, `go vet`, golangci-lint, `vue-tsc`, `eslint`, Go tests, Vitest) after making changes.
- Run `task deb` after changing packaging, icons, `main.go` or Wails bindings, and `task deb:smoke` (Docker) when the change affects installation, startup, the tray or window behaviour.
- **Never claim tests or builds passed without actually running them after the final edit.** Distinguish container results from real-desktop results when reporting GUI verification.

## 6. Documentation Requirements

- Keep `README.md` up to date when prerequisites, installation, usage, settings or tasks change.
- Document any new configuration fields in `README.md` and `internal/config/`.
- Document any new `Taskfile.yml` commands in `README.md`.
- Update `CHANGELOG.md` for meaningful features, fixes, or breaking changes.
- Follow `docs/RELEASING.md` for releases; do not push tags or create GitHub releases unless explicitly asked.

## 7. Agent Workflow

Before making changes:
1. Read `AGENTS.md`.
2. Inspect the existing codebase and relevant tests.
3. Identify the smallest set of modules affected.
4. Implement the smallest clean, maintainable solution without rewriting functioning modules unnecessarily.
5. Do not create placeholder implementations presented as completed features.
6. Run `task format` (if needed) and `task check` (plus `task build`/`task deb` when changing Wails bindings, `main.go`, packaging or build assets).
7. Summarize exact files changed, architectural notes, and observed verification results.
