# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
