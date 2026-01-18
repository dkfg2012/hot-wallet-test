package main

import (
	"context"
	"errors"
	"hot-wallet-test/pkg/rpc"
	"hot-wallet-test/pkg/websocket"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hot-wallet-test/pkg/config"
	"hot-wallet-test/pkg/indexer"
)

func main() {
	cfg, err := config.FromYAML("./config/config.yaml")
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	//pg, err := storage.NewPostgres(ctx, cfg.PostgresDSN)
	//if err != nil {
	//	log.Fatalf("postgres connect error: %v", err)
	//}
	//defer pg.Close()
	//repo := storage.NewBlockchainRepo(pg)

	//redisQ, err := queue.NewRedisStreams(cfg.Redis)
	//if err != nil {
	//	log.Fatalf("redis connect error: %v", err)
	//}
	//defer redisQ.Close()

	ethWs := websocket.NewEthWebsocketClient(cfg.EthRPC.WSURL)
	//polWs := websocket.NewPoLWebsocketClient(cfg.PolRPC.WSURL)

	var wss []websocket.WebsocketClient
	wss = append(wss, ethWs)

	ethRPC := rpc.NewEthRPC(cfg.EthRPC.HTTPURL, cfg.EthRPC.Timeout)
	//polRPC := rpc.NewPolPRC(cfg.PolRPC.HTTPURL, cfg.PolRPC.Timeout)
	//btcRPC := rpc.NewBTCRPC(cfg.BtcRPC.HTTPURL, cfg.BtcRPC.Timeout)
	var rpcs []rpc.RPC
	rpcs = append(rpcs, ethRPC)

	svc := indexer.New(indexer.Deps{
		Cfg: cfg,
		//Store: repo,
		//Queue: redisQ,
		WS:  wss,
		RPC: rpcs,
	})

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("indexer stopped with error: %v", err)
		time.Sleep(250 * time.Millisecond)
		os.Exit(1)
	}
}
