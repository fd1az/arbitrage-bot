// Package config provides configuration loading and validation.
package config

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Ethereum  EthereumConfig  `mapstructure:"ethereum"`
	Binance   BinanceConfig   `mapstructure:"binance"`
	Uniswap   UniswapConfig   `mapstructure:"uniswap"`
	Arbitrage ArbitrageConfig `mapstructure:"arbitrage"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
}

// AppConfig holds general application settings.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
	LogLevel    string `mapstructure:"log_level"`
}

// EthereumConfig holds Ethereum node configuration.
type EthereumConfig struct {
	WebSocketURL   string        `mapstructure:"websocket_url"`
	HTTPURL        string        `mapstructure:"http_url"`
	ChainID        uint64        `mapstructure:"chain_id"`
	MaxReconnects  int           `mapstructure:"max_reconnects"`
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`
	MaxBackoff     time.Duration `mapstructure:"max_backoff"`
}

// BinanceConfig holds Binance API configuration.
type BinanceConfig struct {
	WebSocketURL string        `mapstructure:"websocket_url"` // wss://stream.binance.com:9443 or wss://stream.binance.us:9443 for US
	Symbols      []string      `mapstructure:"symbols"`
	DepthSpeedMs int           `mapstructure:"depth_speed_ms"`
	StaleTimeout time.Duration `mapstructure:"stale_timeout"`
}

// UniswapConfig holds Uniswap V3 contract addresses.
type UniswapConfig struct {
	QuoterAddress  string `mapstructure:"quoter_address"`
	RouterAddress  string `mapstructure:"router_address"`
	FactoryAddress string `mapstructure:"factory_address"`
	DefaultFeeTier int    `mapstructure:"default_fee_tier"`
}

// QuoterAddressHex returns the quoter address as common.Address.
func (c *UniswapConfig) QuoterAddressHex() common.Address {
	return common.HexToAddress(c.QuoterAddress)
}

// RouterAddressHex returns the router address as common.Address.
func (c *UniswapConfig) RouterAddressHex() common.Address {
	return common.HexToAddress(c.RouterAddress)
}

// FactoryAddressHex returns the factory address as common.Address.
func (c *UniswapConfig) FactoryAddressHex() common.Address {
	return common.HexToAddress(c.FactoryAddress)
}

// ArbitrageConfig holds arbitrage detection configuration.
type ArbitrageConfig struct {
	Pairs        []string  `mapstructure:"pairs"`
	TradeSizes   []float64 `mapstructure:"trade_sizes"`
	MinProfitBps float64   `mapstructure:"min_profit_bps"`
	MinProfitUSD float64   `mapstructure:"min_profit_usd"`
	TUIMode      bool      `mapstructure:"-"` // Set at runtime, not from config file
}

// TradeSizesDecimal returns trade sizes as decimal.Decimal slice.
func (c *ArbitrageConfig) TradeSizesDecimal() []decimal.Decimal {
	result := make([]decimal.Decimal, len(c.TradeSizes))
	for i, s := range c.TradeSizes {
		result[i] = decimal.NewFromFloat(s)
	}
	return result
}

// MinProfitBpsDecimal returns min profit bps as decimal.Decimal.
func (c *ArbitrageConfig) MinProfitBpsDecimal() decimal.Decimal {
	return decimal.NewFromFloat(c.MinProfitBps)
}

// MinProfitUSDDecimal returns min profit USD as decimal.Decimal.
func (c *ArbitrageConfig) MinProfitUSDDecimal() decimal.Decimal {
	return decimal.NewFromFloat(c.MinProfitUSD)
}

// TelemetryConfig holds observability configuration.
type TelemetryConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	ServiceName    string `mapstructure:"service_name"`
	OTLPEndpoint   string `mapstructure:"otlp_endpoint"`
	OTLPHeaders    string `mapstructure:"otlp_headers"`
	PrometheusPort int    `mapstructure:"prometheus_port"`
}

