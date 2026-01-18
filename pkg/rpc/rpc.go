package rpc

import "context"

type RPC interface {
	// BlockNumber returns the latest block height (chain head).
	BlockNumber(ctx context.Context) (uint64, error)
	GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (RawBlock, error)
	GetBlockByHash(ctx context.Context, hash string, fullTx bool) (RawBlock, error)
	BatchGetBlocksByNumber(ctx context.Context, numbers []uint64, fullTx bool) ([]RawBlock, error)
}
