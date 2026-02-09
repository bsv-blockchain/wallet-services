# Wallet Services

A standalone Go server that combines [Arcade](https://github.com/bsv-blockchain/arcade) (transaction propagation to teranode) and [Chaintracks](https://github.com/bsv-blockchain/go-chaintracks) (blockchain header storage) behind a single HTTP API with a shared libp2p P2P client.

This is the server that [go-wallet-toolbox](https://github.com/bsv-blockchain/go-wallet-toolbox) connects to for transaction broadcast and header lookups.

## Architecture

```
 go-wallet-toolbox
       |
       | HTTP
       v
 +--------------------+
 | Wallet Services|
 |                    |
 | /arcade/*          |--> Arcade ----> Teranodes (tx broadcast)
 |                    |       |
 | /chaintracks/*     |    libp2p <---> BSV P2P Network
 |                    |       |
 |                    |--> Chaintracks (header sync & storage)
 +--------------------+
```

Arcade and Chaintracks share a single libp2p P2P client for connecting to the BSV network. Arcade handles transaction submission to teranodes and tracks their status via P2P. Chaintracks syncs and stores block headers.

## How Applications Use This Server

Applications and AI agents don't call this server directly. They use [go-wallet-toolbox](https://github.com/bsv-blockchain/go-wallet-toolbox) as a wallet SDK, which connects to this server as its backend infrastructure.

```
Application / AI Agent
    |
    | Go SDK (construct tx, sign, verify)
    v
go-wallet-toolbox (wallet library)
    |
    | HTTP
    v
Wallet Services (this server)
    |
    | libp2p / ARC API
    v
BSV Network / Teranodes
```

**go-wallet-toolbox** handles key management, transaction construction, and signing. This server handles the two things it can't do alone:

- **Broadcasting transactions** — wallet-toolbox submits signed transactions to `/arcade/tx`, which forwards them to teranodes. It can then track status via `/arcade/tx/{txid}` or subscribe to `/arcade/events` (SSE) to know when a transaction is mined.
- **SPV verification** — when verifying an incoming payment, wallet-toolbox fetches block headers from `/chaintracks/v2/*` and validates the merkle proof against them. This is Simplified Payment Verification — the wallet does the cryptographic verification, this server provides the headers.

### Example: AI agent accepting payments

1. Agent generates an invoice (a BSV address + amount) via wallet-toolbox
2. User pays to that address
3. Wallet-toolbox verifies the payment via SPV — fetches the relevant block header from `/chaintracks/v2/header/hash/:hash` and validates the merkle proof
4. Agent performs the requested work
5. Agent optionally anchors a receipt on-chain by broadcasting an OP_RETURN transaction via `/arcade/tx`

Any application that needs to send, receive, or verify BSV transactions can use this same pattern: wallet-toolbox as the client library, this server as the shared infrastructure.

## Quick Start

```bash
# Build
go build -o wallet-services .

# Run (teranode broadcast URL is required)
BWS_TERANODE_BROADCAST_URLS=https://arc.taal.com ./wallet-services

# Verify
curl http://localhost:3011/health
```

## API Endpoints

### Arcade (Transaction Broadcast)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/arcade/tx` | Submit a transaction |
| `POST` | `/arcade/txs` | Submit multiple transactions |
| `GET` | `/arcade/tx/{txid}` | Get transaction status |
| `GET` | `/arcade/events?callbackToken=X` | Stream transaction status updates (SSE) |
| `GET` | `/arcade/policy` | Get broadcast policy |

### Chaintracks v2 (Block Headers)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/chaintracks/v2/network` | Get network name |
| `GET` | `/chaintracks/v2/height` | Get chain height |
| `GET` | `/chaintracks/v2/tip` | Get chain tip |
| `GET` | `/chaintracks/v2/tip/stream` | Stream tip updates (SSE) |
| `GET` | `/chaintracks/v2/header/height/:height` | Get header by height |
| `GET` | `/chaintracks/v2/header/hash/:hash` | Get header by hash |
| `GET` | `/chaintracks/v2/headers?height=N&count=M` | Get multiple headers |

### Chaintracks Legacy v1

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/chaintracks/v1/getChain` | Get network name |
| `GET` | `/chaintracks/v1/getPresentHeight` | Get chain height |
| `GET` | `/chaintracks/v1/findChainTipHashHex` | Get chain tip hash |
| `GET` | `/chaintracks/v1/findChainTipHeaderHex` | Get chain tip header |
| `GET` | `/chaintracks/v1/findHeaderHexForHeight?height=N` | Get header by height |
| `GET` | `/chaintracks/v1/findHeaderHexForBlockHash?hash=X` | Get header by hash |
| `GET` | `/chaintracks/v1/getHeaders?height=N&count=M` | Get multiple headers |

### Server

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check (JSON) |
| `GET` | `/` | Status dashboard (HTML) |

## Configuration

Configuration is loaded from three sources in order of precedence:

1. **Environment variables** with the `BWS_` prefix (highest priority)
2. **Config file** named `config.yaml` searched in: `./`, `$HOME/.bws/`, `/etc/bws/`
3. **Built-in defaults** (lowest priority)

Nested config keys use `_` as separator in environment variables. For example, `server.address` becomes `BWS_SERVER_ADDRESS`.

Copy `config.yaml.example` to `config.yaml` to get started.

### Top-Level Fields

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `mode` | `BWS_MODE` | string | `embedded` | Arcade mode. `embedded` runs all services in-process. `remote` connects to an external arcade server via REST. |
| `url` | `BWS_URL` | string | _(empty)_ | URL of the remote arcade server. Required when `mode` is `remote`. |
| `network` | `BWS_NETWORK` | string | `main` | Bitcoin network: `main`, `test`, or `stn`. |
| `storage_path` | `BWS_STORAGE_PATH` | string | `~/.arcade` | Base directory for persistent data files (P2P keys, peer cache, etc.). |
| `log_level` | `BWS_LOG_LEVEL` | string | `info` | Log level: `debug`, `info`, `warn`, `error`. |

### Server (`server.*`)

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `server.address` | `BWS_SERVER_ADDRESS` | string | `:3011` | HTTP listen address (host:port). |
| `server.read_timeout` | `BWS_SERVER_READ_TIMEOUT` | duration | `30s` | Maximum duration for reading the request. |
| `server.write_timeout` | `BWS_SERVER_WRITE_TIMEOUT` | duration | `30s` | Maximum duration for writing the response. |
| `server.shutdown_timeout` | `BWS_SERVER_SHUTDOWN_TIMEOUT` | duration | `10s` | Grace period for in-flight requests during shutdown. |

### Database (`database.*`)

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `database.type` | `BWS_DATABASE_TYPE` | string | `sqlite` | Database engine: `sqlite` or `postgres`. |
| `database.sqlite_path` | `BWS_DATABASE_SQLITE_PATH` | string | `~/.arcade/arcade.db` | Path to the SQLite database file. Supports `~/` expansion. |
| `database.postgres_conn_str` | `BWS_DATABASE_POSTGRES_CONN_STR` | string | _(empty)_ | PostgreSQL connection string. Required when `database.type` is `postgres`. |

### Teranode (`teranode.*`)

These configure the connection to teranode ARC endpoints for broadcasting transactions.

| Key | Env Var | Type | Default | Required | Description |
|-----|---------|------|---------|----------|-------------|
| `teranode.broadcast_urls` | `BWS_TERANODE_BROADCAST_URLS` | []string | _(empty)_ | **Yes** | Teranode ARC endpoint URLs for submitting transactions. The server will fail to start without at least one URL. |
| `teranode.datahub_urls` | `BWS_TERANODE_DATAHUB_URLS` | []string | _(empty)_ | No | Teranode DataHub URLs for fetching block/subtree data (fallback). |
| `teranode.auth_token` | `BWS_TERANODE_AUTH_TOKEN` | string | _(empty)_ | No | Bearer token for authenticating with teranode endpoints. |
| `teranode.timeout` | `BWS_TERANODE_TIMEOUT` | duration | `30s` | No | HTTP request timeout for teranode calls. |

### Events (`events.*`)

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `events.type` | `BWS_EVENTS_TYPE` | string | `memory` | Event publisher backend: `memory`. |
| `events.buffer_size` | `BWS_EVENTS_BUFFER_SIZE` | int | `1000` | Event channel buffer size. |

### Validator (`validator.*`)

Transaction validation policy settings.

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `validator.max_tx_size` | `BWS_VALIDATOR_MAX_TX_SIZE` | int | `4294967296` | Maximum transaction size in bytes (~4 GB). |
| `validator.max_script_size` | `BWS_VALIDATOR_MAX_SCRIPT_SIZE` | int | `500000` | Maximum script size in bytes. |
| `validator.max_sig_ops` | `BWS_VALIDATOR_MAX_SIG_OPS` | int64 | `4294967295` | Maximum signature operations per transaction. |
| `validator.min_fee_per_kb` | `BWS_VALIDATOR_MIN_FEE_PER_KB` | uint64 | `100` | Minimum fee in satoshis per kilobyte. |

### Webhook (`webhook.*`)

Controls outbound webhook/callback delivery for transaction status updates.

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `webhook.prune_interval` | `BWS_WEBHOOK_PRUNE_INTERVAL` | duration | `1h` | How often to prune old webhook records. |
| `webhook.max_age` | `BWS_WEBHOOK_MAX_AGE` | duration | `24h` | Maximum age of webhook records before pruning. |
| `webhook.max_retries` | `BWS_WEBHOOK_MAX_RETRIES` | int | `10` | Maximum delivery retry attempts per webhook. |

### Auth (`auth.*`)

Optional bearer token authentication for the HTTP API.

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `auth.enabled` | `BWS_AUTH_ENABLED` | bool | `false` | Enable bearer token authentication. |
| `auth.token` | `BWS_AUTH_TOKEN` | string | _(empty)_ | Required bearer token when auth is enabled. |

### P2P (`p2p.*`)

Configuration for the libp2p client that connects to the BSV P2P network.

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `p2p.network` | `BWS_P2P_NETWORK` | string | `main` | Bitcoin network (automatically set from top-level `network`). |
| `p2p.storage_path` | `BWS_P2P_STORAGE_PATH` | string | _(from `storage_path`)_ | Directory for P2P key and peer cache files. Defaults to the top-level `storage_path`. |
| `p2p.msgbus.dht_mode` | `BWS_P2P_MSGBUS_DHT_MODE` | string | `off` | DHT mode for peer discovery: `off`, `client`, `server`, `auto`. |
| `p2p.msgbus.max_connections` | `BWS_P2P_MSGBUS_MAX_CONNECTIONS` | int | `35` | Maximum number of P2P connections. |
| `p2p.msgbus.min_connections` | `BWS_P2P_MSGBUS_MIN_CONNECTIONS` | int | `25` | Minimum number of P2P connections to maintain. |
| `p2p.msgbus.connection_grace_period` | `BWS_P2P_MSGBUS_CONNECTION_GRACE_PERIOD` | duration | `20s` | Grace period before pruning excess connections. |
| `p2p.msgbus.peer_cache_ttl` | `BWS_P2P_MSGBUS_PEER_CACHE_TTL` | duration | `24h` | TTL for cached peer addresses. |

### Chaintracks (`chaintracks.*`)

Configuration for the embedded chaintracks header sync service.

| Key | Env Var | Type | Default | Description |
|-----|---------|------|---------|-------------|
| `chaintracks.mode` | `BWS_CHAINTRACKS_MODE` | string | `embedded` | Chaintracks mode: `embedded` (local header sync) or `remote` (connect to external server). |
| `chaintracks.url` | `BWS_CHAINTRACKS_URL` | string | _(empty)_ | Remote chaintracks server URL. Required when `chaintracks.mode` is `remote`. |
| `chaintracks.storage_path` | `BWS_CHAINTRACKS_STORAGE_PATH` | string | `~/.chaintracks` | Directory for chaintracks header database. When running inside this server, defaults to `{storage_path}/chaintracks`. |
| `chaintracks.bootstrap_url` | `BWS_CHAINTRACKS_BOOTSTRAP_URL` | string | _(empty)_ | URL to bootstrap initial header sync from. |
| `chaintracks.bootstrap_mode` | `BWS_CHAINTRACKS_BOOTSTRAP_MODE` | string | `api` | Bootstrap format: `api` (Gorillanode binary API) or `cdn` (static files). |

## Examples

### Minimal (environment variables only)

```bash
BWS_TERANODE_BROADCAST_URLS=https://arc.taal.com ./wallet-services
```

### Testnet with custom port

```bash
BWS_NETWORK=test \
BWS_SERVER_ADDRESS=:8080 \
BWS_TERANODE_BROADCAST_URLS=https://arc-test.taal.com \
BWS_LOG_LEVEL=debug \
./wallet-services
```

### Config file (`config.yaml`)

```yaml
network: test
log_level: info

server:
  address: ":3011"

teranode:
  broadcast_urls:
    - https://arc.taal.com
  auth_token: "my-token"

database:
  type: sqlite
  sqlite_path: /var/lib/bws/arcade.db

storage_path: /var/lib/bws
```

## Development

```bash
# Build
go build ./...

# Run tests
go test -v ./...

# Build binary
go build -o wallet-services .
```

## License

See [LICENSE](LICENSE) for details.
