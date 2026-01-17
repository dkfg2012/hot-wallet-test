package rpc

import (
	"encoding/json"
	"fmt"
)

type RawBlock struct {
	Number     uint64
	Hash       string
	ParentHash string
	Raw        json.RawMessage
	TxsRaw     []json.RawMessage
}

type blockMeta struct {
	Number       string          `json:"number"`
	Hash         string          `json:"hash"`
	ParentHash   string          `json:"parentHash"`
	Transactions json.RawMessage `json:"transactions"`
}

func DecodeRawBlock(raw json.RawMessage) (RawBlock, error) {
	var m blockMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return RawBlock{}, err
	}
	if m.Hash == "" || m.ParentHash == "" || m.Number == "" {
		return RawBlock{}, fmt.Errorf("missing required block fields (hash/parentHash/number)")
	}
	n, err := parseHexUint64(m.Number)
	if err != nil {
		return RawBlock{}, err
	}

	var txs []json.RawMessage
	if len(m.Transactions) > 0 && string(m.Transactions) != "null" {
		// when fullTx=true, it's array of tx objects; otherwise array of hashes.
		_ = json.Unmarshal(m.Transactions, &txs)
	}

	return RawBlock{
		Number:     n,
		Hash:       m.Hash,
		ParentHash: m.ParentHash,
		Raw:        raw,
		TxsRaw:     txs,
	}, nil
}
