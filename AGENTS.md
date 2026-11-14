# AGENTS.md

Primary engineering instructions for AI coding agents (Claude Code, OpenAI Codex, Gemini / Antigravity) working on **ProjectHub**.

## 1. Project Context

- **Name:** ProjectHub
- **Shared-Memory Project ID:** `ProjectHub` (use `Project:ProjectHub`, `Service:ProjectHub:<service>`, `Module:ProjectHub:<module>`, `Decision:ProjectHub:<decision>`, `Integration:ProjectHub:<integration>`, `Bug:ProjectHub:<bug>` in the `shared-memory` MCP knowledge graph)
- **Purpose:** Lightweight, local-first Linux desktop project manager inspired by the Projects view of JetBrains Toolbox, with an independent visual identity.
- **Target Platforms:**
  - Ubuntu 24.04 LTS & Ubuntu 26.04 LTS
  - GNOME desktop environment (Wayland and X11)
  - Primary architecture: `linux/amd64` (`linux/arm64` optional in future releases)
- **Supported Editors & Tools (target integrations):**
  - Editors/IDEs: VS Code, Cursor, Zed, GoLand, IntelliJ IDEA, WebStorm, PyCharm, Sublime Text, Neovim
  - Terminal: Ghostty
  - System file manager (`xdg-open` / user-configured file manager)
  - Local Git repository inspection
- **Technology Stack:**
  - **Backend:** Go 1.26+, Wails 3 (`github.com/wailsapp/wails/v3` v3.1.0), standard Go libraries wherever practical
  - **Frontend:** Vue 3 (`<script setup lang="ts">`), strict TypeScript, Vite, Tailwind CSS v4, Pinia, `@lucide/vue`
  - **Tooling:** Go Task (`Taskfile.yml`), Git, `golangci-lint` v2 (`.golangci.yml`, `gofumpt`, `goimports`, `govet`, `gosec`, `staticcheck`), `govulncheck`, ESLint, Prettier (`prettier-plugin-tailwindcss`), Vitest (`@vitest/coverage-v8`), Go testing (`go test`)
  - **Storage:** Versioned JSON files following the XDG Base Directory Specification (`~/.config/projecthub/projects.json` and `~/.config/projecthub/settings.json`). No database, no cloud sync, no authentication, no telemetry.

### Architectural Overview

```text
ProjectHub/
├── main.go                    # Wails 3 app bootstrap, GTK4/WebKitGTK6 window, service registration
├── Taskfile.yml               # Unified Go Task entrypoints integrated with Wails 3 build tasks
├── build/                     # Wails 3 build config, icons, Linux desktop & nFPM (.deb) assets
├── internal/
│   ├── app/                   # Application metadata service exposed via Wails bindings
│   ├── config/                # XDG path resolution, versioned schemas, atomic JSON store, service
│   ├── project/               # Project domain model, validation, catalog service
│   ├── editor/                # Supported editor catalog and lookup helpers
│   ├── launcher/              # Path & executable security validation for external launches
│   └── git/                   # Git repository state models
└── frontend/
    └── src/
        ├── assets/            # Global CSS, Tailwind v4 import, light/dark theme tokens
        ├── components/        # Focused UI components and reusable ui/ primitives
        ├── composables/       # Reactive composables (e.g. theme resolution)
        ├── services/          # Typed wrapper around generated Wails 3 bindings
        ├── stores/            # Pinia stores
        ├── types/             # Shared TypeScript types
        └── views/             # Top-level views
```

- **Backend responsibilities:** Filesystem operations, XDG JSON persistence, validation, spawning external apps safely, Git inspection.
- **Frontend responsibilities:** UI rendering, filtering, Pinia state management, user interactions, theme switching.
- **Wails 3 bindings:** Generated TypeScript bindings in `frontend/bindings/` (gitignored, regenerated during build via `wails3 generate bindings -ts -i`). Do not edit files in `frontend/bindings/` manually.

## 2. Development Conventions

- **Idiomatic Go:** Standard formatting (`gofumpt` / `goimports` via `golangci-lint fmt`), small single-purpose packages, explicit error wrapping (`fmt.Errorf("...: %w", err)`), sentinel errors where callers branch on failure kinds (`ErrCorruptedConfig`, `ErrInvalidConfig`, etc.). All Go changes must pass `.golangci.yml` checks (`golangci-lint run . ./internal/...`).
- **Strict TypeScript:** Keep `strict: true`, `noUnusedLocals: true`, `noUnusedParameters: true`, and `noImplicitAny: true`. Avoid `any`.
- **Vue Composition API:** Always use `<script setup lang="ts">`. Keep components small, focused, and easy to test.
- **Separation of Concerns:** Never place OS/filesystem/process execution logic in the frontend, and never embed UI state concerns in backend packages.
- **No Overengineering:** Avoid unnecessary repository abstractions, redundant interfaces, or extra dependencies when Go stdlib or Vue/Pinia primitives suffice.
- **Configuration Safety:**
  - All JSON writes must be atomic (write to temp file in same directory -> `Sync` -> `Close` -> `Rename`).
  - Never silently overwrite corrupted JSON files; surface descriptive errors and preserve user data on disk.
  - Maintain versioned schemas (`schemaVersion`) and preserve existing user configuration during upgrades.

## 3. Security Requirements

- **Never construct shell commands (`sh -c`, `bash -c`) from project paths or user metadata.**
- **Always use `exec.Command(binary, arg1, arg2, ...)` with separate arguments.**
- **Validate project paths before launching any external application** (reject empty paths, NUL bytes, relative paths, and non-directory targets using `launcher.ValidateProjectDir`).
- **Handle paths containing spaces and special characters safely** by passing raw validated paths as discrete OS arguments, never shell-quoted strings.
- **Never execute arbitrary scripts or hooks from project metadata.**
- **No `sudo` or elevated privileges at runtime.**
- **No network calls** unless explicitly requested and approved as a feature.

## 4. Testing Requirements

- Write unit tests for all backend business logic, validation, and storage invariants (`go test -race ./...`).
- Write Vitest tests for frontend stores, composables, and essential UI interactions (`npm --prefix frontend run test`).
- Always run `task check` (formatting check, `go vet`, `vue-tsc`, `eslint`, Go tests, Vitest) after making changes.
- **Never claim tests or builds passed without actually running them after the final edit.**

## 5. Documentation Requirements

- Keep `README.md` up to date when prerequisites, installation, or usage change.
- Document any new configuration fields in `README.md` and `internal/config/`.
- Document any new `Taskfile.yml` commands in `README.md`.
- Update `CHANGELOG.md` for meaningful features, fixes, or breaking changes.

## 6. Agent Workflow

Before making changes:
1. Read `AGENTS.md`.
2. Inspect the existing codebase and relevant tests.
3. Identify the smallest set of modules affected.
4. Implement the smallest clean, maintainable solution without rewriting functioning modules unnecessarily.
5. Do not create placeholder implementations presented as completed features.
6. Run `task format` (if needed) and `task check` (plus `task build` when changing Wails bindings, `main.go`, or build assets).
7. Summarize exact files changed, architectural notes, and observed verification results.
