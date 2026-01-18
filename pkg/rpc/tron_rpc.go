package rpc

import (
	"context"
	"time"
)

type TronRPC struct {
	url     string
	timeout time.Duration
	hc      *HTTPClient
}

func (t TronRPC) BlockNumber(ctx context.Context) (uint64, error) {
	panic("implement me")
}

func (t TronRPC) GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (RawBlock, error) {
	//TODO implement me
	panic("implement me")
}

func (t TronRPC) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (RawBlock, error) {
	//TODO implement me
	panic("implement me")
}

func (t TronRPC) BatchGetBlocksByNumber(ctx context.Context, numbers []uint64, fullTx bool) ([]RawBlock, error) {
	//TODO implement me
	panic("implement me")
}

var _ RPC = (*TronRPC)(nil)

func NewTronRPC(url string, timeout time.Duration, hc *HTTPClient) *TronRPC {
	return &TronRPC{
		url:     url,
		timeout: timeout,
		hc:      NewHTTP(url, timeout),
	}
}
