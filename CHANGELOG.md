# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-11-15

First public release.

### Added

- Project catalog: add, edit, remove and favorite local project directories with descriptions, per-project default editors and last-opened tracking. Removing a project never touches its files.
- All, Favorites and Recent tabs, search by name, description or path, sorting by name or last opened, keyboard navigation, Open with and context menus, dialogs and toast notifications.
- Launching projects in Zed and Sublime Text, in Ghostty (single-instance `+new-window` with an independent-window fallback) or a custom terminal with an argument template and six presets, and in the system file manager (`gio open`, then `xdg-open`) or a custom one. Executable detection with authoritative overrides in Settings.
- Read-only Git integration: branch or detached HEAD, tracked-file changes indicator, ahead/behind tooltip, per-project error badges, and missing-directory flags; refreshed on startup, on request, after catalog changes and when the window regains focus.
- Native folder drag & drop, JSON import with preview and merge/replace modes (replacement keeps a byte-exact backup), and JSON export.
- System tray icon (StatusNotifierItem) with Open ProjectHub, up to eight recently opened projects, Settings… and Quit. Closing the window keeps ProjectHub in the tray by default; the `closeBehavior` setting switches to quitting. Without a tray host (for example stock GNOME without the AppIndicator extension) closing the window always quits.
- Single instance: starting ProjectHub again shows and focuses the running window, including when it is hidden in the tray, and keeps its state.
- `projecthub --version` and `projecthub --help` work without a display.
- Debian package `projecthub_<version>_<arch>.deb` built with `task deb` (Wails 3 / nFPM): binary in `/usr/bin`, desktop entry `io.github.semelyanov86.projecthub.desktop`, AppStream metadata, hicolor icons from 16 px to 512 px plus SVG, man page, copyright and changelog. Runtime dependencies are checked against `dpkg-shlibdeps` on every build.
- Application icon: a project folder carrying a hub of linked nodes, with a simplified variant for 16–24 px and the tray.
- `task install` / `task uninstall` (apt with confirmation), `task deb:verify`, `task deb:smoke`, `task version:check`, `task icons`, an expanded `task doctor`, and GitHub Actions workflows for CI and tagged releases.
- Light, dark and system themes, remembered window size, and an optional always-on-top window.
- XDG-compliant, versioned JSON configuration (`settings.json`, `projects.json`, schema version 2) with lazy in-memory migration from schema 1.

### Security

- External applications are started with discrete arguments and never through a shell; project directories are validated before every launch, and child processes do not inherit ProjectHub's GTK/WebKit/Wails, activation-token or AppImage variables.
- Git inspection is read-only and bounded (four concurrent processes, 3-second timeout, output limits). It disables filesystem-monitor hooks, filter drivers defined by the inspected repository, all transports and lazy fetches, optional locks, rename detection and submodule inspection, and strips inherited `GIT_*` variables.
- File paths for import and export are chosen in native Go dialogs; the webview never supplies paths to read or write. Exports refuse to overwrite the active configuration files; imports are limited to 10 MiB and 10,000 projects.
- Configuration writes are atomic (temporary file, `fsync`, rename, directory `fsync`), and corrupted or unsupported configuration files are reported instead of being overwritten.

[Unreleased]: https://github.com/semelyanov86/ProjectHub/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/semelyanov86/ProjectHub/releases/tag/v1.0.0
