# Releasing ProjectHub

Releases are built and published by the [release workflow](../.github/workflows/release.yml) when a tag `v<version>` is pushed. It uses the repository's `GITHUB_TOKEN`; no personal access token is needed.

## Version

The `VERSION` file is the single source of the version. The application embeds it (`projecthub --version`, the footer of the window), and `task deb` passes it to the package. These files record the version too and must match — `task version:check` verifies them:

| File | Field |
| --- | --- |
| `VERSION` | the version, e.g. `1.0.0` |
| `CHANGELOG.md` | newest `## [x.y.z] - YYYY-MM-DD` section |
| `build/config.yml` | `info.version` |
| `frontend/package.json`, `frontend/package-lock.json` | `version` |
| `build/linux/share/doc/projecthub/changelog` | Debian changelog entry `projecthub (x.y.z)` |
| `build/linux/share/man/man1/projecthub.1` | `.TH` header |
| `build/linux/share/metainfo/io.github.semelyanov86.projecthub.metainfo.xml` | newest `<release>` |

## Checklist

1. Make sure `main` is green in CI.
2. Update the version in all files listed above and move the *Unreleased* entries in `CHANGELOG.md` into a new version section (add the compare link at the bottom).
3. Run locally:

   ```bash
   task version:check
   task check
   task deb
   task deb:smoke      # optional, needs Docker
   ```

4. Commit the release (`chore: release vX.Y.Z`) and merge it to `main`.
5. Tag and push:

   ```bash
   git tag -a vX.Y.Z -m "ProjectHub X.Y.Z"
   git push origin vX.Y.Z
   ```

## What the workflow does

1. **build** — checks that the tag equals `v$(cat VERSION)` and that all version fields match, runs `task check`, builds the production binary and `dist/projecthub_X.Y.Z_amd64.deb` on Ubuntu 24.04 with `task deb` (which verifies metadata, contents, permissions and `dpkg-shlibdeps` dependencies), prints `dpkg-deb --info/--contents`, and writes `SHA256SUMS`.
2. **smoke** — installs the package with apt in clean `ubuntu:24.04` and `ubuntu:26.04` containers and runs the GUI smoke tests (X11, Wayland, tray menu, single instance, close behaviour, removal).
3. **publish** — creates the GitHub release with the notes from `CHANGELOG.md` and attaches the `.deb` and `SHA256SUMS`. Versions with a pre-release suffix (`1.0.0-rc.1`) are marked as pre-releases.

If a step fails, nothing is published. Fix the problem, delete the tag (`git push --delete origin vX.Y.Z` and `git tag -d vX.Y.Z`), and tag again.

## Reproducibility

Package contents are reproducible for a given commit and toolchain: the Go binary is built with `-trimpath`, frontend dependencies are installed with `npm ci`, documentation is compressed with `gzip -n`, and file timestamps come from `SOURCE_DATE_EPOCH` (the commit time). CI pins the Go minor version, Node.js major version, Task, Wails CLI and golangci-lint versions, and GitHub Actions by commit SHA.
