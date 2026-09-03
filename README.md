# Fetchr

A Filestash-style web client for SFTP servers. Users sign in with Keycloak, enter SFTP
credentials for any reachable host, and browse, upload, download, rename and delete files
from the browser.

- **Backend** — Go, `chi`, `pkg/sftp`, `coreos/go-oidc`. Ships as a single static binary
  with the frontend embedded.
- **Frontend** — React 19, TypeScript, Vite, Tailwind, Radix (shadcn/ui style), TanStack Query.
- **Deployment** — distroless container image plus a Helm chart.

## How it works

```
browser ──▶ Go server ──▶ SFTP host
             │
             └─▶ Keycloak (OIDC authorization code + PKCE)
```

A login session is an opaque, HttpOnly cookie backed by an in-memory store. When a user
connects to an SFTP host, the live SSH/SFTP connection is kept in a server-side pool keyed
by that session ID. **SFTP credentials never leave server memory** — they are not written to
disk, not logged, and not returned to the browser. Both the session and its connection are
evicted after `--session-ttl` of inactivity, on logout, and on shutdown.

Uploads and downloads stream directly between the HTTP body and the remote file, so file
size is not bounded by server memory or disk.

## Local development

Two terminals, or `make dev` for both at once:

```sh
make deps

# Terminal 1 — API on :8080, authentication disabled
go run ./cmd/fetchr --skip-login --cookie-secure=false

# Terminal 2 — Vite dev server on :5173, proxying /api to :8080
cd web && npm run dev
```

Open http://localhost:5173.

A throwaway SFTP server to test against:

```sh
docker run --rm -p 2222:22 atmoz/sftp foo:pass:::upload
# then connect to host=localhost port=2222 user=foo password=pass
```

### Production-style build

```sh
make build          # builds the SPA into internal/web/dist, then the binary
./bin/fetchr --skip-login --cookie-secure=false
```

Everything is served from `:8080`.

## Configuration

Every flag has a `FETCHR_`-prefixed environment variable equivalent (uppercase, dashes to
underscores; `SFTPWEB_` is also supported for backwards compatibility). Flags take precedence over the environment.

| Flag | Env | Default | Description |
| --- | --- | --- | --- |
| `--addr` | `FETCHR_ADDR` | `:8080` | HTTP listen address |
| `--log-level` | `FETCHR_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `--skip-login` | `FETCHR_SKIP_LOGIN` | `false` | Bypass Keycloak entirely |
| `--oidc-issuer` | `FETCHR_OIDC_ISSUER` | — | Keycloak realm URL |
| `--oidc-client-id` | `FETCHR_OIDC_CLIENT_ID` | — | Client ID |
| `--oidc-client-secret` | `FETCHR_OIDC_CLIENT_SECRET` | — | Client secret (confidential clients) |
| `--oidc-redirect-url` | `FETCHR_OIDC_REDIRECT_URL` | — | Public URL of `/api/auth/callback` |
| `--oidc-scopes` | `FETCHR_OIDC_SCOPES` | `openid,profile,email` | Requested scopes |
| `--session-secret` | `FETCHR_SESSION_SECRET` | random | Signs the OAuth state cookie |
| `--session-ttl` | `FETCHR_SESSION_TTL` | `30m` | Idle lifetime of a session and its SFTP connection |
| `--cookie-secure` | `FETCHR_COOKIE_SECURE` | `true` | Set `Secure` on cookies; turn off for plain HTTP |
| `--max-upload-size` | `FETCHR_MAX_UPLOAD_SIZE` | `5GiB` | Per-file upload cap |
| `--allowed-hosts` | `FETCHR_ALLOWED_HOSTS` | *(any)* | Comma-separated SFTP host allowlist |
| `--max-connections` | `FETCHR_MAX_CONNECTIONS` | `200` | Concurrent SFTP connection ceiling |
| `--dial-timeout` | `FETCHR_DIAL_TIMEOUT` | `10s` | SFTP dial timeout |
| `--insecure-host-key` | `FETCHR_INSECURE_HOST_KEY` | `true` | Skip SSH host key verification |

`--oidc-*` flags are required unless `--skip-login` is set.

Leaving `--session-secret` unset generates a random one at startup, which invalidates
in-flight logins on restart and breaks multi-replica deployments. Set it explicitly in
production — the Helm chart does this for you.

## Keycloak setup

1. In your realm, create a client (e.g. `fetchr`).
2. Client authentication: **On** (confidential). Copy the secret from the *Credentials* tab.
3. Standard flow: **On**. Direct access grants: **Off**.
4. Valid redirect URIs: `https://sftp.example.com/api/auth/callback`
5. Valid post logout redirect URIs / Web origins: `https://sftp.example.com`

Then run:

```sh
./bin/fetchr \
  --oidc-issuer https://keycloak.example.com/realms/applications \
  --oidc-client-id fetchr \
  --oidc-client-secret "$CLIENT_SECRET" \
  --oidc-redirect-url https://sftp.example.com/api/auth/callback \
  --session-secret "$SESSION_SECRET"
```

For a local realm: `docker run --rm -p 8081:8080 -e KEYCLOAK_ADMIN=admin -e KEYCLOAK_ADMIN_PASSWORD=admin quay.io/keycloak/keycloak start-dev`.

## API

