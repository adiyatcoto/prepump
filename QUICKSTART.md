# 🚀 Quick Start Guide

Get PrePump Scanner running in under 5 minutes!

---

## Prerequisites Check

```bash
# Verify Go is installed (need 1.24.2+)
go version

# Verify Git is installed
git --version
```

If Go is not installed, download it from [golang.org/dl](https://golang.org/dl/)

---

## Installation (Choose One Method)

### Option 1: Clone & Build (Recommended)

```bash
# Clone the repository
git clone https://github.com/YOUR_USERNAME/prepump.git
cd prepump

# Build
go build -o prepump ./cmd/prepump

# Run
./prepump
```

### Option 2: Go Install

```bash
# Install directly
go install github.com/YOUR_USERNAME/prepump/cmd/prepump@latest

# Run
$GOPATH/bin/prepump
```

---

## First Run

```bash
# Basic run with defaults
./prepump

# Or with custom settings
./prepump -top=50 -workers=10 -min-vol=500000
```

---

## Understanding the UI

When PrePump starts, you'll see:

```
┌─────────────────────────────────────────────────────────────┐
│  PREPUMP SCANNER v4                        BTC: $67,432     │
│  F&G: 65 (Greed)                           Scan: 100 coins  │
├─────────────────────────────────────────────────────────────┤
│  COIN    PRICE      SCORE   VOL     OI     FUND   HMM  MKT  │
├─────────────────────────────────────────────────────────────┤
│  SOL    $145.32    🔥 78   +45%   +12%   +0.01%  🟢   F&S   │
│  AVAX   $38.21     ⚠️  62   +28%   +8%    -0.02%  🟡   F    │
└─────────────────────────────────────────────────────────────┘
```

### Column Meanings

| Column | Meaning |
|--------|---------|
| **COIN** | Cryptocurrency symbol |
| **PRICE** | Current price in USDT |
| **SCORE** | Pump score (0-100, higher = more likely to pump) |
| **VOL** | Volume change vs average |
| **OI** | Open Interest change |
| **FUND** | Funding rate |
| **HMM** | Market regime (🟢 Bull / 🟡 Sideways / 🔴 Bear) |
| **MKT** | Market type (F=Futures, S=Spot, F&S=Both) |

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | View coin details |
| `d` | Open DexScreener |
| `r` | Rescan |
| `q` | Quit |

---

## Configuration (Optional)

```bash
# Copy default config
cp config.yaml config.local.yaml

# Edit with your preferences
nano config.local.yaml  # or use your favorite editor

# Run with custom config
./prepump -config config.local.yaml
```

---

## Common Commands

```bash
# Scan top 30 coins with high volume
./prepump -top=30 -min-vol=1000000

# Faster scan with more workers
./prepump -workers=20

# Disable live updates (lower resource usage)
./prepump -no-stream -no-live-stats

# See all options
./prepump -h
```

---

## Troubleshooting

### Build Fails

```bash
# Clean and rebuild
go clean -modcache
go mod download
go build -o prepump ./cmd/prepump
```

### No Coins Showing

- Lower the `-min-vol` threshold
- Check your internet connection
- Wait for the first scan to complete (~30 seconds)

### TUI Looks Wrong

- Ensure your terminal supports true color
- Try a different terminal (iTerm2, Alacritty, Windows Terminal)
- Check `$TERM` environment variable

---

## Next Steps

1. **Read the full [README.md](README.md)** for detailed documentation
2. **Customize your config** in `config.yaml`
3. **Join the community** - open issues, contribute, share ideas
4. **Stay safe** - remember this is a tool, not financial advice

---

## Need Help?

- 📖 [Full Documentation](README.md)
- 🐛 [Report a Bug](https://github.com/YOUR_USERNAME/prepump/issues)
- 💡 [Request a Feature](https://github.com/YOUR_USERNAME/prepump/issues)
- 📧 [Contact Maintainers](mailto:your-email@domain.com)

---

**Happy Scanning! 🎯**
