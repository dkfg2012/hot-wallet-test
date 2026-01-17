CREATE TABLE IF NOT EXISTS raw_blocks (
  id BIGSERIAL PRIMARY KEY,
  number BIGINT NOT NULL,
  hash TEXT NOT NULL,
  parent_hash TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT 'unknown',
  raw JSONB NOT NULL,
  canonical BOOLEAN NOT NULL DEFAULT TRUE,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_raw_blocks_hash ON raw_blocks(hash);
CREATE INDEX IF NOT EXISTS idx_raw_blocks_number ON raw_blocks(number);
CREATE INDEX IF NOT EXISTS idx_raw_blocks_canonical_number ON raw_blocks(canonical, number DESC);

CREATE TABLE IF NOT EXISTS raw_txs (
  id BIGSERIAL PRIMARY KEY,
  hash TEXT NOT NULL,
  block_hash TEXT NOT NULL,
  block_number BIGINT NOT NULL,
  raw JSONB NOT NULL,
  canonical BOOLEAN NOT NULL DEFAULT TRUE,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_raw_txs_hash ON raw_txs(hash);
CREATE INDEX IF NOT EXISTS idx_raw_txs_block_number ON raw_txs(block_number);
CREATE INDEX IF NOT EXISTS idx_raw_txs_canonical_block_number ON raw_txs(canonical, block_number DESC);

