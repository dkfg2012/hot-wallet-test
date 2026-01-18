package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type BTCRPC struct {
	url     string
	timeout time.Duration
	hc      *HTTPClient
}

var _ RPC = (*BTCRPC)(nil)

func NewBTCRPC(url string, timeout time.Duration) *BTCRPC {
	return &BTCRPC{
		url:     url,
		timeout: timeout,
		hc:      NewHTTP(url, timeout),
	}
}

type btcBlockMeta struct {
	Hash              string          `json:"hash"`
	Height            uint64          `json:"height"`
	PreviousBlockHash string          `json:"previousblockhash"`
	Tx                json.RawMessage `json:"tx"`
}

func decodeBTCRawBlock(raw json.RawMessage) (RawBlock, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return RawBlock{}, fmt.Errorf("empty btc block response")
	}

	var m btcBlockMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return RawBlock{}, err
	}
	if m.Hash == "" {
		return RawBlock{}, fmt.Errorf("missing required btc block field (hash)")
	}
	// height=0 is valid for genesis blocks; previousblockhash may be absent.
	if m.Height > 0 && m.PreviousBlockHash == "" {
		return RawBlock{}, fmt.Errorf("missing required btc block field (previousblockhash)")
	}

	var txs []json.RawMessage
	if len(m.Tx) > 0 && string(m.Tx) != "null" {
		_ = json.Unmarshal(m.Tx, &txs)
	}

	return RawBlock{
		Number:     m.Height,
		Hash:       m.Hash,
		ParentHash: m.PreviousBlockHash,
		Raw:        raw,
		TxsRaw:     txs,
	}, nil
}

// Bitcoin Core / Alchemy Bitcoin RPC:
// - getblockhash(height) -> block hash
// - getblock(hash, verbosity) -> block object
//
// verbosity:
// - 1: tx is array of txids (strings)
// - 2: tx is array of tx objects
func (c *BTCRPC) GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (RawBlock, error) {
	var hash string
	if err := c.hc.Call(ctx, "getblockhash", []interface{}{number}, &hash); err != nil {
		return RawBlock{}, err
	}
	return c.GetBlockByHash(ctx, hash, fullTx)
}

func (c *BTCRPC) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (RawBlock, error) {
	verbosity := 1
	if fullTx {
		verbosity = 2
	}

	var resp json.RawMessage
	if err := c.hc.Call(ctx, "getblock", []interface{}{hash, verbosity}, &resp); err != nil {
		return RawBlock{}, err
	}
	return decodeBTCRawBlock(resp)
}

func (c *BTCRPC) BatchGetBlocksByNumber(ctx context.Context, numbers []uint64, fullTx bool) ([]RawBlock, error) {
	if len(numbers) == 0 {
		return nil, nil
	}

	verbosity := 1
	if fullTx {
		verbosity = 2
	}

	// 1) Batch getblockhash for all heights
	hashParams := make([]interface{}, 0, len(numbers))
	for _, n := range numbers {
		hashParams = append(hashParams, []interface{}{n})
	}
	hashRaw, err := c.hc.BatchCall(ctx, "getblockhash", hashParams)
	if err != nil {
		return nil, err
	}

	hashes := make([]string, 0, len(hashRaw))
	for _, r := range hashRaw {
		var h string
		if err := json.Unmarshal(r, &h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}

	// 2) Batch getblock for all hashes
	blockParams := make([]interface{}, 0, len(hashes))
	for _, h := range hashes {
		blockParams = append(blockParams, []interface{}{h, verbosity})
	}
	blockRaw, err := c.hc.BatchCall(ctx, "getblock", blockParams)
	if err != nil {
		return nil, err
	}

	out := make([]RawBlock, 0, len(blockRaw))
	for _, raw := range blockRaw {
		b, err := decodeBTCRawBlock(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}