All `/api/sftp/*` routes require a session cookie and return `{"code","message"}` on error.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/auth/me` | Session state, used by the SPA to route |
| `GET` | `/api/auth/login` | Start the OIDC flow |
| `GET` | `/api/auth/callback` | OIDC redirect target |
| `POST` | `/api/auth/logout` | Drop session and SFTP connection |
| `POST` | `/api/sftp/connect` | `{host, port, username, password}` |
| `POST` | `/api/sftp/disconnect` | Close the SFTP connection, keep the login |
| `GET` | `/api/sftp/status` | Current connection info |
| `GET` | `/api/sftp/list?path=` | Directory listing |
| `GET` | `/api/sftp/download?path=` | Stream a file |
| `POST` | `/api/sftp/upload?path=` | `multipart/form-data`, field `file` (repeatable) |
| `POST` | `/api/sftp/mkdir` | `{path}` |
| `POST` | `/api/sftp/rename` | `{from, to}` |
| `DELETE` | `/api/sftp/remove?path=` | Delete a file or empty directory |

`/healthz` and `/readyz` are unauthenticated.

## Container

```sh
make docker                        # host architecture
make docker-amd64                  # linux/amd64
make docker-multi IMAGE=registry.example.com/fetchr:0.1.0   # multi-arch, pushed
docker run --rm -p 8080:8080 fetchr:dev --skip-login --cookie-secure=false
```

The image is distroless, runs as UID 65532, and needs no writable filesystem.

### Cross-architecture builds

The build requires **buildx**. The Node and Go stages are pinned to
`--platform=$BUILDPLATFORM` so they always run natively, and the Go binary is
cross-compiled via `GOARCH=$TARGETARCH`. Emulating those toolchains under QEMU
instead causes corrupted module downloads and compiler crashes, so do not remove
those `--platform` pins.

If `docker buildx version` fails, the CLI plugin is missing or its symlink is stale
(common after uninstalling Docker Desktop while using Colima):

```sh
brew install docker-buildx
ln -sf "$(brew --prefix)/opt/docker-buildx/bin/docker-buildx" ~/.docker/cli-plugins/docker-buildx
```

### Behind a TLS-intercepting proxy

On networks running Zscaler, Netskope or a similar MITM proxy, `npm ci` and
`go mod download` fail inside the build with `UNABLE_TO_GET_ISSUER_CERT_LOCALLY`
(npm may report the misleading `Exit handler never called!` instead). Export your
organisation's root CAs into `certs/` first:

```sh
make certs             # macOS; writes certs/corporate-ca.crt
make docker
```

The build appends anything matching `certs/*.crt` to the system trust store. The
directory is empty by default and the build works unchanged without it. `*.crt` is
gitignored so internal certificates are never committed.

For local development outside Docker, point Node at the same certificates:

```sh
export NODE_EXTRA_CA_CERTS=$PWD/certs/corporate-ca.crt
```

## Kubernetes

```sh
helm upgrade --install fetchr deploy/helm/fetchr \
  --namespace fetchr --create-namespace \
  --set image.repository=registry.example.com/fetchr \
  --set image.tag=0.1.0 \
  --set oidc.issuer=https://keycloak.example.com/realms/applications \
  --set oidc.clientId=fetchr \
  --set oidc.redirectUrl=https://sftp.example.com/api/auth/callback \
  --set secret.oidcClientSecret="$CLIENT_SECRET" \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=sftp.example.com
```

Prefer `--set secret.existingSecret=my-secret` with a Secret you manage out of band; it must
contain the keys `oidc-client-secret` and `session-secret`. When the chart creates the
Secret itself, it generates a random `session-secret` on first install and preserves it
across upgrades.

Notable values: `app.skipLogin`, `app.allowedHosts`, `app.maxUploadSize`, `service.sessionAffinity`,
`ingress.annotations`, `autoscaling.enabled`, `networkPolicy.enabled`. See
[deploy/helm/fetchr/values.yaml](deploy/helm/fetchr/values.yaml).

### Scaling

SFTP connections live in pod memory, so a client must keep reaching the same pod. The chart
sets `sessionAffinity: ClientIP` on the Service and cookie affinity on the nginx Ingress.
If you swap in a different ingress controller, configure equivalent affinity.

### Upload limits

`ingress.annotations."nginx.ingress.kubernetes.io/proxy-body-size"` must be at least
`app.maxUploadSize`, otherwise the ingress rejects large uploads before they reach the pod.

## Security notes

- **Host keys are not verified** (`--insecure-host-key`, on by default). This matches
  Filestash's default but leaves connections open to man-in-the-middle attacks on untrusted
  networks. Use `--allowed-hosts` and a NetworkPolicy to constrain reachable targets.
  Setting `--insecure-host-key=false` currently fails fast, since `known_hosts` pinning is
  not implemented yet.
- **`--skip-login` disables authentication completely.** Anyone who can reach the port gets
  a session. Local testing only.
- Users authenticate to the SFTP host with their own credentials, so they cannot exceed the
  permissions that host already grants them. Requested paths are still normalised and
  null-byte checked before use.
- Set `--cookie-secure=true` (the default) and serve over TLS in any shared environment.

## Testing

```sh
make test        # go vet, go test, tsc --noEmit
helm lint deploy/helm/fetchr
```

## Not implemented

SSH key authentication, non-SFTP backends (S3, WebDAV), in-browser file preview or editing,
share links, and recursive directory delete or upload.
