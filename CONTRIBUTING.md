# Contributing to ProjectHub

Thank you for your interest in improving ProjectHub! Bug reports, ideas and pull requests are welcome.

## Reporting bugs and requesting features

Open an issue on [GitHub](https://github.com/semelyanov86/ProjectHub/issues) and include:

- your Ubuntu version, desktop session (Wayland or X11) and how you installed ProjectHub (`.deb` or source);
- the output of `projecthub --version`;
- the steps to reproduce, and what you expected instead;
- relevant terminal output from starting `projecthub` in a terminal.

Please do not attach your `projects.json` if it contains private paths; a minimal example is enough.

## Development setup

1. Install the prerequisites listed in [README.md › Development](README.md#development).
2. Check your environment:

   ```bash
   task doctor
   ```

3. Run the application with hot reload:

   ```bash
   task dev
   ```

## Making changes

- Read [AGENTS.md](AGENTS.md): it describes the architecture and the rules every change must follow (for example, never building shell commands from paths, keeping file access in Go, and atomic configuration writes).
- Keep Go packages small and covered by unit tests; keep Vue components focused, using `<script setup lang="ts">` and strict TypeScript.
- Tests must not start real desktop applications — use the fake process runner.
- Format your code with `task format`.
- Run the full verification suite before opening a pull request:

  ```bash
  task check
  ```

- If you touch packaging (`build/linux/`, `Taskfile.yml`, `main.go`, icons), also run `task deb` and, if Docker is available, `task deb:smoke`.
- Update [CHANGELOG.md](CHANGELOG.md) (under *Unreleased*) for user-visible changes, and [README.md](README.md) when you change behaviour, settings or tasks.

## Pull requests

- Keep each pull request focused on one change and describe what it does and why.
- Use [Conventional Commits](https://www.conventionalcommits.org/) style messages (`feat:`, `fix:`, `docs:` …).
- CI runs `task check`, builds the `.deb` and installs it in clean Ubuntu 24.04 and 26.04 containers; all jobs must pass.

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
