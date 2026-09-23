# Paradox CLI

**One CLI for your whole ecosystem.**

```
paradox init · paradox scan · paradox deploy · paradox doctor
```

Paradox is the single entry point for the Paradox Cloud platform. It provides configuration management, diagnostics, project initialization, and will eventually orchestrate Auth, Queue, Storage, Feature Flags, Deploy, Observability, and the Agent Runtime.

## Status

**v0.1.0-dev** — foundation + structured logging complete

| Command     | Status      | Description                                      |
|-------------|-------------|--------------------------------------------------|
| `version`   | ✅ Ready    | Print CLI version                                |
| `init`      | ✅ Ready    | Initialize a new Paradox project                 |
| `doctor`    | ✅ Ready    | Diagnose environment & configuration             |
| `scan`      | 🚧 Stub     | Discover services (next)                         |
| `deploy`    | 🚧 Stub     | Build & deploy (Paradox Deploy service)          |
| `completion`| ✅ Built-in | Shell completion (bash/zsh/fish/powershell)      |
| Logging     | ✅ Ready    | `--log-level` + `--log-format` (text/json)       |

## Quick start

```bash
# Build
make build

# Initialize a project
./bin/paradox init --name my-app

# Check your environment
./bin/paradox doctor

# See all commands
./bin/paradox --help
```

## Configuration

Paradox looks for `paradox.yaml` in:

1. Current directory
2. `./.paradox/`
3. `~/.config/paradox/`
4. `~/.paradox/`

You can also pass `--config /path/to/file.yaml`.

Environment variables are supported with the `PARADOX_` prefix (e.g. `PARADOX_LOG_LEVEL=debug`).

Example `paradox.yaml`:

```yaml
project_name: my-app
environment: development
log_level: info
services: []
```

## Logging

Structured logging is built in (stdlib `log/slog`).

```bash
# Human-readable (default)
paradox doctor --log-level=debug

# Machine-readable JSON (great for CI / shipping to Observability later)
paradox doctor --log-level=info --log-format=json
```

| Flag           | Values                  | Default     |
|----------------|-------------------------|-------------|
| `--log-level`  | debug, info, warn, error | info (or from config) |
| `--log-format` | text, json              | text        |

Logs go to stderr so they never pollute command output.

## Project structure

```
paradox-cli/
├── cmd/paradox/          # main entrypoint
├── internal/
│   ├── cmd/              # cobra commands
│   ├── config/           # configuration loading
│   ├── logging/          # structured logger (slog)
│   └── version/          # version info (ldflags)
├── Makefile
├── go.mod
└── README.md
```

## Development

```bash
make build          # build binary to bin/paradox
make run            # go run
make test           # run tests (when they exist)
make clean          # remove build artifacts
```

Build with version info:

```bash
make build VERSION=0.1.0 COMMIT=$(git rev-parse --short HEAD)
```

## Roadmap (next)

1. **scan** — real service discovery (Dockerfiles, compose, language runtimes)
2. **config validation** — schema + better error messages
3. **plugin system** — allow extensions for Auth / Queue / etc.
4. **update** — self-update mechanism
5. Wire into Paradox Auth, Queue, Deploy as those land

## License

MIT (planned)
