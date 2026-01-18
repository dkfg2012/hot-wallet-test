package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"hot-wallet-test/pkg/util"
	"time"
)

type PolPRC struct {
	url     string
	timeout time.Duration
	hc      *HTTPClient
}

var _ RPC = (*PolPRC)(nil)

func NewPolPRC(url string, timeout time.Duration) *PolPRC {
	return &PolPRC{
		url:     url,
		timeout: timeout,
		hc:      NewHTTP(url, timeout),
	}
}

func (c *PolPRC) BlockNumber(ctx context.Context) (uint64, error) {
	var resp string
	if err := c.hc.Call(ctx, "eth_blockNumber", []interface{}{}, &resp); err != nil {
		return 0, err
	}
	return util.ParseHexUint64(resp)
}

func (c *PolPRC) GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (RawBlock, error) {
	tag := fmt.Sprintf("0x%x", number)
	var resp json.RawMessage
	if err := c.hc.Call(ctx, "eth_getBlockByNumber", []interface{}{tag, fullTx}, &resp); err != nil {
		return RawBlock{}, err
	}
	return DecodeRawBlock(resp)
}

func (c *PolPRC) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (RawBlock, error) {
	var resp json.RawMessage
	if err := c.hc.Call(ctx, "eth_getBlockByHash", []interface{}{hash, fullTx}, &resp); err != nil {
		return RawBlock{}, err
	}
	return DecodeRawBlock(resp)
}

func (c *PolPRC) BatchGetBlocksByNumber(ctx context.Context, numbers []uint64, fullTx bool) ([]RawBlock, error) {
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
