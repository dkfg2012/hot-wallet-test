package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	Stream   RedisStreamConfig
}

type RedisStreamConfig struct {
	Key    string
	MaxLen int64
}

type EthRPCConfig struct {
	HTTPURL string
	WSURL   string
	Timeout time.Duration
}

type PolRPCConfig struct {
	HTTPURL string
	WSURL   string
	Timeout time.Duration
}

type BtcRPCConfig struct {
	HTTPURL string
	WSURL   string
	Timeout time.Duration
}

type TronRPCConfig struct {
	HTTPURL string
	WSURL   string
	Timeout time.Duration
}

type SyncConfig struct {
	StartBlock    uint64
	BatchSize     int
	Confirmations uint64
}

type Config struct {
	Network     string
	PostgresDSN string
	Redis       RedisConfig
	EthRPC      EthRPCConfig
	PolRPC      PolRPCConfig
	BtcRPC      BtcRPCConfig
	TronRPC     TronRPCConfig
	Sync        SyncConfig
}

func FromEnv() (Config, error) {
	var c Config

	c.Network = getenvDefault("INDEXER_NETWORK", "sepolia")
	c.PostgresDSN = os.Getenv("PG_DSN")
	if c.PostgresDSN == "" {
		return Config{}, errors.New("missing PG_DSN")
	}

	c.EthRPC.HTTPURL = os.Getenv("ETH_RPC_HTTP_URL")
	c.EthRPC.WSURL = os.Getenv("ETH_RPC_WS_URL")
	c.EthRPC.Timeout = mustDuration(getenvDefault("ETH_RPC_TIMEOUT", "15s"))

	c.PolRPC.HTTPURL = os.Getenv("POL_RPC_HTTP_URL")
	c.PolRPC.WSURL = os.Getenv("POL_RPC_WS_URL")
	c.PolRPC.Timeout = mustDuration(getenvDefault("POL_RPC_TIMEOUT", "15s"))

	c.BtcRPC.HTTPURL = os.Getenv("BTC_RPC_HTTP_URL")
	c.BtcRPC.WSURL = os.Getenv("BTC_RPC_WS_URL")
	c.BtcRPC.Timeout = mustDuration(getenvDefault("BTC_RPC_TIMEOUT", "15s"))

	c.TronRPC.HTTPURL = os.Getenv("TRON_RPC_HTTP_URL")
	c.TronRPC.WSURL = os.Getenv("TRON_RPC_WS_URL")
	c.TronRPC.Timeout = mustDuration(getenvDefault("TRON_RPC_TIMEOUT", "15s"))

	// Validate required endpoints based on configured network.
	switch c.Network {
	case "sepolia", "eth", "ethereum":
		if c.EthRPC.HTTPURL == "" {
			return Config{}, errors.New("missing ETH_RPC_HTTP_URL")
		}
		if c.EthRPC.WSURL == "" {
			return Config{}, errors.New("missing ETH_RPC_WS_URL")
		}
	case "amoy", "pol", "polygon":
		if c.PolRPC.HTTPURL == "" {
			return Config{}, errors.New("missing POL_RPC_HTTP_URL")
		}
		if c.PolRPC.WSURL == "" {
			return Config{}, errors.New("missing POL_RPC_WS_URL")
		}
	case "btc", "bitcoin":
		if c.BtcRPC.HTTPURL == "" {
			return Config{}, errors.New("missing BTC_RPC_HTTP_URL")
		}
	case "tron":
		if c.TronRPC.HTTPURL == "" {
			return Config{}, errors.New("missing TRON_RPC_HTTP_URL")
		}
	default:
		// Keep flexible: if user sets an unknown network, just ensure at least one RPC URL is present.
		if c.EthRPC.HTTPURL == "" && c.PolRPC.HTTPURL == "" && c.BtcRPC.HTTPURL == "" && c.TronRPC.HTTPURL == "" {
			return Config{}, errors.New("no RPC HTTP URL configured (ETH/POL/BTC/TRON)")
		}
	}

	c.Redis.Addr = getenvDefault("REDIS_ADDR", "127.0.0.1:6379")
	c.Redis.Password = os.Getenv("REDIS_PASSWORD")
	c.Redis.DB = mustInt(getenvDefault("REDIS_DB", "0"))
	c.Redis.Stream.Key = getenvDefault("REDIS_STREAM_KEY", "blocks:raw")
	c.Redis.Stream.MaxLen = int64(mustInt(getenvDefault("REDIS_STREAM_MAXLEN", "100000")))

	c.Sync.StartBlock = mustUint64(getenvDefault("START_BLOCK", "0"))
	c.Sync.BatchSize = mustInt(getenvDefault("SYNC_BATCH_SIZE", "20"))
	if c.Sync.BatchSize <= 0 {
		return Config{}, fmt.Errorf("SYNC_BATCH_SIZE must be > 0")
	}
	c.Sync.Confirmations = mustUint64(getenvDefault("CONFIRMATIONS", "0"))

	return c, nil
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustInt(v string) int {
	i, err := strconv.Atoi(v)
	if err != nil {
		panic(err)
	}
	return i
}

func mustUint64(v string) uint64 {
	u, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		panic(err)
	}
	return u
}

func mustDuration(v string) time.Duration {
	d, err := time.ParseDuration(v)
	if err != nil {
		panic(err)
	}
	return d
}
