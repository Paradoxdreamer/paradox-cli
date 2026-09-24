# Paradox CLI

**One tool for your whole backend platform.**

Paradox is a small “cloud toolkit” you run on your machine.  
It gives you the same *kinds* of pieces real companies use — auth, jobs, flags, storage, deploy, gateway — without needing AWS on day one.

Think of it as:

> **Auth0 + a job queue + feature flags + S3 + a tiny Heroku + an API gateway — in one CLI, running locally.**

---

## Who is this for?

- Developers who want to **learn real backend architecture** by building it  
- People who want a **personal platform** before jumping to Kubernetes  
- Anyone tired of glueing 8 SaaS tools together for a side project  

You do **not** need to be a DevOps expert. If you can run `make build`, you’re fine.

---

## What you get today

| Piece | What it does (in plain English) | Status |
|--------|----------------------------------|--------|
| **CLI** | One command: `paradox` | ✅ |
| **Auth** | Users, login, JWT tokens | ✅ |
| **Queue** | Background jobs (retry + dead-letter) | ✅ |
| **Flags** | Feature flags, rollouts, kill switches | ✅ |
| **Storage** | File buckets like a mini S3 | ✅ |
| **Deploy** | Run → health check → rollback | ✅ (local) |
| **Gateway** | HTTP API, keys, rate limits | ✅ |
| **Scan / Doctor** | “What’s wrong with my setup?” | ✅ |

Still coming later: observability dashboard, multi-machine workers.

---

## Install (2 minutes)

```bash
git clone https://github.com/Paradoxdreamer/paradox-cli.git
cd paradox-cli
make build
./bin/paradox version
```

---

## First 5 minutes

```bash
paradox doctor

paradox auth init
paradox auth register --email you@example.com --password 'secret123' --role admin
paradox auth login --email you@example.com --password 'secret123'

paradox queue init
paradox flags init
paradox storage init
paradox deploy init
paradox gateway init
paradox gateway keys create --name local
paradox gateway start --addr 127.0.0.1:8080 --require-key
# curl -H "X-API-Key: pk_…" http://127.0.0.1:8080/health
```

---

## Everyday commands (cheat sheet)

### Auth
| Command | Meaning |
|---------|--------|
| `paradox auth register / login / whoami` | Users & JWT |

### Queue
| Command | Meaning |
|---------|--------|
| `paradox queue enqueue / worker / list` | Background jobs |

### Flags
| Command | Meaning |
|---------|--------|
| `paradox flags set / eval / disable` | Rollouts & kill switch |

### Storage
| Command | Meaning |
|---------|--------|
| `paradox storage mb / put / get / sign` | Mini S3 |

### Deploy
| Command | Meaning |
|---------|--------|
| `paradox deploy run / status / stop / rollback` | Local process deploys |

### Gateway
| Command | Meaning |
|---------|--------|
| `paradox gateway start` | HTTP API server |
| `paradox gateway keys create --name local` | API key |
| `curl -H "X-API-Key: pk_…" /v1/flags/eval?key=…` | Call over HTTP |

### Helpers
| Command | Meaning |
|---------|--------|
| `paradox init / scan / doctor / config validate` | Project tooling |

---

## How the pieces fit

```
                 YOU (CLI or HTTP)
                        │
                   API Gateway
                 (keys + rate limits)
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
      Auth           Storage          Flags
        │               │               │
        └───────────────┼───────────────┘
                        ▼
                     Queue → Deploy
```

Data lives under `~/.paradox/` (or `$PARADOX_DATA_DIR`).

---

## Gateway HTTP API

| Method | Path | Auth |
|--------|------|------|
| GET | `/health` | none |
| GET | `/v1/whoami` | Bearer JWT |
| GET | `/v1/flags/eval?key=&email=&env=` | API key |
| POST | `/v1/queue/enqueue` | API key |
| GET | `/v1/queue/status` | API key |

```bash
paradox gateway keys create --name local
paradox gateway start --addr 127.0.0.1:8080 --require-key

curl -H "X-API-Key: pk_…" http://127.0.0.1:8080/v1/flags/eval?key=demo_flag&env=development
curl -H "X-API-Key: pk_…" -H "Content-Type: application/json" \
  -d '{"type":"echo","payload":{"hi":true}}' \
  http://127.0.0.1:8080/v1/queue/enqueue
```

---

## What “cloud” means here (honest)

| We have | We don’t have yet |
|---------|-------------------|
| Local auth, jobs, flags, storage, deploy, gateway | Multi-tenant hosted SaaS |
| HTTP API on your machine | Global edge network |
| Architecture you can grow | Automatic scale-out |

**Personal / learning cloud foundation** — not AWS in a box.

---

## License

MIT (planned)