// Load loads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// Environment variables
	v.SetEnvPrefix("ARB")
	v.AutomaticEnv()

	// Bind env vars to config keys
	bindEnvVars(v)

	// Set defaults
	setDefaults(v)

	// Read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found is OK, use env vars
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func bindEnvVars(v *viper.Viper) {
	// App
	v.BindEnv("app.name", "ARB_APP_NAME", "SERVICE_NAME")
	v.BindEnv("app.environment", "ARB_ENVIRONMENT", "ENVIRONMENT")
	v.BindEnv("app.log_level", "ARB_LOG_LEVEL", "LOG_LEVEL")

	// Ethereum
	v.BindEnv("ethereum.websocket_url", "ARB_ETH_WS_URL", "ETH_WS_URL")
	v.BindEnv("ethereum.http_url", "ARB_ETH_HTTP_URL", "ETH_HTTP_URL")
	v.BindEnv("ethereum.chain_id", "ARB_ETH_CHAIN_ID", "ETH_CHAIN_ID")

	// Binance
	v.BindEnv("binance.websocket_url", "ARB_BINANCE_WS_URL", "BINANCE_WS_URL")
	v.BindEnv("binance.symbols", "ARB_BINANCE_SYMBOLS", "BINANCE_SYMBOLS")

	// Uniswap
	v.BindEnv("uniswap.quoter_address", "ARB_UNISWAP_QUOTER", "UNISWAP_QUOTER")
	v.BindEnv("uniswap.router_address", "ARB_UNISWAP_ROUTER", "UNISWAP_ROUTER")
	v.BindEnv("uniswap.factory_address", "ARB_UNISWAP_FACTORY", "UNISWAP_FACTORY")

	// Arbitrage
	v.BindEnv("arbitrage.pairs", "ARB_PAIRS")
	v.BindEnv("arbitrage.min_profit_bps", "ARB_MIN_PROFIT_BPS")
	v.BindEnv("arbitrage.min_profit_usd", "ARB_MIN_PROFIT_USD")

	// Telemetry
	v.BindEnv("telemetry.enabled", "ARB_OTEL_ENABLED", "OTEL_ENABLED")
	v.BindEnv("telemetry.service_name", "ARB_OTEL_SERVICE_NAME", "OTEL_SERVICE_NAME")
	v.BindEnv("telemetry.otlp_endpoint", "ARB_OTEL_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT")
}

func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.name", "arbitrage-bot")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.log_level", "info")

	// Ethereum defaults
	v.SetDefault("ethereum.chain_id", 1)
	v.SetDefault("ethereum.max_reconnects", 0) // infinite
	v.SetDefault("ethereum.initial_backoff", "1s")
	v.SetDefault("ethereum.max_backoff", "30s")

	// Binance defaults
	v.SetDefault("binance.websocket_url", "wss://stream.binance.com:9443")
	v.SetDefault("binance.symbols", []string{"ETHUSDC"})
	v.SetDefault("binance.depth_speed_ms", 100)
	v.SetDefault("binance.stale_timeout", "5s")

	// Uniswap V3 Mainnet defaults
	v.SetDefault("uniswap.quoter_address", "0x61fFE014bA17989E743c5F6cB21bF9697530B21e")
	v.SetDefault("uniswap.router_address", "0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45")
	v.SetDefault("uniswap.factory_address", "0x1F98431c8aD98523631AE4a59f267346ea31F984")
	v.SetDefault("uniswap.default_fee_tier", 3000) // 0.3%

	// Arbitrage defaults
	v.SetDefault("arbitrage.pairs", []string{"ETH-USDC"})
	v.SetDefault("arbitrage.trade_sizes", []float64{0.1, 0.5, 1.0})
	v.SetDefault("arbitrage.min_profit_bps", 10)
	v.SetDefault("arbitrage.min_profit_usd", 5)

	// Telemetry defaults
	v.SetDefault("telemetry.enabled", false)
	v.SetDefault("telemetry.service_name", "arbitrage-bot")
	v.SetDefault("telemetry.prometheus_port", 9090)
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Ethereum.WebSocketURL == "" {
		return fmt.Errorf("ethereum.websocket_url is required")
	}
	if c.Ethereum.HTTPURL == "" {
		return fmt.Errorf("ethereum.http_url is required")
	}
	if !common.IsHexAddress(c.Uniswap.QuoterAddress) {
		return fmt.Errorf("invalid uniswap.quoter_address: %s", c.Uniswap.QuoterAddress)
	}
	if !common.IsHexAddress(c.Uniswap.RouterAddress) {
		return fmt.Errorf("invalid uniswap.router_address: %s", c.Uniswap.RouterAddress)
	}
	if len(c.Binance.Symbols) == 0 {
		return fmt.Errorf("binance.symbols cannot be empty")
	}
	return nil
}
