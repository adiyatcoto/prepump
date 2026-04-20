// internal/pyth/client.go
//
// Pyth Network — Hermes REST + SSE price feed client.
// Docs: https://hermes.pyth.network/docs
//
// Key endpoints:
//   GET  /v2/price_feeds?query=BTC&asset_type=crypto  → find feed IDs
//   GET  /v2/updates/price/latest?ids[]=<feedId>      → latest price
//   GET  /v2/updates/price/stream?ids[]=<feedId>      → SSE real-time stream
//
// Pyth price IDs for common pairs (hex):
//   BTC/USD  e62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43
//   ETH/USD  ff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace
//   SOL/USD  ef0d8b6fda2ceba41da15d4095d1da392a0d2f8ed0c6c7bc0f4cfac8c280b56d
//   BNB/USD  2f95862b045670cd22bee3114c39763a4a08beeb663b145d283c31d7d1101c4f
//   XRP/USD  bfaf7739cb6fe3e1c57a0ac08e1d931e9e6062d476fa57804e165ab572b5b621
//   DOGE/USD dcef50dd0a4cd2dcc17e45df1676dcb336a11a61c69df7a0299b0150c672d25c
//   ADA/USD  2a01deaec9e51a579277b34b122399984d0bbf57e2458a7e42fecd2829867a0d
//   AVAX/USD 93da3352f9f1d105fdfe4971cfa80e9dd777bfc5d0f683ebb6e1294b92137bb7
//   MATIC/USD 5de33a9112c2b700b8d30b8a3402c103578ccfa2765696471cc672bd5cf6ac52
//   LINK/USD  8ac0c70fff57e9aefdf5edf44b51d62c2d433653cbb2cf5cc06bb115af04d221
//   DOT/USD   ca3eed9b267293f6595901c734c7525ce8ef49adafe8284606ceb307afa2ca5b
//   UNI/USD   78d185a741d07edb3412b09008b7c5cfb9bbbd7d568bf00ba737b456ba171501
//   ATOM/USD  b00b60f88b03a6a625a8d1c048c3f66653edf217439983d037e7218b1d4c5f6

package pyth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const hermesBase = "https://hermes.pyth.network"

// Well-known Pyth price feed IDs (BTC, ETH, major alts)
var KnownFeeds = map[string]string{
	"BTC":   "e62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43",
	"ETH":   "ff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace",
	"SOL":   "ef0d8b6fda2ceba41da15d4095d1da392a0d2f8ed0c6c7bc0f4cfac8c280b56d",
	"BNB":   "2f95862b045670cd22bee3114c39763a4a08beeb663b145d283c31d7d1101c4f",
	"XRP":   "bfaf7739cb6fe3e1c57a0ac08e1d931e9e6062d476fa57804e165ab572b5b621",
	"DOGE":  "dcef50dd0a4cd2dcc17e45df1676dcb336a11a61c69df7a0299b0150c672d25c",
	"ADA":   "2a01deaec9e51a579277b34b122399984d0bbf57e2458a7e42fecd2829867a0d",
	"AVAX":  "93da3352f9f1d105fdfe4971cfa80e9dd777bfc5d0f683ebb6e1294b92137bb7",
	"MATIC": "5de33a9112c2b700b8d30b8a3402c103578ccfa2765696471cc672bd5cf6ac52",
	"LINK":  "8ac0c70fff57e9aefdf5edf44b51d62c2d433653cbb2cf5cc06bb115af04d221",
	"DOT":   "ca3eed9b267293f6595901c734c7525ce8ef49adafe8284606ceb307afa2ca5b",
	"UNI":   "78d185a741d07edb3412b09008b7c5cfb9bbbd7d568bf00ba737b456ba171501",
	"ATOM":  "b00b60f88b03a6a625a8d1c048c3f66653edf217439983d037e7218b1d4c5f6",
	"LTC":   "6e3f3fa8253588df9326580180233eb791e03b443a3ba7a1d892e73874e19a54",
	"TRX":   "67aed5a24fdad045475e7195084d2d8195e2f0f6896885354b88b710b5d4ef8e",
	"OP":    "385f64d993f7b77d8182ed5003d97c60aa3361f3cecfe711544d2d59165e9bdf",
	"ARB":   "3fa4252848f9f0a1480be62745a4629d9eb1322aebab8a791e344b3b9c1adcf5",
	"SUI":   "23d7315113f5b1d3ba7a83604c44b94d79f4fd69af77f804fc7f920a6dc65744",
	"APT":   "03ae4db29ed4ae33d323568895aa00337e658e348b37509f5372ae51f0af00d5",
	"INJ":   "7a5bc1d2b56ad029048cd63964b3ad2776eaed1638317f9dce95c7c1e8095001",
	"SEI":   "53614f1cb0c031d4af66c04cb9c756234adad0e1cee85303795091499a4084eb",
	"NEAR":  "c415de8d2eba7db216527dff4b60e8f3a5311c740dadb233e13e12547e226750",
}

// Price holds a decoded Pyth price update.
type Price struct {
	Symbol     string
	FeedID     string
	Price      float64
	Conf       float64   // confidence interval (±)
	Expo       int       // decimal exponent
	PublishTime time.Time
	EMAPrice   float64   // exponential moving average price
	EMAConf    float64
}

type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// ─── REST: latest prices ──────────────────────────────────────────────────────

