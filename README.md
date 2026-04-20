# ⚡ PrePump Scanner v4

A high-performance cryptocurrency pre-pump detection scanner written in Go. PrePump analyzes multiple market signals in real-time to identify coins showing early signs of potential price pumps before they happen.

![Go Version](https://img.shields.io/badge/Go-1.24.2-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-blue.svg)
![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)

---

## 📖 Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [TUI Keyboard Shortcuts](#tui-keyboard-shortcuts)
- [Signal Explanations](#signal-explanations)
- [Understanding the Score](#understanding-the-score)
- [Project Structure](#project-structure)
- [Development](#development)
- [Docker](#docker)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)
- [Disclaimer](#disclaimer)

---

## 🎯 Overview

PrePump Scanner is designed for traders and crypto enthusiasts who want to identify potential pump opportunities early. It combines:

- **Real-time price streams** from Pyth Network and Deepcoin
- **Machine Learning** using a pure Go Hidden Markov Model (HMM) for market regime detection
- **Multi-factor technical analysis** with 9 weighted signals
- **Beautiful terminal UI** with live updates using Bubble Tea

The scanner continuously monitors the top cryptocurrencies by volume, calculates composite pump scores, and presents actionable insights in an intuitive terminal interface.

---

## ✨ Features

| Feature | Description |
|---------|-------------|
| 🔄 **Real-time Streaming** | Pyth Network SSE + Deepcoin WebSocket for sub-second price updates |
| 🧠 **Machine Learning** | Pure Go HMM (Hidden Markov Model) for BULL/BEAR/SIDEWAYS regime detection |
| 📊 **9-Factor Analysis** | Volume, OI, Funding, CVD, RSI, Bollinger Bands, Momentum, HMM, Coinbase Premium |
| 🎨 **Terminal UI** | Beautiful Bubble Tea interface with live price flash animations |
| ⚡ **High Performance** | Concurrent goroutines for maximum throughput |
| 🔧 **Configurable** | YAML config + CLI flags for custom scanning parameters |
| 📈 **Live Stats** | Background recalculation every 30s with fresh 1m candles |
| 🌐 **Multi-Exchange** | Supports both spot and futures markets |

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Main Application (cmd/prepump)                    │
│                                                                          │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────┐   │
│  │  Pyth SSE Stream │  │  Deepcoin WS     │  │  Live Stats Loop     │   │
│  │  (Price Ticks)   │  │  (Tickers/Orders)│  │  (30s Recalc)        │   │
│  └────────┬─────────┘  └────────┬─────────┘  └──────────┬───────────┘   │
│           │                     │                       │               │
│           └─────────────────────┼───────────────────────┘               │
│                                 │                                       │
│                                 ▼                                       │
│                    ┌────────────────────────┐                           │
│                    │   Scanner Engine       │                           │
│                    │   (Signal Calculation) │                           │
│                    └───────────┬────────────┘                           │
│                                │                                        │
│                                ▼                                        │
│                    ┌────────────────────────┐                           │
│                    │   Bubble Tea TUI       │                           │
│                    │   (Live Display)       │                           │
│                    └────────────────────────┘                           │
└─────────────────────────────────────────────────────────────────────────┘

External Services:
┌─────────────┐    ┌─────────────┐    ┌─────────────────┐
│ Pyth Network│    │  Deepcoin   │    │ Fear & Greed API│
│   (SSE)     │    │   (REST/WS) │    │   (Alternative) │
└─────────────┘    └─────────────┘    └─────────────────┘
```

### Data Flow

1. **Ingestion**: Pyth SSE and Deepcoin APIs feed real-time market data
2. **Processing**: Scanner engine calculates 9 signals per coin
3. **Scoring**: Weighted composite score (0-100) determines pump potential
4. **Display**: TUI renders sorted list with live price updates
5. **Refresh**: Background goroutines update stats every 30 seconds

---

## 📋 Prerequisites

Before installing PrePump, ensure you have the following:

### Required

| Requirement | Version | Installation Link |
|-------------|---------|-------------------|
| **Go** | 1.24.2 or higher | [golang.org/dl](https://golang.org/dl/) |
| **Git** | Any recent version | [git-scm.com](https://git-scm.com/) |

### Optional (for development)

| Tool | Purpose |
|------|---------|
| **Make** | Build automation |
| **Docker** | Containerized deployment |

### Verify Installation

```bash
# Check Go version (must be 1.24.2+)
go version

# Check Git
git --version

# Check Make (optional)
make --version
```

---

## 🚀 Installation

### Method 1: Clone & Build (Recommended)

```bash
# 1. Clone the repository
git clone https://github.com/YOUR_USERNAME/prepump.git
cd prepump

# 2. Download dependencies
go mod download

# 3. Build the binary
go build -o prepump ./cmd/prepump

# Or using Make
make build

# 4. Run the scanner
./prepump
```

### Method 2: Go Install

```bash
# Install directly from source
go install github.com/YOUR_USERNAME/prepump/cmd/prepump@latest

# Run from GOPATH/bin
$GOPATH/bin/prepump
```

### Method 3: Docker

```bash
# Build the Docker image
docker build -t prepump:latest .

# Run the container
docker run -it --rm prepump:latest
```

### Method 4: Pre-built Binary

Check the [Releases](https://github.com/YOUR_USERNAME/prepump/releases) page for pre-compiled binaries for your platform.

---

## ⚙️ Configuration

### Configuration File

Copy the example configuration and customize it:

```bash
cp config.yaml config.local.yaml
```

Edit `config.local.yaml`:

```yaml
# PrePump Scanner Configuration

# ─── Scanning Parameters ──────────────────────────────────────────
scan:
  top_coins: 100              # Number of top coins to scan by volume
  min_volume_usd: 200000      # Minimum 24h trading volume (USD)
  workers: 10                 # Concurrent worker goroutines
  interval_seconds: 60        # Full scan interval

# ─── Live Data Streams ────────────────────────────────────────────
streams:
  pyth_sse: true              # Enable Pyth Network SSE stream
  deepcoin_poll: true         # Enable Deepcoin polling
  poll_interval_ms: 100       # Polling interval in milliseconds

# ─── Signal Weights (should sum to ~100) ──────────────────────────
weights:
  volume: 15      # Volume spike detection
  oi: 12          # Open Interest changes
  funding: 15     # Funding rate anomalies
  cvd: 12         # Cumulative Volume Delta
  rsi: 8          # RSI divergence signals
  bb: 8           # Bollinger Bands compression
  momentum: 5     # Price momentum
  hmm: 15         # HMM regime detection
  cb_premium: 10  # Coinbase premium analysis

# ─── Alert Thresholds ─────────────────────────────────────────────
thresholds:
  hot: 70         # Score >= 70 triggers HOT alert
  warm: 50        # Score >= 50 triggers WARM alert

# ─── HMM Configuration ────────────────────────────────────────────
hmm:
  iterations: 80  # Training iterations
  states: 3       # Number of hidden states (BULL/BEAR/SIDEWAYS)

# ─── Pyth Price Feeds ─────────────────────────────────────────────
pyth:
  feeds:
    - BTC
    - ETH
    - SOL
    - BNB
    - XRP
    - DOGE
    - ADA
    - AVAX
    - MATIC
    - LINK
    - DOT
    - UNI

# ─── Cache Settings ───────────────────────────────────────────────
cache:
  candle_ttl_seconds: 30      # Candle data time-to-live
  max_entries: 200            # Maximum cache entries

# ─── API Settings ─────────────────────────────────────────────────
api:
  deepcoin_timeout_seconds: 15
  pyth_timeout_seconds: 10
  rate_limit_per_second: 20
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PREPUMP_CONFIG` | Path to config file | `./config.yaml` |
| `DEEPCOIN_API_KEY` | Deepcoin API key | (empty) |
| `DEEPCOIN_API_SECRET` | Deepcoin API secret | (empty) |

---

## 💻 Usage

### Basic Usage

```bash
# Run with default settings
./prepump

# Run with custom config
./prepump -config config.local.yaml
```

### Command-Line Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-top` | int | 100 | Scan top N coins by volume |
| `-min-vol` | float | 200000 | Minimum 24h USD volume |
| `-workers` | int | 10 | Concurrent analysis goroutines |
| `-no-stream` | bool | false | Disable Pyth SSE live price stream |
| `-no-live-stats` | bool | false | Disable live signal recalculation |
| `-deepcoin-key` | string | "" | Deepcoin API key |
| `-deepcoin-secret` | string | "" | Deepcoin API secret |
| `-config` | string | config.yaml | Path to configuration file |

### Examples

```bash
# Scan top 50 coins with minimum volume of $500k
./prepump -top=50 -min-vol=500000

# Run with 20 workers for faster scanning
./prepump -workers=20

# Disable live streams for lower resource usage
./prepump -no-stream -no-live-stats

# Use Deepcoin API credentials
./prepump -deepcoin-key=YOUR_KEY -deepcoin-secret=YOUR_SECRET

# Combined example
./prepump -top=30 -workers=15 -min-vol=1000000 -no-live-stats
```

---

## 🎹 TUI Keyboard Shortcuts

### Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `g` / `Home` | Jump to top |
| `G` / `End` | Jump to bottom |
| `Ctrl+U` | Scroll up half page |
| `Ctrl+D` | Scroll down half page |

### Actions

| Key | Action |
|-----|--------|
| `Enter` | Open coin detail view |
| `d` | Open DexScreener in browser |
| `r` | Force rescan immediately |
| `s` | Toggle sort order |
| `f` | Toggle filters |
| `?` | Show help overlay |

### Application

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit application |
| `ESC` / `Backspace` | Back to coin list (from detail view) |
| `Tab` | Switch panels (if applicable) |

---

## 📊 Signal Explanations

### Signal Breakdown

| Signal | Weight | Description | Calculation |
|--------|--------|-------------|-------------|
| **Volume** | 15% | Detects unusual volume spikes | Current volume vs 50-period moving average |
| **Open Interest (OI)** | 12% | Tracks futures market interest | OI change rate over 1h/4h periods |
| **Funding Rate** | 15% | Identifies leveraged positioning | Perpetual funding rate anomalies |
| **CVD** | 12% | Buy/sell pressure analysis | Cumulative Volume Delta divergence |
| **RSI** | 8% | Momentum with divergence | 14-period RSI with bullish divergence detection |
| **Bollinger Bands** | 8% | Volatility compression | BB width percentile (squeeze detection) |
| **Momentum** | 5% | Short-term price velocity | 5-min and 15-min ROC (Rate of Change) |
| **HMM** | 15% | ML regime classification | Hidden Markov Model state probability |
| **Coinbase Premium** | 10% | Institutional flow indicator | Price premium vs other exchanges |

### HMM States

The Hidden Markov Model classifies market regimes into three states:

| State | Color | Description |
|-------|-------|-------------|
| **BULL** | 🟢 Green | High probability of upward movement |
| **BEAR** | 🔴 Red | High probability of downward movement |
| **SIDEWAYS** | 🟡 Yellow | Consolidation/ranging market |

---

## 🎯 Understanding the Score

### Pump Score Calculation

```
PumpScore = Σ(Signal_i × Weight_i)

Where:
- Signal_i = Normalized score for each signal (0-100)
- Weight_i = Configured weight for each signal
```

### Score Interpretation

| Score Range | Status | Color | Action |
|-------------|--------|-------|--------|
| **70-100** | 🔥 HOT | Red | High pump probability - monitor closely |
| **50-69** | ⚠️ WARM | Yellow | Moderate signals - watch for confirmation |
| **0-49** | 📊 NORMAL | Default | No significant signals detected |

### Example Output

```
┌─────────────────────────────────────────────────────────────────┐
│  PREPUMP SCANNER v4                              BTC: $67,432  │
│  F&G: 65 (Greed)                                 Scan: 100 coins│
├─────────────────────────────────────────────────────────────────┤
│  COIN      PRICE      SCORE   VOL    OI    FUND   HMM    MKT   │
├─────────────────────────────────────────────────────────────────┤
│  SOL      $145.32    🔥 78   +45%   +12%  +0.01%  BULL   F&S   │
│  AVAX     $38.21     ⚠️  62   +28%   +8%   -0.02%  SIDE   F    │
│  DOT      $7.45      📊  45   +12%   -3%   +0.00%  BEAR   S    │
└─────────────────────────────────────────────────────────────────┘
```

---

## 📁 Project Structure

```
prepump/
├── cmd/
│   └── prepump/
│       └── main.go              # Application entry point
├── internal/
│   ├── scanner/
│   │   ├── engine.go            # Core signal calculation engine
│   │   └── engine_test.go       # Scanner tests
│   ├── tui/
│   │   └── tui.go               # Bubble Tea terminal UI
│   ├── pyth/
│   │   └── client.go            # Pyth Network SSE client
│   ├── deepcoin/
│   │   └── client.go            # Deepcoin API client
│   ├── hmm/
│   │   ├── engine.go            # HMM implementation
│   │   └── engine_test.go       # HMM tests
│   ├── cache/
│   │   └── cache.go             # Candle data caching
│   ├── config/
│   │   └── config.go            # Configuration loader
│   ├── metrics/
│   │   └── metrics.go           # Performance metrics
│   └── ratelimit/
│       └── ratelimit.go         # API rate limiting
├── config.yaml                  # Default configuration
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── Makefile                     # Build automation
├── Dockerfile                   # Docker container definition
├── backup_original.sh           # Backup utility script
├── README.md                    # This file
└── prepump                      # Compiled binary (after build)
```

---

## 👨‍💻 Development

### Setup Development Environment

```bash
# Clone the repository
git clone https://github.com/YOUR_USERNAME/prepump.git
cd prepump

# Download dependencies
go mod download

# Verify everything compiles
go build ./...
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/scanner/...

# Run tests with verbose output
go test -v ./...
```

### Code Quality

```bash
# Format code
go fmt ./...

# Vet code for issues
go vet ./...

# Run linter (requires golangci-lint)
golangci-lint run
```

### Makefile Targets

```bash
# Build the binary
make build

# Build for all platforms
make build-all

# Run tests
make test

# Format code
make fmt

# Clean build artifacts
make clean

# Run the application
make run
```

### Building for Multiple Platforms

```bash
# Using Make
make build-all

# Manual cross-compilation
GOOS=linux GOARCH=amd64 go build -o prepump-linux-amd64 ./cmd/prepump
GOOS=darwin GOARCH=amd64 go build -o prepump-darwin-amd64 ./cmd/prepump
GOOS=windows GOARCH=amd64 go build -o prepump-windows-amd64.exe ./cmd/prepump
```

---

## 🐳 Docker

### Build Image

```bash
docker build -t prepump:latest .
```

### Run Container

```bash
# Interactive mode
docker run -it --rm prepump:latest

# With custom config mounted
docker run -it --rm -v $(pwd)/config.yaml:/app/config.yaml prepump:latest

# With environment variables
docker run -it --rm \
  -e DEEPCOIN_API_KEY=your_key \
  -e DEEPCOIN_API_SECRET=your_secret \
  prepump:latest
```

### Docker Compose (Optional)

Create `docker-compose.yml`:

```yaml
version: '3.8'
services:
  prepump:
    build: .
    container_name: prepump-scanner
    restart: unless-stopped
    volumes:
      - ./config.yaml:/app/config.yaml
    environment:
      - DEEPCOIN_API_KEY=${DEEPCOIN_API_KEY}
      - DEEPCOIN_API_SECRET=${DEEPCOIN_API_SECRET}
    tty: true
    stdin_open: true
```

Run with:

```bash
docker-compose up -d
docker-compose logs -f
```

---

## 🔧 Troubleshooting

### Common Issues

#### 1. Build Fails with Dependency Errors

```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
go mod download

# Tidy module file
go mod tidy

# Rebuild
go build ./cmd/prepump
```

#### 2. "Connection Refused" Errors

- Check your internet connection
- Verify Pyth Network and Deepcoin APIs are accessible
- Try increasing timeout values in config.yaml

#### 3. TUI Display Issues

```bash
# Ensure your terminal supports true color
echo $TERM

# Try a different terminal emulator
# Recommended: iTerm2 (macOS), Alacritty, Kitty, or Windows Terminal
```

#### 4. High CPU Usage

```bash
# Reduce number of workers
./prepump -workers=5

# Reduce scan range
./prepump -top=50

# Disable live stats
./prepump -no-live-stats
```

#### 5. No Coins Showing Signals

- Increase `min_volume_usd` threshold in config
- Check if Deepcoin API is responding
- Verify the scan interval isn't too aggressive

### Debug Mode

Enable verbose logging:

```bash
# Set debug environment variable
export PREPUMP_DEBUG=true
./prepump
```

### Log Files

Check for logs in:

- Standard output (terminal)
- `/tmp/prepump.log` (if configured)

### Getting Help

1. Check this README
2. Review the [Issues](https://github.com/YOUR_USERNAME/prepump/issues) page
3. Open a new issue with:
   - Your OS and Go version
   - Error messages
   - Steps to reproduce

---

## 🤝 Contributing

Contributions are welcome! Please follow these steps:

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

### Contribution Guidelines

- Write clear, concise commit messages
- Add tests for new features
- Ensure all tests pass (`make test`)
- Format code properly (`make fmt`)
- Update documentation as needed

### Code Style

- Follow Go best practices
- Use meaningful variable names
- Add comments for complex logic
- Keep functions small and focused

---

## 📄 License

This project is licensed under the MIT License - see below for details:

```
MIT License

Copyright (c) 2024 PrePump Scanner

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

## ⚠️ Disclaimer

**IMPORTANT: This software is for educational and informational purposes only.**

- **Not Financial Advice**: PrePump Scanner does not constitute financial, investment, or trading advice.
- **No Guarantees**: The signals and scores generated are probabilistic and do not guarantee any particular outcome.
- **Risk Warning**: Cryptocurrency trading involves substantial risk of loss. Only trade with funds you can afford to lose.
- **Do Your Own Research**: Always conduct your own research before making any trading decisions.
- **API Usage**: You are responsible for complying with the terms of service of Pyth Network, Deepcoin, and any other APIs used.

The authors and contributors are not responsible for any financial losses or damages resulting from the use of this software.

---

## 🙏 Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - The delightful TUI framework
- [Pyth Network](https://pyth.network/) - Real-time price feeds
- [Deepcoin](https://www.deepcoin.com/) - Cryptocurrency exchange API
- [Hidden Markov Model](https://en.wikipedia.org/wiki/Hidden_Markov_model) - Statistical modeling technique

---

## 📬 Contact

- **Repository**: [github.com/YOUR_USERNAME/prepump](https://github.com/YOUR_USERNAME/prepump)
- **Issues**: [github.com/YOUR_USERNAME/prepump/issues](https://github.com/YOUR_USERNAME/prepump/issues)

---

<div align="center">

**Made with ❤️ using Go**

[⬆ Back to Top](#-prepump-scanner-v4)

</div>
