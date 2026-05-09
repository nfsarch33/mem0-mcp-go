# mem0-mcp-go

Go-native MCP server for self-hosted Mem0 OSS.

It replaces the cloud-oriented `uvx mem0-mcp-server` once the personal stack
flips reads from managed Mem0 to the Oracle-hosted OSS deployment.

## Tools

- `mem0_add`
- `mem0_search`
- `mem0_get_all`
- `mem0_update`
- `mem0_delete`
- `mem0_history`
- `mem0_doctor`

## Configuration

| Env var | Default | Purpose |
| --- | --- | --- |
| `MEM0_BASE_URL` | empty (required) | Self-hosted Mem0 base URL. |
| `MEM0_API_KEY` | empty | Mem0 OSS API key, sent as `X-API-Key`. |
| `MEM0_USER_ID` | `default-user` | Default user namespace. |
| `MEM0_APP_ID` | `default-app` | Default app namespace. |
| `MCP_TRANSPORT` | `stdio` | `stdio` or `sse`. |
| `MCP_SSE_ADDR` | `:9092` | SSE bind address. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |

`MEM0_DEFAULT_USER_ID` is accepted as a compatibility fallback for existing
Cursor MCP templates.

### Operator deploy step

`MEM0_BASE_URL` is intentionally empty by default; the binary will not work
until you set it. Wire it in your private deploy environment, for example:

```bash
export MEM0_BASE_URL="http://<your-mem0-host>:<port>"
export MEM0_API_KEY="$(op read 'op://<vault-name>/<item-name>/api_key')"
export MEM0_USER_ID="<your-account>"
export MEM0_APP_ID="<your-app>"
```

The Mem0 host can be a tunnel endpoint, a private LAN address, or any URL
your network reaches. Keep that value out of public repositories — the
defaults shipped here intentionally encode no operator topology.

Do not pass secrets on argv. Keep `MEM0_API_KEY` in 1Password or the target host
environment.

### Dual-Write

Fan out writes to a managed cloud API and/or a backup target while keeping
the self-hosted OSS instance as the primary. Reads are always served by one
backend (configurable).

| Env var | Default | Purpose |
| --- | --- | --- |
| `MEM0_DUAL_WRITE` | `false` | Enable write fan-out. |
| `MEM0_CLOUD_URL` | `https://api.mem0.ai` | Cloud API base URL. |
| `MEM0_CLOUD_API_KEY` | empty | Cloud API key. Required when dual-write is on. |
| `MEM0_READ_SOURCE` | `oss` | Read from `oss` (primary) or `cloud` (shadow). |
| `MEM0_BACKUP_URL` | empty | Optional third write target. |
| `MEM0_BACKUP_API_KEY` | empty | API key for the backup target. |

When dual-write is enabled:

- **Primary** (OSS) writes are synchronous — the MCP tool blocks until done.
- **Shadow** (cloud) writes fire in a background goroutine and never block.
- **Backup** writes (if configured) also fire async.
- Shadow/backup errors are logged via slog but never fail the MCP response.
- The `/health` and `/healthz` endpoints report dual-write status.

## Development

```bash
go test ./...
go run ./cmd/mem0-mcp-go --version
```
