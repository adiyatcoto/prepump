# Changelog

All notable changes to PrePump Scanner will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release preparation
- Comprehensive README and documentation
- GitHub Actions CI/CD pipeline
- Docker support
- Issue and PR templates

### Changed
- N/A

### Deprecated
- N/A

### Removed
- N/A

### Fixed
- N/A

### Security
- N/A

---

## [4.0.0] - 2024-04-19

### Added
- **Real-time price streams**: Pyth Network SSE + Deepcoin WebSocket integration
- **Machine Learning**: Pure Go HMM (Hidden Markov Model) for market regime detection
  - 3 states: BULL, BEAR, SIDEWAYS
  - Configurable iterations and training parameters
- **Multi-factor analysis**: 9 weighted signals
  - Volume spike detection (15%)
  - Open Interest changes (12%)
  - Funding rate anomalies (15%)
  - Cumulative Volume Delta (12%)
  - RSI divergence signals (8%)
  - Bollinger Bands compression (8%)
  - Price momentum (5%)
  - HMM regime detection (15%)
  - Coinbase premium analysis (10%)
- **T-TUI**: Beautiful Bubble Tea terminal interface
  - Live price updates with flash animations (▲/▼)
  - Navigable coin list sorted by PumpScore
  - Hot/Warm threshold indicators (🔥/⚠️)
  - Keyboard shortcuts for navigation and actions
  - Detail view for individual coins
  - DexScreener integration
- **Concurrent architecture**: 3 concurrent goroutine streams
  - Pyth SSE stream for price ticks
  - Deepcoin polling for market data
  - Live stats recalculation loop (30s interval)
- **Configuration system**: YAML-based configuration
  - Configurable scan parameters
  - Adjustable signal weights
  - Customizable thresholds
  - API timeout and rate limit settings
- **Caching**: Efficient candle data caching with TTL
- **Rate limiting**: API rate limiting to prevent throttling
- **Fear & Greed Index**: Integration with Alternative.me API
- **CLI flags**: Comprehensive command-line options
  - `-top`: Number of coins to scan
  - `-min-vol`: Minimum volume filter
  - `-workers`: Concurrent worker count
  - `-no-stream`: Disable live price stream
  - `-no-live-stats`: Disable live recalculation
  - `-deepcoin-key`: Deepcoin API key
  - `-deepcoin-secret`: Deepcoin API secret
- **Makefile**: Build automation targets
  - `make build`: Build binary
  - `make build-all`: Cross-platform builds
  - `make test`: Run tests
  - `make fmt`: Format code
  - `make run`: Run application
  - `make clean`: Clean build artifacts
- **Docker support**: Containerized deployment
  - Multi-stage Dockerfile for small image size
  - Docker Compose configuration
- **Documentation**: Comprehensive documentation
  - README.md with detailed usage guide
  - QUICKSTART.md for quick setup
  - CONTRIBUTING.md for contributors
  - CODE_OF_CONDUCT.md
  - SECURITY.md
  - LICENSE (MIT)
- **Testing**: Unit tests for core components
  - Scanner engine tests
  - HMM engine tests
- **GitHub templates**: Issue and PR templates
  - Bug report template
  - Feature request template
  - Pull request template

### Changed
- N/A (Initial release)

### Deprecated
- N/A

### Removed
- N/A

### Fixed
- N/A

### Security
- API keys handled via environment variables or CLI flags
- Sensitive files excluded via .gitignore
- Security policy established

---

## [3.0.0] - 2024-04-06

### Added
- Initial scanner engine implementation
- Basic TUI with Bubble Tea
- Deepcoin API integration
- Pyth Network integration

### Changed
- N/A

### Deprecated
- N/A

### Removed
- N/A

### Fixed
- N/A

### Security
- N/A

---

## [2.0.0] - 2024-03-15

### Added
- Basic price scanning functionality
- Simple command-line interface

### Changed
- N/A

### Deprecated
- N/A

### Removed
- N/A

### Fixed
- N/A

### Security
- N/A

---

## [1.0.0] - 2024-02-01

### Added
- Initial project setup
- Basic Go module structure

### Changed
- N/A

### Deprecated
- N/A

### Removed
- N/A

### Fixed
- N/A

### Security
- N/A

---

## Legend

- **Added**: New features
- **Changed**: Changes in existing functionality
- **Deprecated**: Soon to be removed features
- **Removed**: Removed features
- **Fixed**: Bug fixes
- **Security**: Security improvements

## Versioning

We use [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible changes
- **MINOR** version for backwards-compatible functionality
- **PATCH** version for backwards-compatible bug fixes

## Release Notes

### Version 4.x (Current)
- Production-ready release
- Full feature set with ML integration
- Real-time streaming capabilities
- Professional documentation

### Version 3.x
- Beta release
- Core scanning functionality
- Basic TUI

### Version 2.x
- Alpha release
- Proof of concept

### Version 1.x
- Initial development
- Project scaffolding

---

**Last Updated**: April 19, 2024
