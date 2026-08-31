# AGENTS.md

Permission-based LiveKit video conferencing manager. A Go backend ([main.go](main.go)) authenticates users, checks per-user permissions, and mints LiveKit join tokens; a vanilla-JS SPA ([frontend/](frontend/)) drives login and room join; users are redirected to the LiveKit Meet UI for the actual call.

## Architecture

Layered, constructor-injected dependencies wired in [main.go](main.go). Request flow:

```
frontend/ (SPA) → transport/ (net/http handlers + session middleware)
                → service/ (business logic + permission gates)
                → db/ (SQLite) and LiveKit server-sdk-go (gRPC)
```

- [api/api.go](api/api.go) — shared DTOs, the `Permission` enum, and the `api.ErrNotAuthorized` sentinel. No logic; every layer imports this.
- [transport/](transport/) — HTTP routing/handlers ([handler.go](transport/handler.go)) and cookie-session auth middleware ([auth.go](transport/auth.go)).
- [service/](service/) — `Rooms` room orchestration + LiveKit token minting ([rooms.go](service/rooms.go)), `PermissionChecker` ([permission_checker.go](service/permission_checker.go)), in-memory `SessionStore` ([sessions.go](service/sessions.go)).
- [auth/](auth/) — context identity propagation ([ctx.go](auth/ctx.go)) and bcrypt hashing ([hash.go](auth/hash.go)).
- [db/](db/) — SQLite store via pure-Go `modernc.org/sqlite` (no CGO); schema + queries ([db.go](db/db.go)), JSON user seeding ([seed.go](db/seed.go)).

Two LiveKit URLs matter: the **private** URL is backend→server (in-cluster); the **public** URL is embedded in client tokens. Do not conflate them.

## Build / run / test

- `make up-dev` — build the app image and start all services with dev env (`.env.dev`, `users-dev.json`). App on `:8080`, LiveKit server on `:7880`, Meet UI on `:3088`.
- `make build-app` — build only the Go app image (`docker compose build livekit-app`).
- `make up-prod` — start detached with prod `.env`.
- `make token-dev` — mint a dev LiveKit token via the CLI for manual testing.
- `go build ./...` / `go test ./...` — local compile and tests. Unit tests are fast; the end-to-end suite in [integration/](integration/) is Docker-backed.

The app is compiled `CGO_ENABLED=0` into a static binary ([Dockerfile](Dockerfile)); keep dependencies CGO-free.

## Conventions

- **Method receivers are named `this`** throughout the codebase. Match this for consistency (it is non-idiomatic Go but project-wide).
- Constructors are `New*`; HTTP handlers are `Handle*`; dependencies are passed as **interfaces** defined at the consumer (e.g. `RoomService`, `PermissionChecker`) to keep layers testable.
- Errors: wrap with `fmt.Errorf("...: %w", err)`; signal authorization failures with the `api.ErrNotAuthorized` sentinel and branch on it via `errors.Is` in handlers to pick the HTTP status.
- Logging: structured `log/slog` with key-value pairs (`slog.Error("msg", "error", err)`). Debug logs are gated by `APP_LOG_DEBUG=true`.
- Permission checks live in the **service layer**, not handlers. `PermGodAlmighty` bypasses all checks. Add new permissions to both `api.Permission` and `db.defaultPermissions`.
- HTTP routing uses the **Go 1.22+ `net/http` mux** with method+path patterns (`"POST /api/v1/rooms/{room}/join"`) and `r.PathValue(...)` — no third-party router.

## Auth model

Session cookies (`session`, `HttpOnly`, `SameSite=Lax`, 12h TTL) map to an **in-memory** `SessionStore` (sessions are lost on restart). Middleware validates the cookie and injects the username into the request context via `auth.WithIdentity`; downstream code reads it with `auth.IdentityFromContext`. Passwords are bcrypt-hashed. LiveKit join tokens are short-lived JWTs (5 min) minted by `service.TokenGenerator`.

## Configuration

Config is parsed from environment variables in `ParseConfigFromEnv` ([main.go](main.go)). Env files (`.env`, `.env.dev`) and `users.json` are gitignored (secrets); `users-dev.json` is tracked for local dev. Users are seeded idempotently from the JSON file on startup — edit [users-dev.json](users-dev.json) to change dev accounts/permissions. LiveKit server behavior is set in [livekit.yaml](livekit.yaml) (rooms auto-create, max 4 participants).
