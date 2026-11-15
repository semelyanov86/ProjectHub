# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Stage 2 functional project catalog: add, edit, remove, favorites, descriptions, per-project editor defaults, and last-opened tracking with transactional storage.
- All/Favorites/Recent tabs, name/last-opened sorting, search, keyboard navigation, Open with and context menus, dialogs, and toast notifications.
- Sublime Text and Zed launching, executable detection and authoritative overrides, Ghostty activation with an independent-window fallback, automatic/custom file managers, and custom terminal argument templates with six presets.
- Bounded local Git inspection with branch/detached HEAD display, tracked-file dirty indicator, ahead/behind tooltips, missing-directory flags, and refresh on startup, explicit request, catalog changes, or stale window focus.
- Native folder drag & drop, JSON import preview with merge/replace modes, byte-exact catalog backup before replacement, and schema v2 export.
- ID-based `desktop.Service`, native GTK dialogs, typed `projects:dropped` event, and the `internal/process` runner abstraction for bounded commands and detached launches with background reaping.
- Single-instance window focusing and application home-directory metadata.

- Initial ProjectHub repository foundation using Go 1.26, Wails 3 (`v3.1.0`), Vue 3, TypeScript, Tailwind CSS v4, Pinia, and `@lucide/vue`.
- Comprehensive `AGENTS.md` documenting architecture, conventions, security rules, and agent workflow.
- XDG-compliant versioned JSON configuration storage (`settings.json` and `projects.json`) with atomic writes, corrupted JSON preservation, and validation (`internal/config`).
- Core domain models and foundational validation for projects (`internal/project`), editors (`internal/editor`), launcher path/executable checks (`internal/launcher`), Git metadata (`internal/git`), and app info (`internal/app`).
- Compact desktop UI shell (~440px wide) with light/dark/system themes, search filter shell, welcome/empty state, and persistent preferences view.
- Unified `Taskfile.yml` integrating Wails 3 build and `.deb` packaging with `prepare`, `test`, `test:coverage`, `lint`, `format`, `audit`, `clean`, `doctor`, and `check`.
- Atomic config write guards in `internal/config` preventing overwriting corrupted or unsupported-schema JSON files.
- Independent `Promise.allSettled` initialization in `frontend/src/stores/app.ts` so project load errors preserve loaded user settings and block accidental defaults overwrite.
- Persisted `window.rememberSize` / `window.alwaysOnTop` lifecycle handling in `main.go` and `internal/config`.
- Asynchronous save confirmation in `SettingsView.vue` and `task prepare` dependency in `Taskfile.yml` so `task check` succeeds from a clean state.
- Code quality tooling: `.golangci.yml` (v2 with `gofumpt`, `goimports`, `govet`, `staticcheck`, `gosec`, `errorlint`, `revive`), `govulncheck`, ESLint (`eslint-plugin-vue`, `typescript-eslint`), Prettier (`prettier-plugin-tailwindcss`), and Vitest v8 coverage.
- Open-source documentation (`README.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `.editorconfig`, `.gitignore`).

### Changed

- Settings and projects now use schema v2, with lazy in-memory migration from v1 and v2 output on the next save. Stage 1 builds cannot read v2 files.
- The global default editor is now Zed; unsupported legacy editor references migrate to Zed globally or inherit per project.
- Default window dimensions are now 440×650; native file drops are enabled.
- Settings now include executable overrides, custom terminal arguments, remembered editor and double-click behavior, Git dirty visibility, and project sorting. Settings saves are serialized.
- Frontend backend calls now go through `frontend/src/services/backend.ts` to `desktop.Service`, `config.Service`, and `app.Service`.

### Removed

- VS Code, Cursor, GoLand, IntelliJ IDEA, WebStorm, PyCharm, and Neovim entries from the editor catalog; supported editors are Sublime Text and Zed.
- The old `SettingsView`, replaced by `SettingsDialog`.

### Fixed

- Desktop integration: the desktop entry is now installed as `io.github.semelyanov86.projecthub.desktop` (matching the GTK application id) from the tracked `build/linux/desktop` template with `StartupWMClass=projecthub`, and the 256×256 icon goes to `hicolor/256x256`, so GNOME shows the ProjectHub icon on Wayland and X11. New `task install:desktop` / `task uninstall:desktop` register a locally built binary for the current user.

### Security

- External commands use discrete arguments without a shell; launch directories are validated and child environments strip ProjectHub's GTK/WebKit/Wails, activation-token, and AppImage runtime variables.
- Git inspection disables filesystem-monitor hooks (`core.fsmonitor=false`), every filter driver defined by the inspected repository's own config (so repository-defined clean/process filters never run; global/system drivers such as Git LFS stay enabled and dirty results that may be affected are marked uncertain), all transports (`GIT_ALLOW_PROTOCOL=`, `GIT_NO_LAZY_FETCH=1`, preventing lazy fetches in partial clones), optional locks, rename detection, and submodule inspection; inherited `GIT_*` variables are removed and runtime/output limits apply.
- Native Go dialogs select import/export paths; the webview applies prepared imports by token rather than supplying file paths for reads or writes.
- Exports refuse to target active settings/catalog files or existing aliases; imports are limited to 10 MiB and 10,000 projects.
- Replacement imports require explicit confirmation and a successful backup of any existing catalog. Corrupted or unsupported-schema settings/catalog files remain protected from overwrite.
