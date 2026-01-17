package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type EthRPC struct {
	url     string
	timeout time.Duration
	hc      *http.Client
}

var _ RPC = (*EthRPC)(nil)

func NewEthRPC(url string, timeout time.Duration) *EthRPC {
	return &EthRPC{url: url, timeout: timeout}
}

func (c *EthRPC) GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (RawBlock, error) {
	tag := fmt.Sprintf("0x%x", number)
	var raw json.RawMessage
	if err := c.hc.Call(ctx, "eth_getBlockByNumber", []interface{}{tag, fullTx}, &raw); err != nil {
		return RawBlock{}, err
	}
	return DecodeRawBlock(raw)
}

func (c *EthRPC) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (RawBlock, error) {
	var raw json.RawMessage
	if err := c.hc.Call(ctx, "eth_getBlockByHash", []interface{}{hash, fullTx}, &raw); err != nil {
		return RawBlock{}, err
	}
	return DecodeRawBlock(raw)
}

func (c *EthRPC) BatchGetBlocksByNumber(ctx context.Context, numbers []uint64, fullTx bool) ([]RawBlock, error) {
	paramsList := make([]interface{}, 0, len(numbers))
	for _, n := range numbers {
		tag := fmt.Sprintf("0x%x", n)
		paramsList = append(paramsList, []interface{}{tag, fullTx})
	}

	raws, err := c.hc.BatchCall(ctx, "eth_getBlockByNumber", paramsList)
	if err != nil {
		return nil, err
	}
	out := make([]RawBlock, 0, len(raws))
	for _, raw := range raws {
		b, err := DecodeRawBlock(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}
