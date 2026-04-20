package scanner

import (
	"math"
	"testing"
	"time"

	"github.com/you/prepump/internal/deepcoin"
)

func TestCalcRSI(t *testing.T) {
	// Test flat prices
	prices := make([]float64, 30)
	for i := range prices {
		prices[i] = 100.0
	}
	rsi := calcRSI(prices, 14)
	last := rsi[len(rsi)-1]
	// For flat prices with no losses, RSI is 100
	if last != 100.0 {
		t.Errorf("RSI of flat price should be 100 (no losses), got %.2f", last)
	}

	// Test rising prices
	prices = []float64{}
	for i := 0; i < 30; i++ {
		prices = append(prices, float64(100+i))
	}
	rsi = calcRSI(prices, 14)
	last = rsi[len(rsi)-1]
	if last < 50 {
		t.Errorf("RSI of rising price should be > 50, got %.2f", last)
	}
}

func TestCalcBBWidth(t *testing.T) {
	// Squeeze pattern: low volatility
	prices := make([]float64, 50)
	for i := range prices {
		prices[i] = 100 + float64(i)*0.01
	}
	
	bw := calcBBWidth(prices, 20)
	lastWidth := bw[len(bw)-1]
	
	// Width should be very small for flat-ish prices
	if math.IsNaN(lastWidth) {
		t.Error("BB width is NaN")
	}
	if lastWidth > 0.1 {
		t.Errorf("BB width should be small for low volatility, got %.4f", lastWidth)
	}
}

func TestCalcATR(t *testing.T) {
	candles := make([]deepcoin.Candle, 100)
	for i := range candles {
		candles[i] = deepcoin.Candle{
			Time:   time.Now().Add(time.Duration(i) * time.Hour),
			Open:   float64(100 + i),
			High:   float64(102 + i),
			Low:    float64(98 + i),
			Close:  float64(101 + i),
			Volume: 1000,
		}
	}

	atr := calcATR(candles, 14)
	if atr <= 0 {
		t.Errorf("ATR should be positive, got %.2f", atr)
	}

	// ATR should be around 4 (High-Low roughly 4 with this pattern)
	if atr < 2 || atr > 6 {
		t.Errorf("ATR expected ~4, got %.2f", atr)
	}
}

func TestCalcSRLevels(t *testing.T) {
	// Create candles with clear support/resistance
	candles := make([]deepcoin.Candle, 50)
	for i := range candles {
		close := 100.0
		if i%5 == 0 {
			close = 105 // Resistance level
		} else if i%7 == 0 {
			close = 95 // Support level
		}
		candles[i] = deepcoin.Candle{
			Time:   time.Now().Add(time.Duration(i) * time.Hour),
			Open:   close - 1,
			High:   close + 2,
			Low:    close - 2,
			Close:  close,
			Volume: 1000,
		}
	}

	sr := calcSR(candles, 3)
	if len(sr.Supports) == 0 || len(sr.Resistances) == 0 {
		t.Error("Should find support/resistance levels")
	}
}

func TestCalcTradePlan(t *testing.T) {
	candles := make([]deepcoin.Candle, 50)
	for i := range candles {
		candles[i] = deepcoin.Candle{
			Time:   time.Now().Add(time.Duration(i) * time.Hour),
			Open:   100 + float64(i),
			High:   102 + float64(i),
			Low:    98 + float64(i),
			Close:  101 + float64(i),
			Volume: 1000,
		}
	}

	sr := calcSR(candles, 3)
	plan := CalcTradePlan(candles, 75.0, sr, "BTC")

	if plan.SL == 0 || plan.TP1 == 0 {
		t.Error("Trade plan should have SL and TP levels")
	}
	if plan.RR <= 0 {
		t.Error("Risk/Reward should be positive")
	}
}

func TestScoreLabel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{HotThreshold + 1, "HOT"},
		{HotThreshold, "HOT"},
		{WarmThreshold + 1, "WARM"},
		{WarmThreshold, "WARM"},
		{WarmThreshold - 1, "EARLY"},
	}

	for _, tc := range tests {
		result := CoinResult{Signals: Signals{PumpScore: tc.score}}.ScoreLabel()
		if result != tc.expected {
			t.Errorf("Score %.1f expected %q, got %q", tc.score, tc.expected, result)
		}
	}
}

func BenchmarkCalcSignals(b *testing.B) {
	// Setup candles
	candles4h := make([]deepcoin.Candle, 300)
	candles1h := make([]deepcoin.Candle, 100)
	
	for i := range candles4h {
		candles4h[i] = deepcoin.Candle{
			High:   100 + float64(i%10),
			Low:    90 + float64(i%10),
			Close:  95 + float64(i%10),
			Volume: 10000 + float64(i*100),
		}
	}
	for i := range candles1h {
		candles1h[i] = deepcoin.Candle{
			High:   100 + float64(i%5),
			Low:    95 + float64(i%5),
			Close:  98 + float64(i%5),
			Volume: 5000 + float64(i*50),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalcSignals(candles4h, candles1h, nil, nil, nil, nil, 100.0, 50, "Neutral")
	}
}
