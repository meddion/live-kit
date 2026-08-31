# Livekit Go Demo

This project consist of three services that allow for live meetings:
1. Entrypoint app (`livekit-app`). It's a Go backend that authenticates users, checks per-user permissions, and mints LiveKit join tokens. Users interact with it via a simple web UI.
2. LiveKit Meet UI (`livekit-meet`). After both authentication and authorization are successful for creating a new room, a user is redirected to the [LiveKit Meet UI](https://github.com/livekit-examples/meet) for the actual call. With the right JWT token, a user will establish a connection to `livekit-server` via WebSocket.
3. `livekit-server` is the videoconferencing core that enables WebRTC based calls.

## Capabilities

- **Authentication**: username/password login with bcrypt-hashed passwords; users are seeded idempotently from a JSON file on startup.
- **Session management**: in-memory, cookie-based sessions (`HttpOnly`, `SameSite=Lax`, 12h TTL). Sessions are lost on restart.
- **Permission-based authorization**: per-user permissions gate every action — `ViewRooms` (list rooms), `JoinRooms` (join an existing room), `CreateRooms` (create a new room), and `GodAlmighty` (bypasses all checks).
- **Room orchestration**: rooms auto-create on join for authorized users, with a 5-minute empty/departure timeout and a **hard cap of 4 participants** per room.
- **Short-lived join tokens**: LiveKit access tokens are minted as JWTs valid for 5 minutes, scoped to a single room and identity.
- **Real-time calls**: audio/video conferencing over WebRTC, handled by `livekit-server` and rendered in the LiveKit Meet UI.

## Limitations

- **TURN server is DISABLED**. TURN is commented out in [`livekit.yaml`](livekit.yaml), so media relaying is not available. WebRTC connectivity relies solely on direct/STUN paths (ports `7881` TCP and `7882` UDP). Participants behind **symmetric NATs, restrictive firewalls, or corporate/mobile networks** that block direct UDP may fail to establish media even after a successful join. To fix this in production, enable a TURN server (e.g. via the `turn` block in `livekit.yaml` with a valid TLS domain).
- **Max 4 participants per room** (`max_participants` in [`livekit.yaml`](livekit.yaml) / `RoomMaxParticipants`).
- **Sessions are in-memory only** — a backend restart logs everyone out; there is no horizontal scaling of the app without a shared session store.
- **No access-token/room password gating on join** — the `AccessToken` field on join is currently ignored (see `AuthorizeJoin`).
- **Single-node LiveKit** — the compose setup runs one `livekit-server`; no clustering/regional media routing.

## Development

This project relies on Docker Compose for dev and prod environments. 

In order to run the whole application via Docker Compose with the default users from `users-dev.json`, run from the root of the project:
```sh
make up-dev
```

To run all tests:
```sh
go test ./...
```

## Deployment

For a production deployment create a separate `.env` file, with randomly generated and secured `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET` and the composite `LIVEKIT_KEYS` (use `make generate-keys`). And if needed, set `USERS_FILE_PATH` to file with pre-existing users (like `users-dev.json`, but with secure passwords). For prod all three service endpoints (`livekit-app`, `livekit-meet` and `livekit-server`) must be TLS signed. You can use Caddy a reverse-proxy which will handle TLS certificates via Let's Encrypt for that. Or use Easypanel which uses Traefik proxy underhood and handles certificates for you. For certificates to be valid you need to register domains for the services: there are many providers to choose from.

### Example of a production .env
```env
APP_LOG_DEBUG=false
APP_PUBLIC_LIVEKIT_SERVER_URL=wss://livekit.example.domain
APP_PRIVATE_LIVEKIT_SERVER_URL=ws://livekit-server:7880
APP_MEET_URL=https://meet.example.domain
APP_DB_FILE=/app/data/main.db

LIVEKIT_API_KEY=<SUPER_SECRET_KEY>
LIVEKIT_API_SECRET=<SUPER_SECRET_SECRET>
LIVEKIT_KEYS=<SUPER_SECRET_KEY>: <SUPER_SECRET_SECRET>

ENV_FILE=.env
USERS_FILE_PATH=/etc/my-project/users.json # Path on a prod machine
``` 

