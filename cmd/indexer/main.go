package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hot-wallet-test/pkg/config"
	"hot-wallet-test/pkg/ethrpc"
	"hot-wallet-test/pkg/indexer"
	"hot-wallet-test/pkg/queue"
	"hot-wallet-test/pkg/storage"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pg, err := storage.NewPostgres(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres connect error: %v", err)
	}
	defer pg.Close()

	redisQ, err := queue.NewRedisStreams(cfg.Redis)
	if err != nil {
		log.Fatalf("redis connect error: %v", err)
	}
	defer redisQ.Close()

	rpc := ethrpc.NewHTTP(cfg.EthRPC.HTTPURL, cfg.EthRPC.Timeout)

	svc := indexer.New(indexer.Deps{
		Cfg:   cfg,
		RPC:   rpc,
		Store: pg,
		Queue: redisQ,
	})

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("indexer stopped with error: %v", err)
		time.Sleep(250 * time.Millisecond)
		os.Exit(1)
	}
}

