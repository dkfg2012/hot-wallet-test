package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hot-wallet-test/pkg/rpc"
	"hot-wallet-test/pkg/websocket"
	"log"
	"sync"
	"time"

	"hot-wallet-test/pkg/config"
	"hot-wallet-test/pkg/queue"
	"hot-wallet-test/pkg/storage"
)

type Deps struct {
	Cfg   config.Config
	RPC   []rpc.RPC
	WS    []websocket.WebsocketClient
	Store *storage.BlockchainRepo
	Queue *queue.RedisStreams
}

type Service struct {
	cfg   config.Config
	rpcs  []rpc.RPC
	ws    []websocket.WebsocketClient
	store *storage.BlockchainRepo
	queue *queue.RedisStreams
}

func New(d Deps) *Service {
	return &Service{
		cfg:   d.Cfg,
		rpcs:  d.RPC,
		ws:    d.WS,
		store: d.Store,
		queue: d.Queue,
	}
}

func (s *Service) Run(ctx context.Context) error {
	if len(s.rpcs) == 0 {
		return errors.New("no rpc clients configured")
	}
	if len(s.ws) == 0 {
		return errors.New("no websocket clients configured")
	}

	if err := s.store.MustHaveMigrations(ctx); err != nil {
		return err
	}

	// 1) Initial chain sync to (latest - confirmations)
	if err := s.chainSync(ctx); err != nil {
		return err
	}

	// 2) Start head listener and keep up
	heads := s.listenHeads(ctx)

	for {
		select {
		case h, ok := <-heads:
			if !ok {
				return ctx.Err()
			}
			if err := s.onNewHead(ctx, h); err != nil {
				log.Printf("onNewHead error: %v", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Service) chainSync(ctx context.Context) error {
	head, _ := s.store.GetCanonicalHead(ctx)
	start := head.Number + 1
	if start < s.cfg.Sync.StartBlock {
		start = s.cfg.Sync.StartBlock
	}

	latest, err := s.rpcBlockNumber(ctx)
	if err != nil {
		return err
	}
	var target uint64
	if latest > s.cfg.Sync.Confirmations {
		target = latest - s.cfg.Sync.Confirmations
	} else {
		target = 0
	}
	if start > target {
		return nil
	}

	log.Printf("chain sync: from=%d to=%d batch=%d", start, target, s.cfg.Sync.BatchSize)

	for from := start; from <= target; {
		to := from + uint64(s.cfg.Sync.BatchSize) - 1
		if to > target {
			to = target
		}
		nums := make([]uint64, 0, int(to-from+1))
		for n := from; n <= to; n++ {
			nums = append(nums, n)
		}

		blocks, err := s.rpcBatchGetBlocksByNumber(ctx, nums, true)
		if err != nil {
			return err
		}

		for _, b := range blocks {
			// During initial sync, we treat sequential blocks as canonical.
			if err := s.processBlock(ctx, b, "sync", false); err != nil {
				return err
			}
		}
		from = to + 1
	}

	return nil
}

func (s *Service) onNewHead(ctx context.Context, h websocket.NewHead) error {
	// If we are behind by a lot (gap), do a quick sync first.
	canon, _ := s.store.GetCanonicalHead(ctx)
	if canon.Hash != "" && h.Number > canon.Number+1 {
		// Best-effort catch-up to (h.Number - 1)
		catchTo := h.Number - 1
		log.Printf("gap detected: canonical=%d incoming=%d; catching up to %d", canon.Number, h.Number, catchTo)
		if err := s.syncRange(ctx, canon.Number+1, catchTo); err != nil {
			log.Printf("catch-up syncRange error: %v", err)
		}
	}

	b, err := s.rpcGetBlockByHash(ctx, h.Hash, true)
	if err != nil {
		return err
	}
	return s.processBlock(ctx, b, "head", true)
}

func (s *Service) syncRange(ctx context.Context, from, to uint64) error {
	if from > to {
		return nil
	}
	for cur := from; cur <= to; {
		end := cur + uint64(s.cfg.Sync.BatchSize) - 1
		if end > to {
			end = to
		}
		nums := make([]uint64, 0, int(end-cur+1))
		for n := cur; n <= end; n++ {
			nums = append(nums, n)
		}
		blocks, err := s.rpcBatchGetBlocksByNumber(ctx, nums, true)
		if err != nil {
			return err
		}
		for _, b := range blocks {
			if err := s.processBlock(ctx, b, "sync", false); err != nil {
				return err
			}
		}
		cur = end + 1
	}
	return nil
}

// processBlock writes into DB, handles reorg if needed, and emits to Redis stream.
func (s *Service) processBlock(ctx context.Context, b rpc.RawBlock, source string, allowReorg bool) error {
	canon, _ := s.store.GetCanonicalHead(ctx)

	// Normal extension
	if canon.Hash == "" || (b.ParentHash == canon.Hash && b.Number == canon.Number+1) {
		if err := s.store.UpsertBlockWithTxs(ctx, storage.BlockInsert{
			Number:     b.Number,
			Hash:       b.Hash,
			ParentHash: b.ParentHash,
			Source:     source,
			Raw:        b.Raw,
			Canonical:  true,
			TxsRaw:     b.TxsRaw,
		}); err != nil {
			return err
		}
		_, _ = s.queue.PushBlock(ctx, queue.BlockEvent{
			Number:     b.Number,
			Hash:       b.Hash,
			ParentHash: b.ParentHash,
			Source:     source,
			Raw:        b.Raw,
		})
		return nil
	}

	// Out-of-order older blocks: store but don't flip canonical.
	if b.Number <= canon.Number && !allowReorg {
		return s.store.UpsertBlockWithTxs(ctx, storage.BlockInsert{
			Number:     b.Number,
			Hash:       b.Hash,
			ParentHash: b.ParentHash,
			Source:     source,
			Raw:        b.Raw,
			Canonical:  false,
			TxsRaw:     b.TxsRaw,
		})
	}

	// Potential reorg
	if allowReorg {
		return s.handleReorg(ctx, canon, b, source)
	}

	// Otherwise store as side chain
	return s.store.UpsertBlockWithTxs(ctx, storage.BlockInsert{
		Number:     b.Number,
		Hash:       b.Hash,
		ParentHash: b.ParentHash,
		Source:     source,
		Raw:        b.Raw,
		Canonical:  false,
		TxsRaw:     b.TxsRaw,
	})
}

func (s *Service) handleReorg(ctx context.Context, oldHead storage.CanonicalHead, newHead rpc.RawBlock, source string) error {
	ancestorNum, ancestorHash, chain, err := s.findCommonAncestorAndChain(ctx, oldHead, newHead)
	if err != nil {
		return err
	}
	forkPoint := ancestorNum + 1

	blocksRolled, txsRolled, err := s.store.RollbackFrom(ctx, forkPoint)
	if err != nil {
		return err
	}

	detail, _ := json.Marshal(map[string]interface{}{
		"ancestor_number": ancestorNum,
		"ancestor_hash":   ancestorHash,
		"rolled_blocks":   blocksRolled,
		"rolled_txs":      txsRolled,
		"new_chain_len":   len(chain),
	})
	_, _ = s.queue.PushReorg(ctx, queue.ReorgEvent{
		ForkPoint:   forkPoint,
		OldHeadHash: oldHead.Hash,
		NewHeadHash: newHead.Hash,
		DetailJSON:  detail,
	})

	// Apply new canonical chain blocks (from ancestor+1 to new head) in order.
	for _, b := range chain {
		if err := s.store.UpsertBlockWithTxs(ctx, storage.BlockInsert{
			Number:     b.Number,
			Hash:       b.Hash,
			ParentHash: b.ParentHash,
			Source:     source,
			Raw:        b.Raw,
			Canonical:  true,
			TxsRaw:     b.TxsRaw,
		}); err != nil {
			return err
		}
		_, _ = s.queue.PushBlock(ctx, queue.BlockEvent{
			Number:     b.Number,
			Hash:       b.Hash,
			ParentHash: b.ParentHash,
			Source:     source,
			Raw:        b.Raw,
		})
	}

	log.Printf("reorg handled: ancestor=%d forkPoint=%d oldHead=%s newHead=%s", ancestorNum, forkPoint, oldHead.Hash, newHead.Hash)
	return nil
}

// findCommonAncestorAndChain returns:
// - ancestor block number/hash on current canonical chain
// - the new canonical chain blocks from (ancestor+1) .. (newHead) in forward order
func (s *Service) findCommonAncestorAndChain(ctx context.Context, oldHead storage.CanonicalHead, newHead rpc.RawBlock) (uint64, string, []rpc.RawBlock, error) {
	// Walk backwards from new head until we reach a canonical block hash at same height.
	const maxDepth = 256

	chainRev := make([]rpc.RawBlock, 0, 64)
	cur := newHead
	for depth := 0; depth < maxDepth; depth++ {
		if cur.Number == 0 {
			break
		}
		// Compare with canonical hash at this height
		canonHash, ok, err := s.store.GetCanonicalHashByNumber(ctx, cur.Number)
		if err != nil {
			return 0, "", nil, err
		}
		if ok && canonHash == cur.Hash {
			// new head is already canonical at this height (no reorg needed)
			return cur.Number, cur.Hash, nil, nil
		}
		// Check parent height canonical match: if canonical at cur.Number-1 equals cur.ParentHash => ancestor found
		parentCanon, ok, err := s.store.GetCanonicalHashByNumber(ctx, cur.Number-1)
		if err != nil {
			return 0, "", nil, err
		}
		if ok && parentCanon == cur.ParentHash {
			// found ancestor at cur.Number-1
			ancestorNum := cur.Number - 1
			ancestorHash := parentCanon
			chainRev = append(chainRev, cur)
			// reverse to forward order
			chain := make([]rpc.RawBlock, 0, len(chainRev))
			for i := len(chainRev) - 1; i >= 0; i-- {
				chain = append(chain, chainRev[i])
			}
			return ancestorNum, ancestorHash, chain, nil
		}

		chainRev = append(chainRev, cur)

		// fetch parent block by hash for continued walk
		parent, err := s.rpcGetBlockByHash(ctx, cur.ParentHash, true)
		if err != nil {
			return 0, "", nil, fmt.Errorf("fetch parent block %s: %w", cur.ParentHash, err)
		}
		cur = parent

		// give context cancellation a chance on long walks
		if depth%16 == 0 {
			select {
			case <-ctx.Done():
				return 0, "", nil, ctx.Err()
			default:
			}
			time.Sleep(0) // yield
		}
	}

	return 0, "", nil, fmt.Errorf("reorg depth exceeded; could not find common ancestor (maxDepth=%d)", maxDepth)
}

func (s *Service) listenHeads(ctx context.Context) <-chan websocket.NewHead {
	out := make(chan websocket.NewHead, 256)

	var wg sync.WaitGroup
	for _, c := range s.ws {
		if c == nil {
			continue
		}
		wg.Add(1)
		go func(c websocket.WebsocketClient) {
			defer wg.Done()
			in := c.ListenNewHeads(ctx)
			for {
				select {
				case h, ok := <-in:
					if !ok {
						return
					}
					select {
					case out <- h:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func (s *Service) rpcBlockNumber(ctx context.Context) (uint64, error) {
	var lastErr error
	for _, c := range s.rpcs {
		if c == nil {
			continue
		}
		n, err := c.BlockNumber(ctx)
		if err == nil {
			return n, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no rpc clients configured")
	}
	return 0, lastErr
}

func (s *Service) rpcBatchGetBlocksByNumber(ctx context.Context, numbers []uint64, fullTx bool) ([]rpc.RawBlock, error) {
	var lastErr error
	for _, c := range s.rpcs {
		if c == nil {
			continue
		}
		blocks, err := c.BatchGetBlocksByNumber(ctx, numbers, fullTx)
		if err == nil {
			return blocks, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no rpc clients configured")
	}
	return nil, lastErr
}

func (s *Service) rpcGetBlockByHash(ctx context.Context, hash string, fullTx bool) (rpc.RawBlock, error) {
	var lastErr error
	for _, c := range s.rpcs {
		if c == nil {
			continue
		}
		b, err := c.GetBlockByHash(ctx, hash, fullTx)
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no rpc clients configured")
	}
	return rpc.RawBlock{}, lastErr
}
