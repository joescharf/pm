# Release Infrastructure: Binaries, Docker, Homebrew, GitHub Pages

*2026-02-13T06:19:26Z*

Built a complete release pipeline for pm: CI testing, GoReleaser binaries+Docker+Homebrew, and GitHub Pages docs deployment. All patterns drawn from the fdsn reference implementation and validated locally.

## 1. GoReleaser Configuration (.goreleaser.yml)

Rewrote from scratch. Key fixes: single cross-platform build matrix (linux/darwin/windows × amd64/arm64), correct ldflags targeting `main.*` instead of `cmd.*`, `-s -w` strip flags, UI build via before hooks, `homebrew_casks:` and `dockers_v2:` (non-deprecated goreleaser v2 keys), removed broken `universal_binaries` and `release.draft`.

```bash
cat /Users/joescharf/app/pm/.goreleaser.yml
```

```output
version: 2

before:
  hooks:
    - sh -c "cd ui && bun install --frozen-lockfile"
    - sh -c "cd ui && bun run build"
    - sh -c "rm -rf internal/ui/dist/assets && cp -r ui/dist/* internal/ui/dist/"

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - README.md

checksum:
  name_template: "checksums.txt"

snapshot:
  version_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  groups:
    - title: Features
      regexp: "^.*feat[(\\w)]*:+.*$"
      order: 0
    - title: "Bug fixes"
      regexp: "^.*fix[(\\w)]*:+.*$"
      order: 1
    - title: Others
      order: 999
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
      - "^chore:"

homebrew_casks:
  - name: pm
    repository:
      owner: joescharf
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Casks
    homepage: https://github.com/joescharf/pm
    description: "Project manager CLI — track projects, issues, and agent sessions from a single binary."

dockers_v2:
  - images:
      - "ghcr.io/joescharf/pm"
    tags:
      - "v{{ .Version }}"
      - latest
```

Validate the config passes goreleaser's built-in linter with no warnings:

```bash
cd /Users/joescharf/app/pm && goreleaser check 2>&1
```

```output
  • checking                                  path=.goreleaser.yml
  • 1 configuration file(s) validated
  • thanks for using GoReleaser!
```

## 2. Dockerfile

Hardened: pinned Alpine 3.21, non-root `pm` user, `tzdata` for timezone support, `/data` directory for SQLite, `TARGETPLATFORM` ARG for goreleaser `dockers_v2` compatibility.

```bash
cat /Users/joescharf/app/pm/Dockerfile
```

```output
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S pm && adduser -S pm -G pm
RUN mkdir -p /data && chown pm:pm /data
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/pm /usr/local/bin/pm
USER pm
ENV PM_DB_PATH=/data/pm.db
EXPOSE 8080
ENTRYPOINT ["pm"]
CMD ["serve"]
```

## 3. GitHub Actions Workflows

Created three workflows in `.github/workflows/`:

- **ci.yml** — Runs on push to main and PRs. Two parallel jobs: `test` (go test + vet) and `lint` (golangci-lint). Both build and embed the UI first so the `//go:embed` directive resolves.
- **release.yml** — Runs on `v*` tag push. Sets up Go, Bun, QEMU, Docker Buildx, logs into GHCR, then runs GoReleaser (whose before hooks handle the UI build).
- **docs.yml** — Runs on `docs/**` changes to main or manual dispatch. Builds MkDocs via uv, uploads pages artifact, deploys to GitHub Pages.

```bash
ls -1 /Users/joescharf/app/pm/.github/workflows/
```

```output
ci.yml
docs.yml
release.yml
```

```bash
cat /Users/joescharf/app/pm/.github/workflows/ci.yml
```

```output
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: oven-sh/setup-bun@v2

      - name: Install UI dependencies
        run: cd ui && bun install --frozen-lockfile

      - name: Build UI
        run: cd ui && bun run build

      - name: Embed UI
        run: rm -rf internal/ui/dist/assets && cp -r ui/dist/* internal/ui/dist/

      - name: Run tests
        run: go test -v -race -count=1 ./...

      - name: Run vet
        run: go vet ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: oven-sh/setup-bun@v2

      - name: Install UI dependencies
        run: cd ui && bun install --frozen-lockfile

      - name: Build UI
        run: cd ui && bun run build

      - name: Embed UI
        run: rm -rf internal/ui/dist/assets && cp -r ui/dist/* internal/ui/dist/

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: latest
```

```bash
cat /Users/joescharf/app/pm/.github/workflows/release.yml
```

```output
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write
  packages: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: oven-sh/setup-bun@v2

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

```bash
cat /Users/joescharf/app/pm/.github/workflows/docs.yml
```

```output
name: Docs

on:
  push:
    branches: [main]
    paths:
      - "docs/**"
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: true

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: astral-sh/setup-uv@v5

      - name: Install dependencies
        run: cd docs && uv sync

      - name: Build docs
        run: cd docs && uv run mkdocs build

      - uses: actions/upload-pages-artifact@v3
        with:
          path: docs/site

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - id: deployment
        uses: actions/deploy-pages@v4
