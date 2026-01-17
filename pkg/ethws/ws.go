package ethws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	url string
}

func New(url string) *Client {
	return &Client{url: url}
}

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

func (e *rpcErr) Error() string { return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message) }

type subMsg struct {
	Method string `json:"method"`
	Params struct {
		Subscription string `json:"subscription"`
		Result       struct {
			Number     string `json:"number"`
			Hash       string `json:"hash"`
			ParentHash string `json:"parentHash"`
		} `json:"result"`
	} `json:"params"`
}

// ListenNewHeads connects, subscribes to `newHeads`, and emits heads to the returned channel.
// It will reconnect automatically until ctx is canceled.
func (c *Client) ListenNewHeads(ctx context.Context) <-chan NewHead {
	out := make(chan NewHead, 256)
	go func() {
		defer close(out)
		backoff := 1 * time.Second
		for ctx.Err() == nil {
			if err := c.listenOnce(ctx, out); err != nil && !errors.Is(err, context.Canceled) {
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

func (c *Client) listenOnce(ctx context.Context, out chan<- NewHead) error {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 10 * time.Second,
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
		JSONRPC: "2.0",
		ID:      subID,
		Method:  "eth_subscribe",
		Params:  []interface{}{"newHeads"},
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
	var sr subscribeResp
	if err := json.Unmarshal(msg, &sr); err == nil && sr.ID == subID {
		if sr.Error != nil {
			return sr.Error
		}
	} // if not parseable, we still continue; some nodes may interleave messages.

	for ctx.Err() == nil {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var sm subMsg
		if err := json.Unmarshal(msg, &sm); err != nil {
			continue
		}
		if sm.Method != "eth_subscription" {
			continue
		}
		n, err := parseHexUint64(sm.Params.Result.Number)
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

func parseHexUint64(hexStr string) (uint64, error) {
	if len(hexStr) < 3 || hexStr[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex quantity: %q", hexStr)
	}
	if hexStr == "0x0" {
		return 0, nil
	}
	var out uint64
	_, err := fmt.Sscanf(hexStr, "0x%x", &out)
	return out, err
}

