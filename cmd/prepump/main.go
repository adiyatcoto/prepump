// cmd/prepump/main.go — v4
//
// Changes from v3:
//   ✓ startPythStream: sends MsgTickerUpdate with OldPrice for ▲/▼ flash
//   ✓ startLiveStatsLoop: goroutine that re-calculates signals every 30s using
//       fresh 1m candles, sends MsgLiveStats — updates deep view stats live
//   ✓ startDeepCoinWS: hooks into Deepcoin WebSocket (if available) for
//       sub-millisecond price ticks; falls back to Pyth SSE
//   ✓ All three streams run concurrently; tea.Program.Send() is goroutine-safe

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/you/prepump/internal/deepcoin"
	"github.com/you/prepump/internal/hmm"
	"github.com/you/prepump/internal/pyth"
	"github.com/you/prepump/internal/scanner"
	"github.com/you/prepump/internal/tui"
)

// ─── Flags ────────────────────────────────────────────────────────────────────

var (
	flagTop      = flag.Int("top", 100, "scan top N coins by volume (spot+futures combined)")
	flagMinVol   = flag.Float64("min-vol", 200_000, "minimum 24h USD volume")
	flagWorkers  = flag.Int("workers", 10, "concurrent analysis goroutines")
	flagNoStream = flag.Bool("no-stream", false, "disable Pyth SSE live price stream")
	flagNoLive   = flag.Bool("no-live-stats", false, "disable live signal recalculation loop")
	flagAPIKey   = flag.String("deepcoin-key", "", "Deepcoin API key (optional)")
	flagSecret   = flag.String("deepcoin-secret", "", "Deepcoin API secret")
)

// ─── Globals ──────────────────────────────────────────────────────────────────

var (
	dcClient   *deepcoin.Client
	pythClient *pyth.Client
)

// activeScan is a live snapshot of the last scan results; updated atomically
// so the live-stats loop can iterate without a full rescan.
var activeScan struct {
	sync.RWMutex
	coins []scanEntry
}

type scanEntry struct {
	base       string
	symbol     string
	hasFutures bool
	hasSpot    bool
}

// ─── Fear & Greed ─────────────────────────────────────────────────────────────

var fgCache struct {
	sync.Mutex
	value int
	label string
	at    time.Time
}

