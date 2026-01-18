package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"hot-wallet-test/pkg/config"

	"github.com/redis/go-redis/v9"
)

type RedisStreams struct {
	rdb    *redis.Client
	stream config.RedisStreamConfig
}

func NewRedisStreams(cfg config.RedisConfig) (*RedisStreams, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return &RedisStreams{rdb: rdb, stream: cfg.Stream}, nil
}

func (q *RedisStreams) Close() error { return q.rdb.Close() }

type BlockEvent struct {
	Number     uint64
	Hash       string
	ParentHash string
	Source     string
	Raw        json.RawMessage
}

func (q *RedisStreams) PushBlock(ctx context.Context, ev BlockEvent) (string, error) {
	fields := map[string]interface{}{
		"type":        "block",
		"number":      fmt.Sprintf("%d", ev.Number),
		"hash":        ev.Hash,
		"parent_hash": ev.ParentHash,
		"source":      ev.Source,
		"raw":         string(ev.Raw),
	}

	args := &redis.XAddArgs{
		Stream: q.stream.Key,
		Values: fields,
	}
	if q.stream.MaxLen > 0 {
		args.MaxLen = q.stream.MaxLen
		args.Approx = true
	}
	return q.rdb.XAdd(ctx, args).Result()
}

type ReorgEvent struct {
	ForkPoint   uint64
	OldHeadHash string
	NewHeadHash string
	DetailJSON  json.RawMessage
}

func (q *RedisStreams) PushReorg(ctx context.Context, ev ReorgEvent) (string, error) {
	fields := map[string]interface{}{
		"type":        "reorg",
		"fork_point":  fmt.Sprintf("%d", ev.ForkPoint),
		"old_head":    ev.OldHeadHash,
		"new_head":    ev.NewHeadHash,
		"detail_json": string(ev.DetailJSON),
	}
	args := &redis.XAddArgs{
		Stream: q.stream.Key,
		Values: fields,
	}
	if q.stream.MaxLen > 0 {
		args.MaxLen = q.stream.MaxLen
		args.Approx = true
	}
	return q.rdb.XAdd(ctx, args).Result()
}
