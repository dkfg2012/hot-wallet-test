## Hot Wallet Test - Indexer (Chain Syncer + Head Listener + Reorg)

### What this service does

- **Chain Syncer**: via HTTP JSON-RPC batch requests, downloads historical blocks and stores raw data into Postgres.
- **Head Listener**: via WebSocket `eth_subscribe` (`newHeads`), listens new blocks and fetches full blocks via HTTP.
- **Reorg handling**: detects forks, finds common ancestor, **rolls back** invalid canonical blocks/txs in DB (marks `canonical=false`), then applies the new canonical chain.
- **Block Queue**: pushes raw blocks (and reorg events) into **Redis Streams** for downstream parsers.

### Prerequisites

- Go 1.22+
- Docker (for local Postgres/Redis)
- Sepolia RPC endpoints (HTTP + WS)

### Local run

Start infra:

```bash
docker compose up -d
```

Apply migrations (example):

```bash
psql "postgres://postgres:postgres@127.0.0.1:5432/hotwallet?sslmode=disable" -f migrate/001_init.sql
```

Set env (copy from `config/env.example`), then run:

```bash
go run ./cmd/indexer
```

### Redis Streams payload

Stream key: `REDIS_STREAM_KEY` (default `blocks:raw`)

- `type=block`: fields include `number`, `hash`, `parent_hash`, `source`, `raw`
- `type=reorg`: fields include `fork_point`, `old_head`, `new_head`, `detail_json`