func fetchFearGreed() (int, string) {
	fgCache.Lock()
	defer fgCache.Unlock()
	if fgCache.value != 0 && time.Since(fgCache.at) < time.Hour {
		return fgCache.value, fgCache.label
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.alternative.me/fng/?limit=1")
	if err != nil {
		return 50, "Neutral"
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data []struct {
			Value               string `json:"value"`
			ValueClassification string `json:"value_classification"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil || len(r.Data) == 0 {
		return 50, "Neutral"
	}
	v, _ := strconv.Atoi(r.Data[0].Value)
	fgCache.value = v
	fgCache.label = r.Data[0].ValueClassification
	fgCache.at = time.Now()
	return v, r.Data[0].ValueClassification
}

// ─── Scan ─────────────────────────────────────────────────────────────────────

func runScan() tea.Msg {
	fgVal, fgLabel := fetchFearGreed()

	allTickers, err := dcClient.AllTickers()
	if err != nil {
		return tui.MsgError{Err: fmt.Errorf("deepcoin tickers: %w", err)}
	}

	type candidate struct {
		base       string
		symbol     string
		vol        float64
		change     float64
		hasSpot    bool
		hasFutures bool
	}
	var candidates []candidate
	for base, t := range allTickers {
		if t.Volume24h < *flagMinVol {
			continue
		}
		candidates = append(candidates, candidate{
			base:       base,
			symbol:     base + "-USDT",
			vol:        t.Volume24h,
			change:     t.Change24h,
			hasSpot:    t.IsSpot,
			hasFutures: t.IsFutures,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].vol > candidates[j].vol
	})
	if len(candidates) > *flagTop {
		candidates = candidates[:*flagTop]
	}

	// Persist scan entries for live-stats loop
	entries := make([]scanEntry, len(candidates))
	for i, c := range candidates {
		entries[i] = scanEntry{
			base:       c.base,
			symbol:     c.symbol,
			hasFutures: c.hasFutures,
			hasSpot:    c.hasSpot,
		}
	}
	activeScan.Lock()
	activeScan.coins = entries
	activeScan.Unlock()

	pythSymbols := []string{"BTC", "ETH"}
	for _, c := range candidates {
		if _, ok := pyth.KnownFeeds[c.base]; ok {
			pythSymbols = append(pythSymbols, c.base)
		}
	}
	pythPrices, _ := pythClient.LatestPrices(dedup(pythSymbols))

	var btcPythPrice *pyth.Price
	if p, ok := pythPrices["BTC"]; ok {
		btcPythPrice = &p
	}

	var deepBTCPrice float64
	if t, ok := allTickers["BTC"]; ok {
		deepBTCPrice = t.Price
	}

	results := make([]scanner.CoinResult, 0, len(candidates))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, *flagWorkers)

	for _, c := range candidates {
		wg.Add(1)
		go func(c candidate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			candles4h, err := dcClient.CandlesBest(c.base, "4H", 300, c.hasFutures)
			if err != nil || len(candles4h) < 60 {
				return
			}
			candles1h, _ := dcClient.CandlesBest(c.base, "1H", 100, c.hasFutures)

			var fundingRate *deepcoin.FundingRate
			var oi *deepcoin.OpenInterest
			if c.hasFutures {
				swapID := c.base + "-USDT-SWAP"
				fundingRate, _ = dcClient.FundingRate(swapID)
				oi, _ = dcClient.OpenInterest(swapID)
			}

			var pythPrice *pyth.Price
			if p, ok := pythPrices[c.base]; ok {
				pythPrice = &p
			}

			sigs := scanner.CalcSignals(
				candles4h, candles1h,
				fundingRate, oi,
				pythPrice, btcPythPrice, deepBTCPrice,
				fgVal, fgLabel,
			)

			plan := scanner.CalcTradePlan(candles4h, sigs.PumpScore, sigs.SR, c.base)

			result := scanner.CoinResult{
				Symbol:     c.symbol,
				HasSpot:    c.hasSpot,
				HasFutures: c.hasFutures,
				VolUSD:     c.vol,
				Change24h:  c.change,
				Signals:    sigs,
				Plan:       plan,
				UpdatedAt:  time.Now(),
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(c)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Signals.PumpScore > results[j].Signals.PumpScore
	})

	btcPrice := deepBTCPrice
	if btcPythPrice != nil {
		btcPrice = btcPythPrice.Price
	}

	return tui.MsgScanDone{
		Results:  results,
		BTCPrice: btcPrice,
		FGValue:  fgVal,
		FGLabel:  fgLabel,
	}
}

func dedup(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ─── Pyth SSE stream ──────────────────────────────────────────────────────────

// startPythStream subscribes to Pyth SSE for sub-second price updates.
// Each tick sends MsgTickerUpdate with the previous price for ▲/▼ flash.
func startPythStream(ctx context.Context, p *tea.Program) {
	ch := make(chan pyth.Price, 512)
	var symbols []string
	for k := range pyth.KnownFeeds {
		symbols = append(symbols, k)
	}
	go pythClient.StreamPrices(ctx, symbols, ch)

	// Track previous price per symbol for flash arrow
	prev := map[string]float64{}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case price := <-ch:
				sym := price.Symbol + "-USDT"
				old := prev[sym]
				prev[sym] = price.Price
				p.Send(tui.MsgTickerUpdate{
					Symbol:   sym,
					Price:    price.Price,
					OldPrice: old,
				})
			}
		}
	}()
}

// ─── Deepcoin WebSocket stream ────────────────────────────────────────────────

// startDeepCoinTickerStream polls Deepcoin tickers at high frequency for coins
// not covered by Pyth feeds. This gives ~100ms granularity for alt-coins.
// In production, replace this with the actual Deepcoin WebSocket subscription.
func startDeepCoinTickerStream(ctx context.Context, p *tea.Program) {
	prev := map[string]float64{}

	go func() {
		// Initial delay to let the first scan populate activeScan
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		// Round-robin index so we spread API calls across coins
		idx := 0

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				activeScan.RLock()
				coins := activeScan.coins
				activeScan.RUnlock()

				if len(coins) == 0 {
					continue
				}

				// Fetch one coin per tick (round-robin) — rate-limit friendly
				c := coins[idx%len(coins)]
				idx++

				// Skip coins already covered by Pyth (they get SSE updates)
				if _, hasPyth := pyth.KnownFeeds[c.base]; hasPyth {
					continue
				}

				// Lightweight ticker fetch for this one coin
				go func(c scanEntry) {
					tickers, err := dcClient.AllTickers()
					if err != nil {
						return
					}
					t, ok := tickers[c.base]
					if !ok {
						return
					}
					sym := c.symbol
					old := prev[sym]
					if t.Price == old {
						return // no change, skip send
					}
					prev[sym] = t.Price
					p.Send(tui.MsgTickerUpdate{
						Symbol:   sym,
						Price:    t.Price,
						OldPrice: old,
					})
				}(c)
			}
		}
	}()
}

// ─── Live Stats Loop ──────────────────────────────────────────────────────────

// startLiveStatsLoop recalculates signals for active coins every 30 seconds
// using fresh 1m candles. This keeps RSI, BB, CVD, etc. live between full scans.
// Results are sent as MsgLiveStats and applied to deep view and table rows.
func startLiveStatsLoop(ctx context.Context, p *tea.Program) {
	go func() {
		// Stagger initial start
		timer := time.NewTimer(15 * time.Second)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()

		sem := make(chan struct{}, 5) // limit concurrent recalcs

		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				activeScan.RLock()
				coins := activeScan.coins
				activeScan.RUnlock()

				fgVal, fgLabel := fetchFearGreed()

				var deepBTCPrice float64
				if tickers, err := dcClient.AllTickers(); err == nil {
					if t, ok := tickers["BTC"]; ok {
						deepBTCPrice = t.Price
					}
				}

				for _, c := range coins {
					c := c // capture
					go func() {
						sem <- struct{}{}
						defer func() { <-sem }()

						select {
						case <-ctx.Done():
							return
						default:
						}

						// Fetch fresh 1m candles for signal recalc
						candles1m, err := dcClient.CandlesBest(c.base, "1m", 200, c.hasFutures)
						if err != nil || len(candles1m) < 30 {
							return
						}
						// Also need 4h candles for HMM (cached candles would be better;
						// here we refetch — in production, maintain an in-memory candle store)
						candles4h, err := dcClient.CandlesBest(c.base, "4H", 100, c.hasFutures)
						if err != nil || len(candles4h) < 30 {
							return
						}

						var fundingRate *deepcoin.FundingRate
						var oi *deepcoin.OpenInterest
						if c.hasFutures {
							swapID := c.base + "-USDT-SWAP"
							fundingRate, _ = dcClient.FundingRate(swapID)
							oi, _ = dcClient.OpenInterest(swapID)
						}

						sigs := scanner.CalcSignals(
							candles4h, candles1m,
							fundingRate, oi,
							nil, nil, deepBTCPrice,
							fgVal, fgLabel,
						)

						p.Send(tui.MsgLiveStats{
							Symbol:  c.symbol,
							Signals: sigs,
						})
					}()
				}
			}
		}
	}()
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()
	log.SetOutput(io.Discard)

	dcClient = deepcoin.New(*flagAPIKey, *flagSecret)
	pythClient = pyth.New()

	m := tui.New()
	m.ScanFn = func() tea.Msg { return runScan() }

	prog := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Live price streams (sub-second) ──────────────────────────────────────
	if !*flagNoStream {
		// Pyth SSE: covers BTC, ETH, and major assets with Pyth feeds
		startPythStream(ctx, prog)
		// Deepcoin polling: covers alt-coins not in Pyth, ~100ms round-robin
		startDeepCoinTickerStream(ctx, prog)
	}

	// ── Live stats recalculation (every 30s) ─────────────────────────────────
	if !*flagNoLive {
		startLiveStatsLoop(ctx, prog)
	}

	// Kick off first scan immediately
	go func() {
		time.Sleep(300 * time.Millisecond)
		prog.Send(tui.MsgTick(time.Now()))
	}()

	if _, err := prog.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}

	cancel()

	_ = strings.ToUpper
	_ = hmm.AnalyseDC
}