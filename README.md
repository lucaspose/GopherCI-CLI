# GopherCI CLI

[![CI](https://github.com/lucaspose/GopherCI-CLI/actions/workflows/ci.yml/badge.svg)](https://github.com/lucaspose/GopherCI-CLI/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-AGPL--3.0-blue)

An interactive terminal client for [GopherCI](https://github.com/lucaspose/GopherCI), a minimal CI/CD platform written in Go.
Run pipelines, follow their status live, read logs and download build artifacts without leaving the terminal.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## Features

- **Login** with email / password or with GitHub (OAuth in the browser)
- **Repositories** — browse your GitHub repositories or your GopherCI organizations
- **Jobs** — paginated list with status filter, updated live through Server-Sent Events
- **Run a pipeline** from the `.goci` file of the current directory, or from a one-off command
- **Job actions** — view logs (scrollable), re-run, download the build artifact as a ZIP, delete
- **SSH keys** — add, delete and copy fingerprints, to build private repositories
- **Settings** — point the client at any GopherCI server

> The interface is in French.

---

## Installation

Requires Go 1.26+.

```bash
git clone https://github.com/lucaspose/GopherCI-CLI.git
cd GopherCI-CLI
make install          # builds and copies gopherci to ~/.local/bin
```

Or build without installing:

```bash
make build            # → ./bin/gopherci
```

With Docker only:

```bash
make docker-extract   # builds inside Docker and writes ./bin/gopherci
```

---

## Usage

```bash
gopherci              # start the interactive UI
gopherci --version
```

By default the client talks to `http://localhost:8080`, which is where a
GopherCI server started with `make up` listens. To use another server, change
the URL in **Settings** or set it for one run:

```bash
GOCI_API_URL=https://ci.example.com gopherci
```

Remote servers must use `https://`; plain `http://` is only accepted for `localhost`
so that passwords and tokens are never sent in clear text.

### Pipeline file

Put a `.goci` file at the root of your project, then create a new job (`n`) from the jobs screen:

```toml
[pipeline]
name = "my-project"

[[steps]]
name = "test"
cmd  = ["go", "test", "./..."]

[[steps]]
name = "build"
cmd  = ["go", "build", "./..."]
```

Each step runs in order on the GopherCI server; the job fails at the first failing step.

### Keyboard shortcuts

| Screen | Key | Action |
|--------|-----|--------|
| Everywhere | `↑` `↓` / `enter` / `esc` | Move / select / back |
| | `ctrl+c` | Quit |
| Jobs | `n` | New job |
| | `f` | Cycle status filter (running / failed / success / pending) |
| | `pgdn` `pgup` | Next / previous page |
| | `enter` | Job actions (logs, re-run, download ZIP, delete) |
| Repositories | `←` `→` | Change page |
| | `j` | Open the selected repository's jobs |
| SSH keys | `a` / `d` / `c` | Add / delete / copy fingerprint |

---

## Where things are stored

| What | Where |
|------|-------|
| Config and session token | `~/.goci-cli/config.json` (permissions `0600`) |
| Downloaded artifacts | `~/Downloads/gopherci/` (or `~/gopherci-artifacts/`), permissions `0600` |

---

## Development

```bash
make test             # unit tests
make ci               # format check + vet + race tests + build (same as the CI)
make help             # all targets
```

Integration tests run every API call against a real GopherCI server:

```bash
GOCI_API_URL=http://localhost:8080 go test -tags integration ./internal/api/
```

### Project layout

```
cmd/                 Bubble Tea program: model, update, views, commands
internal/api/        HTTP client for the GopherCI API
internal/config/     User config and .goci parsing
```

---

## License

Copyright (C) 2026 Lucas POSE

Licensed under the [GNU Affero General Public License v3.0](LICENSE).
You may use, modify and share this project, but any modified version, including
one offered as a network service, must be released under the same license.
