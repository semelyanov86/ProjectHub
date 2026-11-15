# ProjectHub

A lightweight, local-first Linux desktop project manager built with **Go**, **Wails 3**, and **Vue 3**.

ProjectHub helps developers organize local project directories and launch them quickly in their preferred code editor, Ghostty terminal, or system file manager—with zero databases, zero cloud accounts, and zero telemetry.

> **Status:** Stage 2 (Functional Application). Project registration, application launching, Git inspection, native drag & drop, and JSON import/export are implemented.

## Features

- Add, edit, remove, and favorite local project directories, with descriptions and per-project editors.
- Search by name, description, or path; browse All, Favorites, and Recent tabs; sort by name or last opened.
- Open projects in Sublime Text, Zed, Ghostty, a custom terminal, or the file manager.
- See Git branches, tracked-file changes, and missing directories in the project list.
- Use keyboard navigation, native folder drops, and JSON import/export with a preview and replacement backups.
- Choose light, dark, or system theme, remember window size, and optionally keep the window on top.
- Launch a second instance to focus the existing window.

## Target Environment

- **OS:** Ubuntu 24.04 LTS, Ubuntu 26.04 LTS
- **Desktop:** GNOME (Wayland and X11)
- **Architecture:** `linux/amd64` (`linux/arm64` planned for later releases)

## Tech Stack

- **Backend:** Go 1.26+, Wails 3 (`v3.1.0`), Go standard library
- **Frontend:** Vue 3, TypeScript (strict), Vite, Tailwind CSS v4, Pinia, Lucide icons (`@lucide/vue`)
- **Tooling:** Go Task (`Taskfile.yml`), ESLint, Prettier, Vitest, Go `testing`

## Prerequisites (Ubuntu 24.04+)

Install the required system libraries for Wails 3 (GTK4 + WebKitGTK 6.0):

```bash
sudo apt update
sudo apt install -y build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev
```

Install toolchain dependencies:

