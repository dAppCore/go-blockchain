# daemon — Lethean Go Chain Daemon

JSON-RPC + REST API server backed by the Go chain.

## File Structure

| File | Purpose | Methods |
|------|---------|---------|
| server.go | Core RPC router + all handlers (LEGACY — needs splitting) | 84 |
| wallet_rpc.go | Wallet RPC proxy to C++ wallet | 16 |
| server_test.go | Unit tests for RPC handlers | 22 |
| server_integration_test.go | Go-vs-C++ comparison tests | 6 |
| coverage_test.go | Tests ALL 48 RPC methods in one pass | 1 |

## API Surface

- 69 chain JSON-RPC methods (native Go + CGo crypto)
- 16 wallet proxy methods (C++ wallet backend)
- 17 HTTP/REST endpoints (web-friendly JSON)
- Total: 102 endpoints

## AX Debt

server.go uses banned imports (encoding/json, net/http).
Migration to core/api is tracked. When core/api integration happens:
- JSON encoding → core.JSONMarshalString
- HTTP routing → core/api router
- SSE → core/api SSEBroker
- Metrics → core/api middleware

## Categories (for future file split)

1. Chain queries (getinfo, getheight, getblockheaderbyheight, etc.)
2. Alias operations (get_all_alias_details, get_alias_by_address, etc.)
3. Crypto utilities (validate_signature, generate_keys, fast_hash, etc.)
4. Explorer/analytics (get_chain_stats, get_difficulty_history, etc.)
5. Service discovery (get_gateways, get_vpn_gateways, get_network_topology)
6. REST endpoints (/api/info, /api/block, /api/aliases, /health, /metrics)
7. SSE (/events/blocks)
8. Admin (pool, relay, reset stubs)
