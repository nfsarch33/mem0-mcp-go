# mem0-mcp-go

Go-native MCP server for [Mem0 OSS](https://github.com/mem0ai/mem0) (self-hosted).

Provides a fast, single-binary MCP interface for any self-hosted Mem0 instance.
Designed as the primary Mem0 surface for Cursor, Claude Code, and other
MCP-compatible agents once you migrate from managed Mem0 cloud to self-hosted.

## Cursor MCP setup

Add this to `~/.cursor/mcp.json` under `mcpServers`:

```json
"mem0-oss": {
  "command": "<path-to>/mem0-mcp-go",
  "args": [],
  "env": {
    "MEM0_BASE_URL": "http://<your-mem0-host>:<port>",
    "MEM0_API_KEY": "<your-api-key>",
    "MEM0_USER_ID": "<your-user>",
    "MEM0_APP_ID": "<your-app>"
  }
}
```

The recommended MCP server name is **`mem0-oss`** to clearly distinguish it
from any cloud-backed Mem0 MCP server.

## Tools

| Tool | Description |
|------|-------------|
| `mem0_add` | Store a new memory |
| `mem0_search` | Semantic search over memories |
| `mem0_get_all` | List memories (paginated) |
| `mem0_update` | Update an existing memory |
| `mem0_delete` | Delete a memory |
| `mem0_history` | View edit history for a memory |
| `mem0_doctor` | Health check against the configured endpoint |

Cursor-compatible aliases are also registered: `memory_search`, `memory_write`,
`memory_read`.

## Configuration

| Env var | Default | Purpose |
|---------|---------|---------|
| `MEM0_BASE_URL` | empty (required) | Self-hosted Mem0 base URL |
| `MEM0_API_KEY` | empty | Mem0 OSS API key, sent as `X-API-Key` |
| `MEM0_USER_ID` | `default-user` | Default user namespace |
| `MEM0_APP_ID` | `default-app` | Default app namespace |
| `MCP_TRANSPORT` | `stdio` | `stdio` or `sse` |
| `MCP_SSE_ADDR` | `:9092` | SSE bind address |
| `MEM0_TIMEOUT` | `30s` | HTTP timeout (seconds or Go duration) |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |

`MEM0_DEFAULT_USER_ID` is accepted as a compatibility fallback.

### Operator deploy step

`MEM0_BASE_URL` is intentionally empty by default; the binary will not work
until you set it. Wire it in your private deploy environment. The Mem0 host
can be a tunnel endpoint, a private LAN address, or any URL your network
reaches. Keep that value out of public repositories.

Do not pass secrets on argv. Keep `MEM0_API_KEY` in a secrets manager or the
target host environment.

### Dual-Write (migration mode)

Fan out writes to a managed cloud API and/or a backup target while keeping
the self-hosted OSS instance as the primary read source. Useful during the
canary period when migrating from cloud to self-hosted.

| Env var | Default | Purpose |
|---------|---------|---------|
| `MEM0_DUAL_WRITE` | `false` | Enable write fan-out |
| `MEM0_CLOUD_URL` | `https://api.mem0.ai` | Cloud API base URL |
| `MEM0_CLOUD_API_KEY` | empty | Cloud API key (required when dual-write is on) |
| `MEM0_READ_SOURCE` | `oss` | Read from `oss` (primary) or `cloud` (shadow) |
| `MEM0_BACKUP_URL` | empty | Optional third write target |
| `MEM0_BACKUP_API_KEY` | empty | API key for the backup target |

When dual-write is enabled:

- **Primary** (OSS) writes are synchronous.
- **Shadow** (cloud) writes fire in a background goroutine and never block.
- **Backup** writes (if configured) also fire async.
- Shadow/backup errors are logged but never fail the MCP response.

## API compatibility

The search endpoint sends `user_id` and `app_id` inside the `filters` dict
(not as top-level params), matching the Mem0 OSS `/search` contract. The
`/memories` endpoint (add/get) uses top-level params as expected by Mem0 OSS.

## Development

```bash
go test -race ./...
go build -o bin/mem0-mcp-go ./cmd/mem0-mcp-go
```

## License

MIT
