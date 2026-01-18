package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"hot-wallet-test/pkg/util"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type PolSubResp struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Subscription string `json:"subscription"`
		Result       struct {
			Number     string `json:"number"`
			Hash       string `json:"hash"`
			ParentHash string `json:"parentHash"`
		} `json:"result"`
	} `json:"params"`
}

var _ WebsocketClient = (*POLWebsocketClient)(nil)

type POLWebsocketClient struct {
	url string
}

func NewPoLWebsocketClient(url string) *POLWebsocketClient {
	return &POLWebsocketClient{url: url}
}

// ListenNewHeads implements [WebsocketClient].
func (p *POLWebsocketClient) ListenNewHeads(ctx context.Context) <-chan NewHead {
	out := make(chan NewHead, 256)
	go func() {
		defer close(out)
		backoff := 1 * time.Second
		for ctx.Err() == nil {
			if err := p.listenOnce(ctx, out); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("ws listen error: %v (reconnecting in %s)", err, backoff)
			}
			select {
			case <-time.After(backoff):
				if backoff < 15*time.Second {
					backoff *= 2
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (c *POLWebsocketClient) listenOnce(ctx context.Context, out chan<- NewHead) error {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	var writeMu sync.Mutex
	done := make(chan struct{})
	defer close(done)

	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				writeMu.Lock()
				_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
				writeMu.Unlock()
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	subID := rand.Int63()
	req := subscribeReq{
		ID:     subID,
		Method: "eth_subscribe",
		Params: []interface{}{"newHeads"},
	}
	writeMu.Lock()
	if err := conn.WriteJSON(req); err != nil {
		writeMu.Unlock()
		return err
	}
	writeMu.Unlock()

	// read the subscription response first
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	var fsr FirstSubscriptionResp
	if err := json.Unmarshal(msg, &fsr); err == nil && fsr.ID == subID {
		if fsr.Error != nil {
			return fsr.Error
		}
	} // if not parseable, we still continue; some nodes may interleave messages.

	for ctx.Err() == nil {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var sm PolSubResp
		if err := json.Unmarshal(msg, &sm); err != nil {
			continue
		}
		if sm.Method != "eth_subscription" {
			continue
		}
		n, err := util.ParseHexUint64(sm.Params.Result.Number)
		if err != nil {
			continue
		}
		h := sm.Params.Result.Hash
		ph := sm.Params.Result.ParentHash
		if h == "" || ph == "" {
			continue
		}
		select {
		case out <- NewHead{Number: n, Hash: h, ParentHash: ph}:
		case <-ctx.Done():
			return context.Canceled
		}
	}
	return context.Canceled
}
