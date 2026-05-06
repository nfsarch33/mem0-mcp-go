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
| `MEM0_BASE_URL` | `http://127.0.0.1:18888` | Self-hosted Mem0 base URL. |
| `MEM0_API_KEY` | empty | Mem0 OSS API key, sent as `X-API-Key`. |
| `MEM0_USER_ID` | `nfsarch33` | Default user namespace. |
| `MEM0_APP_ID` | `cursor-global-kb` | Default app namespace. |
| `MCP_TRANSPORT` | `stdio` | `stdio` or `sse`. |
| `MCP_SSE_ADDR` | `:9092` | SSE bind address. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |

`MEM0_DEFAULT_USER_ID` is accepted as a compatibility fallback for existing
Cursor MCP templates.

Do not pass secrets on argv. Keep `MEM0_API_KEY` in 1Password or the target host
environment.

## Development

```bash
go test ./...
go run ./cmd/mem0-mcp-go --version
```