- Go 1.26+
- Node.js 22+ / npm
- [Task](https://taskfile.dev/) (`task`)
- [golangci-lint](https://golangci-lint.run/) v2 (for Go linting and `gofumpt`/`goimports` formatting)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) (optional, for `task audit`)
- Wails 3 CLI:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
```

Put `$(go env GOPATH)/bin` first on your `PATH` so the installed `wails3` and `golangci-lint` v2 binaries are selected:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Use a golangci-lint v2 build compatible with your installed Go toolchain.

## Getting Started

Check your environment:

```bash
task doctor
```

Run in development mode (hot-reloading Vue frontend + Wails desktop window):

```bash
task dev
```

Build the production binary (output in `bin/projecthub`):

```bash
task build
```

Build a `.deb` package (output in `bin/`):

```bash
task deb
```

## Taskfile Commands

| Command | Description |
| --- | --- |
| `task dev` | Run ProjectHub in Wails 3 development mode |
| `task prepare` | Generate Wails TypeScript bindings and build embedded `frontend/dist` |
| `task build` | Build the production desktop binary |
| `task deb` | Build a Debian/Ubuntu `.deb` package via nFPM |
| `task test` | Run Go unit tests (`-race`) and frontend Vitest tests |
| `task test:coverage` | Run Go and frontend unit tests with coverage output |
| `task lint` | Run `go vet`, `golangci-lint`, `vue-tsc --noEmit`, and ESLint |
| `task format` | Format Go (`gofumpt`/`goimports`) and frontend (`prettier` + Tailwind sorting) sources |
| `task audit` | Run `govulncheck` and `npm audit` security checks |
| `task install:desktop` | Register the locally built `bin/projecthub` in the desktop menu with its icon (current user, `~/.local/share`) |
| `task uninstall:desktop` | Remove the desktop entry and icon installed by `task install:desktop` |
| `task clean` | Remove build, coverage, and packaging output directories |
| `task doctor` | Check toolchain versions and run `wails3 doctor` |
| `task check` | Verify formatting, run all linters, and execute all unit tests |

## Usage

### Adding and managing projects

Click **Add**, choose an existing directory in the native folder picker, and review its suggested name, description, favorite flag, and default editor before saving. Names you enter must be a single line of at most 200 characters; descriptions allow up to 2000 characters (existing records from older versions are kept as they are). Paths must be absolute: `~` is not expanded. Duplicate detection compares cleaned paths; symlink aliases are distinct.

Use the tabs and search field to filter projects. The sort button switches All and Favorites between name and last-opened order; Favorites are pinned first in All; Recent shows opened projects with the newest first. Missing or inaccessible directories are flagged in the list. Removing a project requires confirmation and only unregisters it; its files stay on disk.

### Opening projects

Single-click a row to show **Open with**: Sublime Text, Zed, Ghostty (or the configured custom terminal), and File Manager. Unavailable applications are disabled with an explanation. Double-click opens the effective editor when **Double-click opens editor** is enabled (the default).

The effective editor is the project's explicit default, then its last used editor if **Remember last used editor** is enabled, then the global default (Zed). Successful launches update the last-opened time; editor launches also remember the editor when enabled.

Right-click or click a row's **⋯** for the context menu: open in the default editor, Open with…, Edit, toggle favorite, Copy path, Refresh status, or Remove.

### Keyboard shortcuts

List shortcuts apply while a project row has focus.

| Shortcut | Action |
| --- | --- |
| `Ctrl+F` | Focus search |
| `↑` / `↓` | Select the previous / next project |
| `Home` / `End` | Select the first / last project |
| `↓` in search | Move focus into the project list |
| `Enter` / `Space` | Show Open with |
| `Ctrl+Enter` | Open in the effective editor |
| `Shift+F10` / Menu key | Show the context menu |
| `F2` | Edit the selected project |
| `Delete` | Remove the selected project, with confirmation |
| `Esc` | Close a popup or dialog; clear search when focused there |

### Drag & drop

Drop one folder from the file manager onto the window to open a prefilled Add dialog. Drop several folders to register them immediately using directory names. Duplicate, missing, or non-directory entries are skipped and reported in a notification.

### Import and export

Choose **Import projects…** or **Export projects…** from the toolbar's **⋯** menu. Native dialogs select files; the import preview shows new, duplicate, and missing directories before anything changes.

- **Merge** (default) adds new projects, keeps existing ones, and skips duplicate paths and directories missing on this computer.
- **Replace** requires explicit confirmation. Before replacing an existing catalog, ProjectHub saves a byte-exact copy alongside it as `projects.json.bak-<UTC timestamp>` (for example, `projects.json.bak-20261115T120000Z`, with a numeric suffix on collisions). A failed backup aborts replacement. No backup is needed when no catalog exists.
- **Export** writes schema v2 JSON in the same format as `projects.json`. Export cannot target the active `settings.json` or `projects.json` files, including existing aliases to them.

Imports accept schema v1 and v2 documents with explicit `schemaVersion` and `projects` fields, limited to 10 MiB and 10,000 projects. Paths remain local absolute paths; import does not copy project files between computers.

## Supported applications & detection

| Application | CLI names / behavior |
| --- | --- |
| Sublime Text | `subl` (editor ID `sublime`) |
| Zed | `zed`, then `zeditor` (editor ID `zed`; global default) |
| Ghostty | `ghostty +new-window --working-directory=<dir>`; bounded to 4 seconds, with detached `ghostty --gtk-single-instance=false --working-directory=<dir>` fallback on failure |
| File Manager | Auto mode tries `gio open <dir>` (bounded to 5 seconds), then `xdg-open <dir>`; custom mode passes the directory to the configured executable |
| Custom terminal | Executable plus discrete argument template; each `{path}` occurrence is replaced with the project directory |

Detection checks CLI candidates on `PATH`, then probes `~/.local/bin`, `~/.local/zed.app/bin`, `~/bin`, `/usr/local/bin`, `/usr/bin`, `/opt/sublime_text`, and `/snap/bin`. Settings overrides are authoritative: an invalid configured executable does not fall back to another application. Use a bare executable name or an absolute path, without command arguments.

Custom terminal presets are available for Ptyxis, GNOME Terminal, kitty, Alacritty, WezTerm, and Konsole. For example, kitty uses executable `kitty` with arguments `["--directory", "{path}"]`. Settings accepts one argument per line. Substitution never splits arguments or invokes a shell, so spaces, `$`, and `;` in directory names remain literal. Child applications run with the project directory as their working directory and without ProjectHub's GTK/WebKit/Wails activation or AppImage runtime variables.

## Git integration

Project rows show the branch (or a short commit hash for detached HEAD), with an optional dirty dot for staged or unstaged **tracked** files. The branch tooltip includes an unborn-branch note and ahead/behind counts when available. Untracked files and submodule changes are excluded; there is no fetch or remote refresh, so ahead/behind counts reflect local knowledge. Git is optional: if it is missing, project management and launching remain available.

Inspections run in batches on startup, after catalog changes, on explicit refresh, and when the window regains focus after the last full refresh is at least 60 seconds old. There is no polling. At most four Git processes run concurrently, each with a 3-second timeout and bounded output capture.

The read-only inspection command is:

```text
git --no-optional-locks -c core.fsmonitor=false -c safe.bareRepository=explicit -c color.ui=false -C <dir> status --porcelain=v2 --branch --untracked-files=no --no-renames --ignore-submodules=all
```

`core.fsmonitor=false` disables repository-configured filesystem-monitor hooks. Before `git status`, ProjectHub lists configured filter drivers with their scope (`git config --show-scope --name-only --get-regexp '^filter\.'`, which never runs commands). Drivers defined by the repository itself (local/worktree config) are disabled for the status call through `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_n`/`GIT_CONFIG_VALUE_n`, so a repository's own `.git/config` cannot make status execute a clean/process filter; drivers from your global or system config (for example a globally installed Git LFS) stay enabled because your own Git commands run them anyway. When a repository-defined driver was disabled and the tree looks modified, the dirty indicator is shown as a hollow ring ("may be inaccurate"), because files those filters convert can then compare as changed. Inspection errors (timeouts, dubious ownership, …) appear as a **Git error** badge with the reason in its tooltip. Inherited `GIT_*` variables are stripped, then inspection sets `GIT_OPTIONAL_LOCKS=0`, `GIT_TERMINAL_PROMPT=0`, `GIT_ALLOW_PROTOCOL=` (no transport may be used, so partial clones never lazily fetch missing objects), `GIT_NO_LAZY_FETCH=1`, and `GIT_CEILING_DIRECTORIES` to the user's home directory. Output uses `LC_ALL=C`. Inspection errors or timeouts are reported without modifying repository contents. Known limitation: for repositories that use a split index (`core.splitIndex`), Git itself refreshes the modification time of `.git/sharedindex.*` while reading it; file contents are not changed.

## Configuration

ProjectHub stores versioned JSON under `$XDG_CONFIG_HOME/projecthub/` (default `~/.config/projecthub/`). Both files use `schemaVersion: 2`.

### settings.json

The complete field list and fresh-install defaults are:

| Field | Default | Meaning |
| --- | --- | --- |
| `schemaVersion` | `2` | Settings schema version |
| `defaultEditor` | `"zed"` | Global editor: `zed` or `sublime` |
| `preferredTerminal` | `"ghostty"` | Terminal menu target: `ghostty` or `custom` |
| `preferredFileManager` | `"auto"` | File manager mode: `auto` or `custom` |
| `theme` | `"system"` | `system`, `light`, or `dark` |
| `window.width` | `440` | Window width in pixels |
| `window.height` | `650` | Window height in pixels |
| `window.rememberSize` | `true` | Restore saved dimensions |
| `window.alwaysOnTop` | `false` | Keep the window above others |
| `executables.sublime` | `""` | Sublime executable override; empty enables detection |
| `executables.zed` | `""` | Zed executable override; empty enables detection |
| `executables.ghostty` | `""` | Ghostty executable override; empty enables detection |
| `executables.fileManager` | `""` | Executable used in custom file manager mode |
| `customTerminal.executable` | `""` | Custom terminal executable |
| `customTerminal.args` | `[]` | Argument array with optional `{path}` substitutions |
| `rememberLastEditor` | `true` | Remember and use each project's last used editor |
| `doubleClickOpensEditor` | `true` | Double-click opens the effective editor |
| `showGitDirty` | `true` | Display the tracked-file dirty indicator |
| `projectSort` | `"name"` | All/Favorites order: `name` or `lastOpened` |

### projects.json

The document contains `schemaVersion` and a `projects` array (initially empty). Each project stores:

| Field | Meaning |
| --- | --- |
| `id` | Unique project identifier |
| `name` | Single-line display name, at most 200 characters |
| `description` | Optional description text, at most 2000 characters |
| `path` | Absolute directory path |
| `favorite` | Favorite flag |
| `defaultEditor` | `sublime` or `zed`; empty inherits the effective-editor rules |
| `lastUsedEditor` | Last remembered editor; omitted when empty |
| `lastOpened` | Last successful launch time; omitted until first launch |
| `createdAt` | Creation timestamp |
| `updatedAt` | Metadata update timestamp |

Schema v1 files migrate **in memory** when loaded; loading never writes them. The next save of that file writes v2. Unsupported global editor IDs become `zed`; unsupported per-project editor references become empty (inherit) in any schema version. Legacy free-form terminal and file manager values become custom executables (extra words become terminal arguments, never shell-interpreted); values that cannot be an executable fall back to Ghostty / the system file manager. Stored records are validated structurally only, so older entries with long or multi-line names or descriptions keep loading, editing and exporting; the 200/2000-character single-line limits apply to text you enter or change. Stage 1 builds cannot read v2 files; retain backups before reverting to an older build.

Catalog mutations use locked read-modify-write transactions. JSON writes are atomic (temporary file in the same directory → `Sync` → `Close` → `Rename`). Corrupted or unsupported-schema configuration is preserved and reported as an error rather than overwritten. Replace imports additionally create the catalog backup described above.

## Troubleshooting

- **No icon in the dock / app menu when running `bin/projecthub` directly:** GTK4 cannot set a window icon itself; GNOME takes it from an installed desktop entry whose id matches the application id `io.github.semelyanov86.projecthub` (Wayland) or `StartupWMClass=projecthub` (X11). Run `task install:desktop` once (or install the `.deb`), then restart ProjectHub.
- **Ghostty opens incorrectly or D-Bus activation fails:** `+new-window` requires Ghostty 1.2 or later; forwarding `--working-directory` requires 1.3 or later. See the upstream [1.2 release notes](https://ghostty.org/docs/install/release-notes/1-2-0) and [1.3 release notes](https://ghostty.org/docs/install/release-notes/1-3-0). ProjectHub tries the activation command first, then starts an independent window with `--gtk-single-instance=false` if it fails or times out. Older versions that accept the command but ignore the directory may need upgrading to 1.3+.
- **An installed application is unavailable from a desktop session:** the session may omit `~/.local/bin` from `PATH`. ProjectHub probes it automatically along with the locations above. Set an explicit executable path in Settings if detection still fails, and check any existing override because it takes precedence.
- **settings.json or projects.json cannot be loaded:** malformed JSON and unsupported schemas are never overwritten, including during replacement import. Keep a copy of the affected file, correct it or restore a known-good backup, then retry. Settings saves are blocked while existing settings cannot be loaded.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE)