```

## 4. MkDocs + Docs .gitignore Updates

Updated `docs/mkdocs.yml`: `site_url` now points to GitHub Pages (`https://joescharf.github.io/pm/`), added `repo_url` and `repo_name` for header link, `edit_uri` enables "Edit on GitHub" links. Simplified `docs/.gitignore` to match fdsn pattern.

```bash
head -6 /Users/joescharf/app/pm/docs/mkdocs.yml
```

```output
site_name: pm Documentation
site_url: https://joescharf.github.io/pm/
site_description: "pm documentation"
repo_url: https://github.com/joescharf/pm
repo_name: joescharf/pm
edit_uri: edit/main/docs/docs/
```

## 5. Snapshot Build Verification

Run a full snapshot build to prove all 6 platform binaries compile and package correctly:

```bash
cd /Users/joescharf/app/pm && goreleaser release --snapshot --clean --skip docker,homebrew 2>&1
```

```output
  • skipping announce, docker, homebrew, publish, and validate...
  • cleaning distribution directory
  • loading environment variables
    • using token from /Users/joescharf/.config/goreleaser/github_token
  • getting and validating git state
    • ignoring errors because this is a snapshot     error=git doesn't contain any tags - either add a tag or use --snapshot
    • git state                                      commit=ec94b1f3902d6bf16c5fdf4a70ec38452b49103b branch=main current_tag=v0.0.0 previous_tag=<unknown> dirty=false
    • pipe skipped or partially skipped              reason=disabled during snapshot mode
  • parsing tag
  • setting defaults
  • snapshotting
    • building snapshot...                           version=0.0.1-next
  • running before hooks
    • running                                        hook=sh -c "cd ui && bun install --frozen-lockfile"
    • running                                        hook=sh -c "cd ui && bun run build"
    • running                                        hook=sh -c "rm -rf internal/ui/dist/assets && cp -r ui/dist/* internal/ui/dist/"
  • ensuring distribution directory
  • setting up metadata
  • writing release metadata
  • loading go mod information
  • build prerequisites
  • building binaries
    • building                                       binary=dist/pm_windows_amd64_v1/pm.exe
    • building                                       binary=dist/pm_darwin_arm64_v8.0/pm
    • building                                       binary=dist/pm_linux_arm64_v8.0/pm
    • building                                       binary=dist/pm_linux_amd64_v1/pm
    • building                                       binary=dist/pm_windows_arm64_v8.0/pm.exe
    • building                                       binary=dist/pm_darwin_amd64_v1/pm
  • archives
    • archiving                                      name=dist/pm_0.0.1-next_darwin_arm64.tar.gz
    • archiving                                      name=dist/pm_0.0.1-next_windows_amd64.zip
    • archiving                                      name=dist/pm_0.0.1-next_windows_arm64.zip
    • archiving                                      name=dist/pm_0.0.1-next_linux_arm64.tar.gz
    • archiving                                      name=dist/pm_0.0.1-next_linux_amd64.tar.gz
    • archiving                                      name=dist/pm_0.0.1-next_darwin_amd64.tar.gz
  • calculating checksums
  • writing artifacts metadata
  • release succeeded after 4s
  • thanks for using GoReleaser!
```

List the artifacts produced:

```bash
ls -1 /Users/joescharf/app/pm/dist/*.{tar.gz,zip,txt} 2>/dev/null | xargs -I{} basename {}
```

```output
checksums.txt
pm_0.0.1-next_darwin_amd64.tar.gz
pm_0.0.1-next_darwin_arm64.tar.gz
pm_0.0.1-next_linux_amd64.tar.gz
pm_0.0.1-next_linux_arm64.tar.gz
pm_0.0.1-next_windows_amd64.zip
pm_0.0.1-next_windows_arm64.zip
```

Verify the built binary reports the snapshot version:

```bash
/Users/joescharf/app/pm/dist/pm_darwin_arm64_v8.0/pm version 2>&1
```

```output
pm version 0.0.1-next (commit: ec94b1f3902d6bf16c5fdf4a70ec38452b49103b, built: 2026-02-13T06:20:35Z)
```

## 6. Docs Build Verification

```bash
cd /Users/joescharf/app/pm/docs && uv run mkdocs build 2>&1
```

```output
INFO    -  Cleaning site directory
INFO    -  Building documentation to directory: /Users/joescharf/app/pm/docs/site
INFO    -  Documentation built in 1.17 seconds
```

## 7. gsi Template Spec

Created comprehensive spec at `/Users/joescharf/app/gsi/docs/docs/release-infrastructure.md` documenting all template changes needed to update gsi's scaffolding. Tracked as gsi issue 01KHATCYWG (high priority).

## Manual Setup Required

Before the first release, configure in the GitHub repo settings:
1. **HOMEBREW_TAP_TOKEN** secret — GitHub PAT with `repo` scope for `joescharf/homebrew-tap`
2. **GitHub Pages** source — set to "GitHub Actions"
3. **GHCR visibility** — check after first release at `ghcr.io/joescharf/pm`
