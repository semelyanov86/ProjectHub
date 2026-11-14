# Contributing to ProjectHub

Thank you for your interest in contributing to ProjectHub.

## Development Setup

1. Ensure your system meets the prerequisites listed in [README.md](README.md).
2. Verify your environment:

   ```bash
   task doctor
   ```

3. Run the verification suite before and after your changes:

   ```bash
   task check
   ```

## Guidelines

- Keep backend Go packages focused, idiomatic, and covered by unit tests.
- Keep Vue components small (`<script setup lang="ts">`) with strict TypeScript types.
- Follow the security rules in [AGENTS.md](AGENTS.md): never build shell commands from project paths or metadata; always pass separate arguments to `exec.Command` after validating directory paths.
- Format code with `task format` and verify all checks pass with `task check` before opening a pull request.
- Update `CHANGELOG.md` and `README.md` when introducing user-facing changes or new configuration options.
