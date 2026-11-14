# ProjectHub

A lightweight, local-first Linux desktop project manager built with **Go**, **Wails 3**, and **Vue 3**.

ProjectHub helps developers organize local project directories and launch them quickly in their preferred code editor, Ghostty terminal, or system file manager—with zero databases, zero cloud accounts, and zero telemetry.

> **Status:** Stage 1 (Architecture & Foundation). Core configuration storage, Wails 3 desktop shell, theme persistence, and build/packaging tooling are in place. Project registration, editor/terminal launching, and Git status inspection will follow in subsequent stages.

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

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

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
| `task clean` | Remove build, coverage, and packaging output directories |
| `task doctor` | Check toolchain versions and run `wails3 doctor` |
| `task check` | Verify formatting, run all linters, and execute all unit tests |

## Configuration

ProjectHub stores configuration as versioned JSON files under `$XDG_CONFIG_HOME/projecthub/` (defaulting to `~/.config/projecthub/`):

- `settings.json` — User preferences (`schemaVersion`, `defaultEditor`, `preferredTerminal`, `preferredFileManager`, `theme`, `window`)
- `projects.json` — Registered project metadata (`schemaVersion`, `projects[]` with `id`, `name`, `description`, `path`, `favorite`, `defaultEditor`, `lastOpened`, `createdAt`, `updatedAt`)

All writes are atomic (`CreateTemp` + `Sync` + `Rename`), and corrupted JSON files are preserved untouched while surfacing an explicit error rather than silently overwriting user data.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE)
