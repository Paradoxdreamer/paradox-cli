# Paradox CLI

**One CLI for your whole ecosystem.**

```
paradox init · scan · doctor · config · auth · deploy
```

## Status

**v0.1.0-dev**

| Area | Status | Notes |
|------|--------|-------|
| CLI foundation | ✅ | Cobra, config, logging |
| `init` / `doctor` / `scan` | ✅ | |
| Config validation | ✅ | Single schema in `internal/config` |
| Service registry | ✅ | Thin, explicit — no plugin magic |
| Paradox Auth v0 | ✅ | `auth status/init/whoami` + doctor check |
| Deploy | 🚧 | Stub |

## Architecture (anti-spaghetti)

```
CLI (cmd/)
 └── registry          ← services register here only
      └── auth         ← first real service
      └── queue        ← next
      └── …
 └── config            ← single source of truth + Validate()
 └── logging / scan
```

**Rules:**
1. New settings → `internal/config` + `Validate()` first
2. New services → `registry.Register` + blank import in `internal/cmd/services.go`
3. No second config loaders, no ad-hoc doctor checks outside the registry

## Quick start

```bash
make build
./bin/paradox init --name my-app
./bin/paradox doctor
./bin/paradox scan
./bin/paradox config validate
./bin/paradox auth init
./bin/paradox auth status
```

## Auth (v0)

Local store under `~/.paradox/auth` (or `$PARADOX_DATA_DIR/auth`).

```bash
paradox auth init      # create store + users.json
paradox auth status
paradox auth whoami    # stub until JWT/sessions land
```

Next Auth work: JWT issuance, login, sessions.

## License

MIT (planned)
