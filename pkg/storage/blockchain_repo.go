package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// BlockchainRepo holds all blockchain-specific persistence and query logic.
// It is intentionally decoupled from the concrete Postgres implementation via the DB interface.
type BlockchainRepo struct {
	db DB
}

func NewBlockchainRepo(db DB) *BlockchainRepo {
	return &BlockchainRepo{db: db}
}

type CanonicalHead struct {
	Number uint64
	Hash   string
}

func (r *BlockchainRepo) GetCanonicalHead(ctx context.Context) (CanonicalHead, error) {
	var number int64
	var hash string
	err := r.db.QueryRow(ctx, `
		SELECT number, hash
		FROM raw_blocks
		WHERE canonical = true
		ORDER BY number DESC
		LIMIT 1
	`).Scan(&number, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CanonicalHead{}, nil
		}
		return CanonicalHead{}, err
	}
	if number < 0 {
		number = 0
	}
	return CanonicalHead{Number: uint64(number), Hash: hash}, nil
}

func (r *BlockchainRepo) GetCanonicalHashByNumber(ctx context.Context, number uint64) (string, bool, error) {
	var hash string
	err := r.db.QueryRow(ctx, `
		SELECT hash
		FROM raw_blocks
		WHERE canonical = true AND number = $1
		LIMIT 1
	`, int64(number)).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return hash, true, nil
}

type BlockInsert struct {
	Number     uint64
	Hash       string
	ParentHash string
	Source     string
	Raw        json.RawMessage
	Canonical  bool
	TxsRaw     []json.RawMessage
}

func (r *BlockchainRepo) UpsertBlockWithTxs(ctx context.Context, b BlockInsert) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO raw_blocks (number, hash, parent_hash, source, raw, canonical)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (hash) DO UPDATE SET
			number = EXCLUDED.number,
			parent_hash = EXCLUDED.parent_hash,
			source = EXCLUDED.source,
			raw = EXCLUDED.raw,
			canonical = EXCLUDED.canonical
	`, int64(b.Number), b.Hash, b.ParentHash, b.Source, b.Raw, b.Canonical)
	if err != nil {
		return err
	}

	if len(b.TxsRaw) > 0 {
		for _, txRaw := range b.TxsRaw {
			// Best-effort extract tx hash.
			var tmp struct {
				Hash string `json:"hash"`
			}
			_ = json.Unmarshal(txRaw, &tmp)
			if tmp.Hash == "" {
				continue
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO raw_txs (hash, block_hash, block_number, raw, canonical)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (hash) DO UPDATE SET
					block_hash = EXCLUDED.block_hash,
					block_number = EXCLUDED.block_number,
					raw = EXCLUDED.raw,
					canonical = EXCLUDED.canonical
			`, tmp.Hash, b.Hash, int64(b.Number), txRaw, b.Canonical)
			if err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

// RollbackFrom marks all canonical blocks/txs with number >= forkPoint as non-canonical.
func (r *BlockchainRepo) RollbackFrom(ctx context.Context, forkPoint uint64) (int64, int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	res1, err := tx.Exec(ctx, `
		UPDATE raw_blocks
		SET canonical = false
		WHERE canonical = true AND number >= $1
	`, int64(forkPoint))
	if err != nil {
		return 0, 0, err
	}
	res2, err := tx.Exec(ctx, `
		UPDATE raw_txs
		SET canonical = false
		WHERE canonical = true AND block_number >= $1
	`, int64(forkPoint))
	if err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return res1.RowsAffected(), res2.RowsAffected(), nil
}

func (r *BlockchainRepo) MustHaveMigrations(ctx context.Context) error {
	// A light sanity check so we fail fast if migrations weren't applied.
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema='public' AND table_name='raw_blocks'
		)
	`).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("missing table raw_blocks; did you apply migrations in ./migrate ?")
	}
	return nil
}
