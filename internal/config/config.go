// Package config provides YAML configuration loading
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration
type Config struct {
	Scan     ScanConfig     `yaml:"scan"`
	Streams  StreamsConfig  `yaml:"streams"`
	Weights  WeightsConfig  `yaml:"weights"`
	Thresh   ThresholdConfig `yaml:"thresholds"`
	HMM      HMMConfig      `yaml:"hmm"`
	Pyth     PythConfig     `yaml:"pyth"`
	Cache    CacheConfig    `yaml:"cache"`
	API      APIConfig      `yaml:"api"`
	Deepcoin Credentials    `yaml:"deepcoin,omitempty"`
}

// ScanConfig scanning parameters
type ScanConfig struct {
	TopCoins        int     `yaml:"top_coins"`
	MinVolumeUSD    float64 `yaml:"min_volume_usd"`
	Workers         int     `yaml:"workers"`
	IntervalSeconds int     `yaml:"interval_seconds"`
}

// StreamsConfig live stream settings
type StreamsConfig struct {
	PythSSE           bool `yaml:"pyth_sse"`
	DeepcoinPoll      bool `yaml:"deepcoin_poll"`
	PollIntervalMs    int  `yaml:"poll_interval_ms"`
}

// WeightsConfig signal weights
type WeightsConfig struct {
	Volume     float64 `yaml:"volume"`
	OI         float64 `yaml:"oi"`
	Funding    float64 `yaml:"funding"`
	CVD        float64 `yaml:"cvd"`
	RSI        float64 `yaml:"rsi"`
	BB         float64 `yaml:"bb"`
	Momentum   float64 `yaml:"momentum"`
	HMM        float64 `yaml:"hmm"`
	CBPremium  float64 `yaml:"cb_premium"`
}

// ThresholdConfig score thresholds
type ThresholdConfig struct {
	Hot  float64 `yaml:"hot"`
	Warm float64 `yaml:"warm"`
}

// HMMConfig HMM parameters
type HMMConfig struct {
	Iterations int `yaml:"iterations"`
	States     int `yaml:"states"`
}

// PythConfig Pyth network config
type PythConfig struct {
	Feeds []string `yaml:"feeds"`
}

// CacheConfig cache settings
type CacheConfig struct {
	CandleTTLSeconds int `yaml:"candle_ttl_seconds"`
	MaxEntries       int `yaml:"max_entries"`
}

// APIConfig API timeouts
type APIConfig struct {
	DeepcoinTimeoutSec int `yaml:"deepcoin_timeout_seconds"`
	PythTimeoutSec     int `yaml:"pyth_timeout_seconds"`
	RateLimitPerSec    int `yaml:"rate_limit_per_second"`
}

// Credentials API credentials
type Credentials struct {
	APIKey string `yaml:"api_key"`
	Secret string `yaml:"secret"`
}

// Load reads config from file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	
	// Set defaults
	cfg.SetDefaults()
	
	return &cfg, nil
}

// SetDefaults fills in missing values
func (c *Config) SetDefaults() {
	if c.Scan.TopCoins == 0 {
		c.Scan.TopCoins = 100
	}
	if c.Scan.MinVolumeUSD == 0 {
		c.Scan.MinVolumeUSD = 200000
	}
	if c.Scan.Workers == 0 {
		c.Scan.Workers = 10
	}
	if c.Thresh.Hot == 0 {
		c.Thresh.Hot = 70
	}
	if c.Thresh.Warm == 0 {
		c.Thresh.Warm = 50
	}
	if c.HMM.Iterations == 0 {
		c.HMM.Iterations = 80
	}
	if c.Cache.CandleTTLSeconds == 0 {
		c.Cache.CandleTTLSeconds = 30
	}
	if c.Cache.MaxEntries == 0 {
		c.Cache.MaxEntries = 200
	}
}

// Save writes config to file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
