# Paradox CLI

**One CLI for your whole ecosystem.**

```
paradox init · paradox scan · paradox deploy · paradox doctor · paradox config
```

Paradox is the single entry point for the Paradox Cloud platform. It provides configuration management, diagnostics, project initialization, and will eventually orchestrate Auth, Queue, Storage, Feature Flags, Deploy, Observability, and the Agent Runtime.

## Status

**v0.1.0-dev** — foundation + logging + scan + config validation

| Command              | Status   | Description                                         |
|----------------------|----------|-----------------------------------------------------|
| `version`            | ✅ Ready | Print CLI version                                   |
| `init`               | ✅ Ready | Initialize a new Paradox project                    |
| `doctor`             | ✅ Ready | Diagnose environment, config & schema               |
| `scan`               | ✅ Ready | Discover config, Dockerfiles, compose, runtimes     |
| `config validate`    | ✅ Ready | Validate paradox.yaml against the platform schema   |
| `config show`        | ✅ Ready | Print effective configuration                       |
| `deploy`             | 🚧 Stub  | Build & deploy (Paradox Deploy service)             |
| `completion`         | ✅ Built-in | Shell completion                                 |
| Logging              | ✅ Ready | `--log-level` + `--log-format` (text/json)          |

## Design rule (anti-spaghetti)

**All configuration lives in `internal/config`.**  
Auth, Queue, Deploy, Feature Flags, etc. must:

1. Read from `*config.Config` (or nested sections we add there)
2. Rely on `cfg.Validate()` — never invent their own validation
3. Never add a second YAML/loader path

When you need a new setting, add it to `Config` + `Validate()` first.

## Quick start

```bash
make build
./bin/paradox init --name my-app
./bin/paradox doctor
./bin/paradox scan
./bin/paradox config validate
./bin/paradox config show
```

## Configuration

Looks for `paradox.yaml` in: `.` → `./.paradox/` → `~/.config/paradox/` → `~/.paradox/`

```yaml
project_name: my-app
environment: development   # development|staging|production|test
log_level: info            # debug|info|warn|error
services: []
```

## Logging

```bash
paradox doctor --log-level=debug
paradox doctor --log-format=json
```

## Project structure

```
paradox-cli/
├── cmd/paradox/
├── internal/
│   ├── cmd/
│   ├── config/      # Load + Validate (single source of truth)
│   ├── logging/
│   ├── scan/
│   └── version/
├── Makefile
├── go.mod
└── README.md
```

## Roadmap

1. ~~Config validation~~ ✅
2. Plugin system (so Auth / Queue register cleanly)
3. Self-update
4. Wire Auth → Queue → Deploy

## License

MIT (planned)
