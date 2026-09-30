# Paradox CLI

**One toolkit for a personal backend platform.**

Auth, jobs, feature flags, storage, deploy, gateway, agents, video upscale —  
local-first, under `~/.paradox/`, shaped like real production systems.

> **Auth0 + queue + flags + S3 + tiny Heroku + API gateway + agent runtime** — one CLI.

---

## What you get

| Piece | Plain English | Status |
|--------|----------------|--------|
| **CLI** | One binary: `paradox` | ✅ |
| **Auth** | Users, JWT, roles | ✅ |
| **Queue** | Jobs, retries, dead-letter | ✅ |
| **Flags** | Rollouts + kill switches | ✅ |
| **Storage** | Mini S3 | ✅ |
| **Deploy** | Run → health → rollback | ✅ |
| **Gateway** | HTTP API, keys, rate limits | ✅ |
| **Observability** | track / error / metric | ✅ |
| **Agent** | ParadoxGPT (echo / free / OpenAI) | ✅ |
| **Upscale** | Media jobs + ffmpeg or simulate | ✅ |
| **Cloud** | `paradox cloud up` | ✅ |

---

## Install

```bash
git clone https://github.com/Paradoxdreamer/paradox-cli.git
cd paradox-cli && make build && ./bin/paradox version
```

## 5-minute tour

```bash
paradox doctor
paradox auth init
paradox auth register --email you@example.com --password 'secret123' --role admin
paradox cloud up
# other terminal:
curl http://127.0.0.1:8080/health
paradox cloud status
```

### HTTP

```bash
curl -X POST http://127.0.0.1:8080/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"secret123"}'

curl -X POST http://127.0.0.1:8080/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"secret123"}'

paradox agent create bot --model echo --prompt "Be helpful."
curl -X POST http://127.0.0.1:8080/v1/agent/run -H 'Content-Type: application/json' \
  -d '{"agent":"bot","message":"hello"}'
```

## Agent models

| `--model` | Backend |
|-----------|--------|
| `echo` | Offline |
| `free` / `omega` | Free Qwen → Gpt-4-mini fallback |
| `openai` | OpenAI-compatible (`PARADOX_LLM_*`) |

## Gateway routes

| Method | Path |
|--------|------|
| GET | `/health` |
| POST | `/v1/auth/register` · `/v1/auth/login` |
| GET | `/v1/whoami` |
| POST | `/v1/agent/run` · GET `/v1/agent/list` |
| POST | `/v1/queue/enqueue` · GET `/v1/queue/status` |
| POST | `/v1/upscale` · GET `/v1/upscale/{id}` |
| GET | `/v1/flags/eval` |
| POST | `/v1/obs/track` · `/v1/obs/metric` |

## Architecture

```
         paradox cloud up
                │
         ┌──────┴──────┐
         ▼             ▼
     Gateway        Workers
        │         (echo+upscale)
   Auth Flags Queue Storage Agents Obs
```

Data: `~/.paradox/` or `$PARADOX_DATA_DIR`.

## Honest status

Local learning cloud — not multi-tenant AWS. Serious engineer foundation.

## License

MIT (planned)
