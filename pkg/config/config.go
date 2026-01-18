package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type RedisConfig struct {
	Addr     string            `yaml:"addr"`
	Password string            `yaml:"password"`
	DB       int               `yaml:"db"`
	Stream   RedisStreamConfig `yaml:"stream"`
}

type RedisStreamConfig struct {
	Key    string `yaml:"key"`
	MaxLen int64  `yaml:"max_len"`
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

type yamlRPCNet struct {
	HTTPURL string `yaml:"http_url"`
	WSURL   string `yaml:"ws_url"`
	Timeout string `yaml:"timeout"`
}

type yamlRPCConfig struct {
	Eth  yamlRPCNet `yaml:"eth"`
	Pol  yamlRPCNet `yaml:"pol"`
	Btc  yamlRPCNet `yaml:"btc"`
	Tron yamlRPCNet `yaml:"tron"`
}

type yamlConfig struct {
	Network     string        `yaml:"network"`
	PostgresDSN string        `yaml:"postgres_dsn"`
	Redis       RedisConfig   `yaml:"redis"`
	RPC         yamlRPCConfig `yaml:"rpc"`
	Sync        SyncConfig    `yaml:"sync"`
}

// FromYAML reads config from a YAML file.
//
// Expected shape matches `config/config.yaml`.
func FromYAML(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config yaml: %w", err)
	}

	var yc yamlConfig
	if err := yaml.Unmarshal(b, &yc); err != nil {
		return Config{}, fmt.Errorf("parse config yaml: %w", err)
	}

	// Defaults consistent with FromEnv()
	c := Config{
		Network:     yc.Network,
		PostgresDSN: yc.PostgresDSN,
		Redis:       yc.Redis,
		Sync:        yc.Sync,
	}

	c.EthRPC.HTTPURL = yc.RPC.Eth.HTTPURL
	c.EthRPC.WSURL = yc.RPC.Eth.WSURL
	if yc.RPC.Eth.Timeout != "" {
		c.EthRPC.Timeout = mustDuration(yc.RPC.Eth.Timeout)
	}

	c.PolRPC.HTTPURL = yc.RPC.Pol.HTTPURL
	c.PolRPC.WSURL = yc.RPC.Pol.WSURL
	if yc.RPC.Pol.Timeout != "" {
		c.PolRPC.Timeout = mustDuration(yc.RPC.Pol.Timeout)
	}

	c.BtcRPC.HTTPURL = yc.RPC.Btc.HTTPURL
	c.BtcRPC.WSURL = yc.RPC.Btc.WSURL
	if yc.RPC.Btc.Timeout != "" {
		c.BtcRPC.Timeout = mustDuration(yc.RPC.Btc.Timeout)
	}

	c.TronRPC.HTTPURL = yc.RPC.Tron.HTTPURL
	c.TronRPC.WSURL = yc.RPC.Tron.WSURL
	if yc.RPC.Tron.Timeout != "" {
		c.TronRPC.Timeout = mustDuration(yc.RPC.Tron.Timeout)
	}
	if c.Network == "" {
		c.Network = "sepolia"
	}
	if c.EthRPC.Timeout == 0 {
		c.EthRPC.Timeout = mustDuration("15s")
	}
	if c.PolRPC.Timeout == 0 {
		c.PolRPC.Timeout = mustDuration("15s")
	}
	if c.BtcRPC.Timeout == 0 {
		c.BtcRPC.Timeout = mustDuration("15s")
	}
	if c.TronRPC.Timeout == 0 {
		c.TronRPC.Timeout = mustDuration("15s")
	}
	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6379"
	}
	if c.Redis.Stream.Key == "" {
		c.Redis.Stream.Key = "blocks:raw"
	}
	if c.Redis.Stream.MaxLen == 0 {
		c.Redis.Stream.MaxLen = 100000
	}
	if c.Sync.BatchSize == 0 {
		c.Sync.BatchSize = 20
	}

	return validate(c)
}

// FromFile loads config from either:
// - `*.yaml` / `*.yml` (YAML file)
// - anything else: environment variables (FromEnv)
func FromFile(path string) (Config, error) {
	switch ext := filepath.Ext(path); ext {
	case ".yaml", ".yml":
		return FromYAML(path)
	default:
		return FromEnv()
	}
}

func FromEnv() (Config, error) {
	var c Config

	c.Network = getenvDefault("INDEXER_NETWORK", "sepolia")
	c.PostgresDSN = os.Getenv("PG_DSN")

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

	return validate(c)
}

func validate(c Config) (Config, error) {
	if c.PostgresDSN == "" {
		return Config{}, errors.New("missing PG_DSN / postgres_dsn")
	}

	// Validate required endpoints based on configured network.
	switch c.Network {
	case "sepolia", "eth", "ethereum":
		if c.EthRPC.HTTPURL == "" {
			return Config{}, errors.New("missing ETH_RPC_HTTP_URL / rpc.eth.http_url")
		}
		if c.EthRPC.WSURL == "" {
			return Config{}, errors.New("missing ETH_RPC_WS_URL / rpc.eth.ws_url")
		}
	case "amoy", "pol", "polygon":
		if c.PolRPC.HTTPURL == "" {
			return Config{}, errors.New("missing POL_RPC_HTTP_URL / rpc.pol.http_url")
		}
		if c.PolRPC.WSURL == "" {
			return Config{}, errors.New("missing POL_RPC_WS_URL / rpc.pol.ws_url")
		}
	case "btc", "bitcoin":
		if c.BtcRPC.HTTPURL == "" {
			return Config{}, errors.New("missing BTC_RPC_HTTP_URL / rpc.btc.http_url")
		}
	case "tron":
		if c.TronRPC.HTTPURL == "" {
			return Config{}, errors.New("missing TRON_RPC_HTTP_URL / rpc.tron.http_url")
		}
	default:
		// Keep flexible: if user sets an unknown network, just ensure at least one RPC URL is present.
		if c.EthRPC.HTTPURL == "" && c.PolRPC.HTTPURL == "" && c.BtcRPC.HTTPURL == "" && c.TronRPC.HTTPURL == "" {
			return Config{}, errors.New("no RPC HTTP URL configured (ETH/POL/BTC/TRON)")
		}
	}

	if c.Sync.BatchSize <= 0 {
		return Config{}, fmt.Errorf("SYNC_BATCH_SIZE / sync.batch_size must be > 0")
	}

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
