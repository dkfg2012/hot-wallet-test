package ethrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type HTTPClient struct {
	url     string
	timeout time.Duration
	hc      *http.Client
}

func NewHTTP(url string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &HTTPClient{
		url:     url,
		timeout: timeout,
		hc: &http.Client{
			Timeout: timeout,
		},
	}
}

type rpcReq struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcErr) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

func (c *HTTPClient) Call(ctx context.Context, method string, params interface{}, out interface{}) error {
	id := rand.Int63()
	reqBody, err := json.Marshal(rpcReq{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var rr rpcResp
	if err := json.Unmarshal(raw, &rr); err != nil {
		return fmt.Errorf("decode rpc response: %w (body=%s)", err, string(raw))
	}
	if rr.Error != nil {
		return rr.Error
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(rr.Result, out)
}

func (c *HTTPClient) BatchCall(ctx context.Context, method string, paramsList []interface{}) ([]json.RawMessage, error) {
	reqs := make([]rpcReq, 0, len(paramsList))
	ids := make([]int64, 0, len(paramsList))
	for range paramsList {
		ids = append(ids, rand.Int63())
	}
	for i, params := range paramsList {
		reqs = append(reqs, rpcReq{
			JSONRPC: "2.0",
			ID:      ids[i],
			Method:  method,
			Params:  params,
		})
	}

	reqBody, err := json.Marshal(reqs)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var resps []rpcResp
	if err := json.Unmarshal(raw, &resps); err != nil {
		return nil, fmt.Errorf("decode batch response: %w (body=%s)", err, string(raw))
	}

	byID := make(map[int64]rpcResp, len(resps))
	for _, r := range resps {
		byID[r.ID] = r
	}

	out := make([]json.RawMessage, 0, len(ids))
	for _, id := range ids {
		r, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("missing response for id=%d", id)
		}
		if r.Error != nil {
			return nil, r.Error
		}
		out = append(out, r.Result)
	}
	return out, nil
}

func (c *HTTPClient) BlockNumber(ctx context.Context) (uint64, error) {
	var hexNum string
	if err := c.Call(ctx, "eth_blockNumber", []interface{}{}, &hexNum); err != nil {
		return 0, err
	}
	return parseHexUint64(hexNum)
}

func parseHexUint64(hexStr string) (uint64, error) {
	// expects "0x..."
	if len(hexStr) < 3 || hexStr[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex quantity: %q", hexStr)
	}
	if hexStr == "0x0" {
		return 0, nil
	}
	return strconv.ParseUint(hexStr[2:], 16, 64)
}

