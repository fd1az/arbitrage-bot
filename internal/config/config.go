// Package config provides configuration loading and validation.
package config

import (
	"time"

	"github.com/shopspring/decimal"
)

// Config holds all application configuration.
type Config struct {
	Ethereum  EthereumConfig  `mapstructure:"ethereum"`
	Binance   BinanceConfig   `mapstructure:"binance"`
	Arbitrage ArbitrageConfig `mapstructure:"arbitrage"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	UI        UIConfig        `mapstructure:"ui"`
}

// EthereumConfig holds Ethereum node configuration.
type EthereumConfig struct {
	WebSocketURL   string        `mapstructure:"websocket_url"`
	HTTPURL        string        `mapstructure:"http_url"`
	MaxReconnects  int           `mapstructure:"max_reconnects"`  // 0 = infinite
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`
	MaxBackoff     time.Duration `mapstructure:"max_backoff"`
}

// BinanceConfig holds Binance API configuration.
type BinanceConfig struct {
	BaseURL       string `mapstructure:"base_url"`
	HTTPRateLimit int    `mapstructure:"http_rate_limit"` // requests per minute
}

// ArbitrageConfig holds arbitrage detection configuration.
type ArbitrageConfig struct {
	Pairs        []PairConfig      `mapstructure:"pairs"`
	TradeSizes   []decimal.Decimal `mapstructure:"trade_sizes"`
	MinProfitBps decimal.Decimal   `mapstructure:"min_profit_bps"`
	MinProfitUSD decimal.Decimal   `mapstructure:"min_profit_usd"`
}

// PairConfig defines a trading pair.
type PairConfig struct {
	Base  string `mapstructure:"base"`
	Quote string `mapstructure:"quote"`
}

// TelemetryConfig holds observability configuration.
type TelemetryConfig struct {
	Enabled        bool `mapstructure:"enabled"`
	PrometheusPort int  `mapstructure:"prometheus_port"`
}

// UIConfig holds UI configuration.
type UIConfig struct {
	TUI bool `mapstructure:"tui"` // Enable TUI mode
}

// Load loads configuration from file and environment.
func Load(configPath string) (*Config, error) {
	// TODO: Implement Viper-based config loading
	return &Config{}, nil
}
