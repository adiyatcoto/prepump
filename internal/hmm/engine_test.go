package hmm

import (
	"math"
	"testing"

	"github.com/you/prepump/internal/deepcoin"
)

func TestNewModel(t *testing.T) {
	m := NewModel()
	if m.N != 3 {
		t.Errorf("Expected 3 states, got %d", m.N)
	}
	
	sum := 0.0
	for _, p := range m.Pi {
		sum += p
	}
	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("Initial probs should sum to 1, got %.4f", sum)
	}
}

func TestForwardBackward(t *testing.T) {
	m := NewModel()
	obs := make([]float64, 50)
	
	// Create synthetic observation sequence
	for i := range obs {
		if i < 20 {
			obs[i] = 0.003 // Bull-like
		} else if i < 35 {
			obs[i] = 0.000 // Sideways
		} else {
			obs[i] = -0.003 // Bear-like
		}
	}
	
	alpha, scales := m.forward(obs)
	if len(alpha) != len(obs) {
		t.Error("Alpha length mismatch")
	}
	if len(scales) != len(obs) {
		t.Error("Scales length mismatch")
	}
	// Check all alphas are normalized
	for i, a := range alpha {
		sum := 0.0
		for _, v := range a {
			sum += v
		}
		if math.Abs(sum-1.0) > 0.01 {
			t.Errorf("Alpha %d not normalized: sum=%.4f", i, sum)
		}
	}
	
	beta := m.backward(obs, scales)
	if len(beta) != len(obs) {
		t.Error("Beta length mismatch")
	}
}

func TestViterbi(t *testing.T) {
	m := NewModel()
	
	// Test with clear bull sequence
	obs := make([]float64, 30)
	for i := range obs {
		obs[i] = 0.005 // Strong positive returns
	}
	
	m.Train(obs, 10)
	path := m.Viterbi(obs)
	
	if len(path) != len(obs) {
		t.Errorf("Expected path length %d, got %d", len(obs), len(path))
	}
	
	// Most states should be bull (0) after training on bull data
	bullCount := 0
	for _, s := range path {
		if s < m.N {
			bullCount++
		}
	}
	if bullCount < len(path)/3 {
		t.Error("Viterbi should identify mostly bullish states")
	}
}

func TestAnalyseRegime(t *testing.T) {
	// Create clear bull candles
	candles := make([]deepcoin.Candle, 50)
	price := 100.0
	for i := range candles {
		price *= 1.003 // 0.3% up per candle
		candles[i] = deepcoin.Candle{
			Open:  price / 1.003,
			High:  price * 1.005,
			Low:   price / 1.005,
			Close: price,
		}
	}
	
	result := AnalyseRegime(candles)
	
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("Confidence out of range: %.4f", result.Confidence)
	}
	
	sum := result.BullProb + result.SideProb + result.BearProb
	if math.Abs(sum-1.0) > 0.01 {
		t.Errorf("Posterior probs should sum to 1, got %.4f", sum)
	}
}

func TestAnalyseDC(t *testing.T) {
	// Create trend candles with stronger movement
	candles := make([]deepcoin.Candle, 100)
	price := 100.0
	for i := range candles {
		price *= 1.01 // 1% per candle for stronger trend
		candles[i] = deepcoin.Candle{
			High:  price * 1.015,
			Low:   price / 1.015,
			Close: price,
		}
	}
	
	result := AnalyseDC(candles, 0.005)
	
	if result.NEvents < 1 {
		t.Errorf("Should detect Directional Change events in trending data, got %d events", result.NEvents)
	}
	
	if result.TrendPct < result.RangePct {
		t.Errorf("Trending data should have higher trend%%; got %g%% vs %g%%", result.TrendPct, result.RangePct)
	}
}

func TestLogReturns(t *testing.T) {
	candles := []deepcoin.Candle{
		{Close: 100},
		{Close: 101},
		{Close: 100},
		{Close: 102},
	}
	
	returns := logReturns(candles)
	
	if len(returns) != len(candles) {
		t.Error("Returns length mismatch")
	}
	
	// Check first return is 0 (no previous candle)
	if returns[0] != 0 {
		t.Error("First return should be 0")
	}
}

func BenchmarkTrain(b *testing.B) {
	obs := make([]float64, 200)
	rng := 100.0
	for i := range obs {
		rng *= 1 + (float64(i%3-1)*0.01)
		obs[i] = math.Log(rng) - math.Log(rng/(1+(float64(i%3-1)*0.01)))
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m2 := *NewModel()
		m2.Train(obs, 20)
	}
}

func BenchmarkViterbi(b *testing.B) {
	m := NewModel()
	obs := make([]float64, 200)
	rng := 100.0
	for i := range obs {
		rng *= 1 + (float64(i%3-1)*0.01)
		obs[i] = math.Log(rng) - math.Log(rng/(1+(float64(i%3-1)*0.01)))
	}
	
	m.Train(obs, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Viterbi(obs)
	}
}
