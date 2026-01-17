package websocket

import "context"

var _ WebsocketClient = (*POLWebsocketClient)(nil)

type POLWebsocketClient struct {
	url string
}

// ListenNewHeads implements [WebsocketClient].
func (p *POLWebsocketClient) ListenNewHeads(ctx context.Context) <-chan NewHead {
	panic("unimplemented")
}