// pythPriceResp mirrors the Hermes /v2/updates/price/latest JSON shape.
type pythPriceResp struct {
	Parsed []struct {
		ID    string `json:"id"`
		Price struct {
			Price       string `json:"price"`
			Conf        string `json:"conf"`
			Expo        int    `json:"expo"`
			PublishTime int64  `json:"publish_time"`
		} `json:"price"`
		EMAPrice struct {
			Price string `json:"price"`
			Conf  string `json:"conf"`
			Expo  int    `json:"expo"`
		} `json:"ema_price"`
	} `json:"parsed"`
}

func decodeRaw(raw string, expo int) float64 {
	v, _ := strconv.ParseFloat(raw, 64)
	for expo < 0 {
		v /= 10
		expo++
	}
	for expo > 0 {
		v *= 10
		expo--
	}
	return v
}

// LatestPrices fetches latest prices for a slice of symbols.
// Unknown symbols (not in KnownFeeds) are silently skipped.
func (c *Client) LatestPrices(symbols []string) (map[string]Price, error) {
	ids := []string{}
	symByID := map[string]string{}
	for _, s := range symbols {
		id, ok := KnownFeeds[strings.ToUpper(s)]
		if !ok {
			continue
		}
		ids = append(ids, id)
		symByID[id] = strings.ToUpper(s)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no known Pyth feed IDs for given symbols")
	}

	url := hermesBase + "/v2/updates/price/latest?encoding=json&parsed=true"
	for _, id := range ids {
		url += "&ids[]=" + id
	}

	req, _ := http.NewRequest("GET", url, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pyth latest: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var pr pythPriceResp
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("pyth parse: %w", err)
	}

	result := make(map[string]Price, len(pr.Parsed))
	for _, p := range pr.Parsed {
		sym := symByID[p.ID]
		price := Price{
			Symbol:      sym,
			FeedID:      p.ID,
			Price:       decodeRaw(p.Price.Price, p.Price.Expo),
			Conf:        decodeRaw(p.Price.Conf, p.Price.Expo),
			Expo:        p.Price.Expo,
			PublishTime: time.Unix(p.Price.PublishTime, 0),
			EMAPrice:    decodeRaw(p.EMAPrice.Price, p.EMAPrice.Expo),
			EMAConf:     decodeRaw(p.EMAPrice.Conf, p.EMAPrice.Expo),
		}
		result[sym] = price
	}
	return result, nil
}

// ─── SSE: streaming real-time prices ─────────────────────────────────────────

// StreamPrices opens an SSE stream from Hermes and sends decoded prices to ch.
// Runs until ctx is cancelled. Reconnects automatically on error.
func (c *Client) StreamPrices(ctx context.Context, symbols []string, ch chan<- Price) {
	ids := []string{}
	symByID := map[string]string{}
	for _, s := range symbols {
		id, ok := KnownFeeds[strings.ToUpper(s)]
		if !ok {
			continue
		}
		ids = append(ids, id)
		symByID[id] = strings.ToUpper(s)
	}
	if len(ids) == 0 {
		return
	}

	url := hermesBase + "/v2/updates/price/stream?encoding=json&parsed=true&allow_unordered=false&benchmarks_only=false"
	for _, id := range ids {
		url += "&ids[]=" + id
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		func() {
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				return
			}
			req.Header.Set("Accept", "text/event-stream")

			// Use a long-timeout client for SSE
			sseClient := &http.Client{Timeout: 0}
			resp, err := sseClient.Do(req)
			if err != nil {
				time.Sleep(2 * time.Second)
				return
			}
			defer resp.Body.Close()

			scanner := bufio.NewScanner(resp.Body)
			var dataLines []string
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data:") {
					dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
				} else if line == "" && len(dataLines) > 0 {
					// End of SSE event — parse
					raw := strings.Join(dataLines, "")
					dataLines = dataLines[:0]

					var pr pythPriceResp
					if err := json.Unmarshal([]byte(raw), &pr); err != nil {
						continue
					}
					for _, p := range pr.Parsed {
						sym, ok := symByID[p.ID]
						if !ok {
							continue
						}
						price := Price{
							Symbol:      sym,
							FeedID:      p.ID,
							Price:       decodeRaw(p.Price.Price, p.Price.Expo),
							Conf:        decodeRaw(p.Price.Conf, p.Price.Expo),
							Expo:        p.Price.Expo,
							PublishTime: time.Unix(p.Price.PublishTime, 0),
							EMAPrice:    decodeRaw(p.EMAPrice.Price, p.EMAPrice.Expo),
							EMAConf:     decodeRaw(p.EMAPrice.Conf, p.EMAPrice.Expo),
						}
						select {
						case ch <- price:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}()

		// Reconnect after 2s delay
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// SearchFeeds calls Hermes to find feed IDs for a token symbol.
// Use this if KnownFeeds doesn't have the coin you need.
func (c *Client) SearchFeeds(query string) ([]struct{ ID, Symbol string }, error) {
	url := fmt.Sprintf("%s/v2/price_feeds?query=%s&asset_type=crypto", hermesBase, query)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result []struct {
		ID         string `json:"id"`
		Attributes struct {
			Symbol string `json:"symbol"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	out := make([]struct{ ID, Symbol string }, 0, len(result))
	for _, r := range result {
		out = append(out, struct{ ID, Symbol string }{r.ID, r.Attributes.Symbol})
	}
	return out, nil
}
