# Paradox CLI

**One tool for your whole backend platform.**

Paradox is a small “cloud toolkit” you run on your machine.  
It gives you the same *kinds* of pieces real companies use — auth, jobs, flags, storage, deploy — without needing AWS on day one.

Think of it as:

> **Auth0 + a job queue + feature flags + S3 + a tiny Heroku — in one CLI, running locally.**

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
| **Deploy** | Build → run → health check → rollback | ✅ (local) |
| **Scan / Doctor** | “What’s wrong with my setup?” | ✅ |

Still coming later: public HTTP API gateway, shared dashboard, multi-machine workers.

---

## Install (2 minutes)

```bash
git clone https://github.com/Paradoxdreamer/paradox-cli.git
cd paradox-cli
make build
./bin/paradox version
```

Optional: put it on your PATH:

```bash
make install
# or:  cp bin/paradox /usr/local/bin/
```

---

## First 5 minutes

```bash
# 1. Check your machine
paradox doctor

# 2. Create users & log in
paradox auth init
paradox auth register --email you@example.com --password 'secret123' --role admin
paradox auth login    --email you@example.com --password 'secret123'
paradox auth whoami

# 3. Background jobs
paradox queue init
paradox queue enqueue --type echo --payload '{"msg":"hi"}'
paradox queue worker          # leave running in another terminal

# 4. Feature flags
paradox flags init
paradox flags set new_ui --pct 10 --env development
paradox flags eval new_ui --email you@example.com --env development

# 5. File storage (mini S3)
paradox storage init
paradox storage mb my-bucket
paradox storage put my-bucket notes/hello.txt -f ./README.md

# 6. Deploy something local
paradox deploy init
paradox deploy run --name demo --cmd "python3 -m http.server 8787" --health http://127.0.0.1:8787/
paradox deploy status
```

---

## Everyday commands (cheat sheet)

### Auth

| Command | Meaning |
|---------|--------|
| `paradox auth register --email … --password …` | Create a user |
| `paradox auth login --email … --password …` | Get a JWT + save session |
| `paradox auth whoami` | Who am I? |
| `paradox auth logout` | Clear session |

### Queue

| Command | Meaning |
|---------|--------|
| `paradox queue enqueue --type echo --payload '{…}'` | Add a job |
| `paradox queue worker` | Process jobs (leave this running) |
| `paradox queue list` | See jobs |
| `paradox queue status` | Counts by status |

### Feature flags

| Command | Meaning |
|---------|--------|
| `paradox flags set NAME --pct 25 --env development` | Create / update a flag |
| `paradox flags eval NAME --email you@x.com` | Is it on for this user? |
| `paradox flags disable NAME` | **Instant kill switch** |

### Storage

| Command | Meaning |
|---------|--------|
| `paradox storage mb BUCKET` | Create a bucket |
| `paradox storage put BUCKET key -f FILE` | Upload |
| `paradox storage get BUCKET key -o FILE` | Download |
| `paradox storage sign BUCKET key` | Temporary signed link |

### Deploy

| Command | Meaning |
|---------|--------|
| `paradox deploy run --name APP --cmd "…"` | Start a release |
| `paradox deploy status` | What’s running? |
| `paradox deploy stop --name APP` | Stop |
| `paradox deploy rollback --name APP` | Go back to previous release |
| `paradox deploy logs --name APP` | Tail recent output |

### Project helpers

| Command | Meaning |
|---------|--------|
| `paradox init` | Create `paradox.yaml` |
| `paradox scan` | Detect services in the folder |
| `paradox doctor` | Health check for the whole toolkit |
| `paradox config validate` | Is my config valid? |

---

## How the pieces fit (simple picture)

```
                 YOU (paradox CLI)
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
      Auth           Storage          Flags
   (who are you?)   (files)      (on/off features)
        │               │               │
        └───────────────┼───────────────┘
                        ▼
                     Queue
              (background work)
                        ▼
                     Deploy
           (run the app + health + rollback)
```

Everything stores data under **`~/.paradox/`** by default (or `$PARADOX_DATA_DIR`).

This is **local-first**. It’s meant to teach the architecture and be useful for personal projects. A multi-server “real cloud” comes later on the same design.

---

## Configuration

Paradox looks for `paradox.yaml` in:

1. Current directory  
2. `./.paradox/`  
3. `~/.paradox/`  

Example:

```yaml
project_name: my-app
environment: development   # development | staging | production | test
log_level: info            # debug | info | warn | error
```

Useful env vars:

| Variable | Purpose |
|----------|--------|
| `PARADOX_DATA_DIR` | Where all data lives |
| `PARADOX_JWT_SECRET` | Auth signing key (optional) |
| `PARADOX_STORAGE_SECRET` | Storage signed-URL key (optional) |
| `PARADOX_LOG_LEVEL` | Override log level |

---

## Logging

```bash
paradox doctor --log-level=debug
paradox doctor --log-format=json    # good for CI
```

Logs go to **stderr** so they never mix with normal command output.

---

## Project layout (for contributors)

```
cmd/paradox/          # main entry
internal/
  auth/               # users + JWT
  queue/              # jobs + worker
  flags/              # feature flags
  storage/            # object store
  deploy/             # local deploy + releases
  registry/           # how services plug into the CLI
  config/             # one shared config + validation
  logging/            # structured logs
```

**Rule we follow:** new platform pieces register through `internal/registry` and get a blank import in `internal/cmd/services.go`. That keeps the codebase from turning into spaghetti.

---

## What “cloud” means here (honest)

| We have | We don’t have yet |
|---------|-------------------|
| Local auth, jobs, flags, storage, deploy | Multi-tenant hosted SaaS |
| CLI that feels like a platform | Public HTTP API gateway |
| Architecture you can grow | Automatic global scale |

So: **strong foundation for a personal / learning cloud**, not “AWS in a box” yet.

---

## License

MIT (planned)

---

**Questions / ideas?** Open an issue on the repo.  
Build → break → learn → ship.
