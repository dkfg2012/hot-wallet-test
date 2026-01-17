package websocket

import "context"

type NewHead struct {
	Number     uint64
	Hash       string
	ParentHash string
}

type subscribeReq struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int64         `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type subscribeResp struct {
	JSONRPC string  `json:"jsonrpc"`
	ID      int64   `json:"id"`
	Result  string  `json:"result"`
	Error   *rpcErr `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type WebsocketClient interface {
	ListenNewHeads(ctx context.Context) <-chan NewHead
}
