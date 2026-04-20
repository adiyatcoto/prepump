// internal/deepcoin/client.go — v2
// Adds: TickersFutures() for SWAP instruments + combined scan

package deepcoin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const baseURL = "https://api.deepcoin.com"

type Client struct {
	http   *http.Client
	APIKey string
	Secret string
}

func New(apiKey, secret string) *Client {
	return &Client{
		http:   &http.Client{Timeout: 12 * time.Second},
		APIKey: apiKey,
		Secret: secret,
	}
}

func (c *Client) get(path string, params map[string]string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()
	if c.APIKey != "" {
		req.Header.Set("DC-ACCESS-KEY", c.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deepcoin GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// ─── Ticker ──────────────────────────────────────────────────────────────────

type Ticker struct {
	Symbol    string
	Price     float64
	Volume24h float64
	Change24h float64
	High24h   float64
	Low24h    float64
	// Which markets are active for this base asset
	IsSpot    bool
	IsFutures bool
}

type tickerRaw struct {
	Code string `json:"code"`
	Data []struct {
		InstID    string `json:"instId"`
		Last      string `json:"last"`
		VolCcy24h string `json:"volCcy24h"`
		Vol24h    string `json:"vol24h"`
		Change24h string `json:"change24h"`
		High24h   string `json:"high24h"`
		Low24h    string `json:"low24h"`
	} `json:"data"`
}

func parseTickerRaw(body []byte, isSpot, isFutures bool) ([]Ticker, error) {
	var resp tickerRaw
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("deepcoin ticker parse: %w", err)
	}
	result := make([]Ticker, 0, len(resp.Data))
	for _, d := range resp.Data {
		t := Ticker{
			Symbol:    d.InstID,
			IsSpot:    isSpot,
			IsFutures: isFutures,
		}
		t.Price, _ = strconv.ParseFloat(d.Last, 64)
		// Prefer quote-denominated volume (USDT)
		t.Volume24h, _ = strconv.ParseFloat(d.VolCcy24h, 64)
		if t.Volume24h == 0 {
			t.Volume24h, _ = strconv.ParseFloat(d.Vol24h, 64)
		}
		t.Change24h, _ = strconv.ParseFloat(d.Change24h, 64)
		t.High24h, _ = strconv.ParseFloat(d.High24h, 64)
		t.Low24h, _ = strconv.ParseFloat(d.Low24h, 64)
		result = append(result, t)
	}
	return result, nil
}

// TickersSpot returns 24h tickers for SPOT pairs.
func (c *Client) TickersSpot() ([]Ticker, error) {
	body, err := c.get("/deepcoin/market/tickers", map[string]string{"instType": "SPOT"})
	if err != nil {
		return nil, err
	}
	return parseTickerRaw(body, true, false)
}

// TickersFutures returns 24h tickers for SWAP (perpetual futures) instruments.
// Deepcoin uses instType=SWAP for perpetual futures.
func (c *Client) TickersFutures() ([]Ticker, error) {
	body, err := c.get("/deepcoin/market/tickers", map[string]string{"instType": "SWAP"})
	if err != nil {
		return nil, err
	}
	return parseTickerRaw(body, false, true)
}

// AllTickers fetches both SPOT and SWAP, merges by base asset.
// Returns map[baseSymbol] → Ticker with IsSpot/IsFutures set correctly.
// Volume = max(spot, futures) for ranking purposes.
func (c *Client) AllTickers() (map[string]Ticker, error) {
	spotTickers, err := c.TickersSpot()
	if err != nil {
		// Don't fail entirely if spot fails — try futures only
		spotTickers = nil
	}

	futTickers, err2 := c.TickersFutures()
	if err2 != nil {
		futTickers = nil
	}

	if err != nil && err2 != nil {
		return nil, fmt.Errorf("both spot and futures failed: spot=%v futures=%v", err, err2)
	}

	merged := map[string]Ticker{}

	// Spot tickers: symbol like BTC-USDT
	for _, t := range spotTickers {
		base := stripUSDT(t.Symbol)
		if base == "" {
			continue
		}
		merged[base] = t
	}

	// Futures tickers: symbol like BTC-USDT-SWAP
	for _, t := range futTickers {
		base := stripSWAP(t.Symbol)
		if base == "" {
			continue
		}
		if existing, ok := merged[base]; ok {
			// Merge: mark as both markets, take max volume
			existing.IsFutures = true
			if t.Volume24h > existing.Volume24h {
				existing.Volume24h = t.Volume24h
			}
			// Use futures price if spot is zero
			if existing.Price == 0 {
				existing.Price = t.Price
			}
			merged[base] = existing
		} else {
			// Futures only coin
			t.Symbol = base + "-USDT" // normalise symbol
			merged[base] = t
		}
	}

	return merged, nil
}

func stripUSDT(sym string) string {
	// BTC-USDT → BTC
	if len(sym) > 5 && sym[len(sym)-5:] == "-USDT" {
		return sym[:len(sym)-5]
	}
	return ""
}

func stripSWAP(sym string) string {
	// BTC-USDT-SWAP → BTC
	if len(sym) > 10 && sym[len(sym)-10:] == "-USDT-SWAP" {
		return sym[:len(sym)-10]
	}
	return ""
}

// ─── OHLCV ───────────────────────────────────────────────────────────────────

type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	VolUSD float64
}

// Candles fetches OHLCV for a spot or swap instrument.
// instID: "BTC-USDT" (spot) or "BTC-USDT-SWAP" (futures)
// bar:    "1m","5m","15m","1H","4H","1D"
func (c *Client) Candles(instID, bar string, limit int) ([]Candle, error) {
	body, err := c.get("/deepcoin/market/candles", map[string]string{
		"instId": instID,
		"bar":    bar,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code string     `json:"code"`
		Data [][]string `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("deepcoin candles parse: %w", err)
	}

	candles := make([]Candle, 0, len(resp.Data))
	for _, row := range resp.Data {
		if len(row) < 6 {
			continue
		}
		tsMs, _ := strconv.ParseInt(row[0], 10, 64)
		var k Candle
		k.Time = time.UnixMilli(tsMs)
		k.Open, _ = strconv.ParseFloat(row[1], 64)
		k.High, _ = strconv.ParseFloat(row[2], 64)
		k.Low, _ = strconv.ParseFloat(row[3], 64)
		k.Close, _ = strconv.ParseFloat(row[4], 64)
		k.Volume, _ = strconv.ParseFloat(row[5], 64)
		if len(row) >= 7 {
			k.VolUSD, _ = strconv.ParseFloat(row[6], 64)
		}
		candles = append(candles, k)
	}

	// API returns newest first — reverse to chronological
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}
	return candles, nil
}

// CandlesBest fetches candles preferring futures (richer data) over spot.
func (c *Client) CandlesBest(base, bar string, limit int, hasFutures bool) ([]Candle, error) {
	if hasFutures {
		// Try futures first
		candles, err := c.Candles(base+"-USDT-SWAP", bar, limit)
		if err == nil && len(candles) >= 30 {
			return candles, nil
		}
	}
	return c.Candles(base+"-USDT", bar, limit)
}

// ─── Funding Rate ─────────────────────────────────────────────────────────────

type FundingRate struct {
	InstID      string
	FundingRate float64
	NextTime    time.Time
}

func (c *Client) FundingRate(instID string) (*FundingRate, error) {
	body, err := c.get("/deepcoin/swap/funding-rate", map[string]string{
		"instId": instID,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code string `json:"code"`
		Data []struct {
			InstID          string `json:"instId"`
			FundingRate     string `json:"fundingRate"`
			NextFundingTime string `json:"nextFundingTime"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("deepcoin funding parse: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no funding data for %s", instID)
	}
	d := resp.Data[0]
	fr := &FundingRate{InstID: d.InstID}
	fr.FundingRate, _ = strconv.ParseFloat(d.FundingRate, 64)
	tsMs, _ := strconv.ParseInt(d.NextFundingTime, 10, 64)
	fr.NextTime = time.UnixMilli(tsMs)
	return fr, nil
}

// ─── Open Interest ────────────────────────────────────────────────────────────

type OpenInterest struct {
	InstID string
	OI     float64
	OIUsd  float64
}

func (c *Client) OpenInterest(instID string) (*OpenInterest, error) {
	body, err := c.get("/deepcoin/swap/open-interest", map[string]string{
		"instId": instID,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code string `json:"code"`
		Data []struct {
			InstID string `json:"instId"`
			Oi     string `json:"oi"`
			OiCcy  string `json:"oiCcy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("deepcoin OI parse: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no OI data for %s", instID)
	}
	d := resp.Data[0]
	oi := &OpenInterest{InstID: d.InstID}
	oi.OI, _ = strconv.ParseFloat(d.Oi, 64)
	oi.OIUsd, _ = strconv.ParseFloat(d.OiCcy, 64)
	return oi, nil
}