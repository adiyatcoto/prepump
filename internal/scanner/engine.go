// internal/scanner/engine.go — v3
// Adds HasFutures / HasSpot to CoinResult for MKT column in TUI.

package scanner

import (
	"math"
	"sort"
	"time"

	"github.com/you/prepump/internal/deepcoin"
	"github.com/you/prepump/internal/hmm"
	"github.com/you/prepump/internal/pyth"
)

// ─── Config ───────────────────────────────────────────────────────────────────

var Weights = struct {
	Volume, OI, Funding, CVD, RSI, BB, Momentum, HMM, CBPrem float64
}{
	Volume: 15, OI: 12, Funding: 15, CVD: 12,
	RSI: 8, BB: 8, Momentum: 5, HMM: 15, CBPrem: 10,
}

const (
	HotThreshold  = 70.0
	WarmThreshold = 50.0
)

// ─── Math helpers ─────────────────────────────────────────────────────────────

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range vals {
		s += v
	}
	return s / float64(len(vals))
}

func stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := mean(vals)
	v := 0.0
	for _, x := range vals {
		d := x - m
		v += d * d
	}
	return math.Sqrt(v / float64(len(vals)-1))
}

func clip(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ─── RSI ─────────────────────────────────────────────────────────────────────

func calcRSI(prices []float64, period int) []float64 {
	rsi := make([]float64, len(prices))
	if len(prices) < period+1 {
		return rsi
	}
	gains, losses := 0.0, 0.0
	for i := 1; i <= period; i++ {
		d := prices[i] - prices[i-1]
		if d > 0 {
			gains += d
		} else {
			losses -= d
		}
	}
	avgG := gains / float64(period)
	avgL := losses / float64(period)
	for i := period; i < len(prices); i++ {
		if i > period {
			d := prices[i] - prices[i-1]
			if d > 0 {
				avgG = (avgG*float64(period-1) + d) / float64(period)
				avgL = (avgL * float64(period-1)) / float64(period)
			} else {
				avgG = (avgG * float64(period-1)) / float64(period)
				avgL = (avgL*float64(period-1) - d) / float64(period)
			}
		}
		if avgL == 0 {
			rsi[i] = 100
		} else {
			rs := avgG / avgL
			rsi[i] = 100 - 100/(1+rs)
		}
	}
	return rsi
}

// ─── BB Width ─────────────────────────────────────────────────────────────────

func calcBBWidth(prices []float64, period int) []float64 {
	bw := make([]float64, len(prices))
	for i := period; i < len(prices); i++ {
		slice := prices[i-period : i]
		m := mean(slice)
		s := stddev(slice)
		if m != 0 {
			bw[i] = (4 * s) / m
		}
	}
	return bw
}

// ─── Signals ─────────────────────────────────────────────────────────────────

type Signals struct {
	VolScore     float64
	VolRatio     float64
	VolState     string
	CVDScore     float64
	CVDTrend     float64
	RSINow       float64
	RSIScore     float64
	Divergence   bool
	BBScore      float64
	MomScore     float64
	HMMResult    hmm.RegimeResult
	HMMScore     float64
	DCResult     hmm.DCResult
	CBPremPct    float64
	CBPremScore  float64
	CBPremLabel  string
	FundingRate  float64
	FundingScore float64
	FundingAvail bool
	OIScore      float64
	OIAvail      bool
	FGValue      int
	FGLabel      string
	SR           SRLevels
	Proj         Projection
	PumpScore    float64
}

func CalcSignals(
	candles4h []deepcoin.Candle,
	candles1h []deepcoin.Candle,
	fundingRate *deepcoin.FundingRate,
	oi *deepcoin.OpenInterest,
	pythPrice *pyth.Price,
	pythBTC *pyth.Price,
	deepBTCPrice float64,
	fgValue int,
	fgLabel string,
) Signals {
	var s Signals
	s.FGValue = fgValue
	s.FGLabel = fgLabel

	if len(candles4h) < 30 {
		return s
	}

	// Slices
	cls4h := make([]float64, len(candles4h))
	vol4h := make([]float64, len(candles4h))
	for i, c := range candles4h {
		cls4h[i] = c.Close
		vol4h[i] = c.Volume
	}

	// ── Volume ──
	n := len(vol4h)
	recentVol := mean(vol4h[maxI(0, n-3):])
	avgVol := mean(vol4h[maxI(0, n-50):])
	s.VolRatio = recentVol / (avgVol + 1e-9)
	s.VolScore = clip((s.VolRatio-1.0)*50, 0, 100)
	switch {
	case s.VolRatio >= 1.5:
		s.VolState = "surging"
	case s.VolRatio < 0.7:
		s.VolState = "exhausted"
	default:
		s.VolState = "normal"
	}

	// ── CVD ──
	cvd := make([]float64, n)
	cum := 0.0
	for i, c := range candles4h {
		if c.Close >= c.Open {
			cum += c.Volume
		} else {
			cum -= c.Volume
		}
		cvd[i] = cum
	}
	recent50 := cvd[maxI(0, n-50):]
	minR, maxR := recent50[0], recent50[0]
	for _, v := range recent50 {
		if v < minR {
			minR = v
		}
		if v > maxR {
			maxR = v
		}
	}
	cvdNorm := 0.0
	if maxR > minR {
		cvdNorm = (cvd[n-1]-minR)/(maxR-minR)*200 - 100
	}
	s.CVDScore = clip((cvdNorm+100)/2, 0, 100)
	if n >= 21 {
		s.CVDTrend = cvd[n-1] - cvd[n-21]
	}

	// ── RSI ──
	rsi := calcRSI(cls4h, 14)
	s.RSINow = rsi[len(rsi)-1]
	switch {
	case s.RSINow >= 30 && s.RSINow <= 50:
		s.RSIScore = 80
	case s.RSINow > 50 && s.RSINow <= 65:
		s.RSIScore = 60
	case s.RSINow < 30:
		s.RSIScore = 60
	default:
		s.RSIScore = 20
	}
	// Bullish divergence
	if n >= 20 {
		priceRecent := minF(cls4h[n-5 : n])
		pricePrev := minF(cls4h[n-15 : n-5])
		rsiRecent := minF(rsi[n-5 : n])
		rsiPrev := minF(rsi[n-15 : n-5])
		if priceRecent < pricePrev && rsiRecent > rsiPrev {
			s.Divergence = true
			s.RSIScore = math.Min(s.RSIScore+20, 100)
		}
	}

	// ── BB Compression ──
	bw := calcBBWidth(cls4h, 20)
	bwNow := bw[len(bw)-1]
	bwAvg := mean(bw[maxI(0, len(bw)-50):])
	if bwAvg > 0 {
		s.BBScore = clip((1-bwNow/bwAvg)*100, 0, 100)
	}

	// ── Momentum ──
	s.MomScore = calcMomentum(candles4h, candles1h)

	// ── HMM ──
	s.HMMResult = hmm.AnalyseRegime(candles4h)
	s.HMMScore = calcHMMScore(s.HMMResult)
	s.DCResult = hmm.AnalyseDC(candles4h, 0.005)

	// ── Pyth Premium (CBPrem equivalent) ──
	if pythBTC != nil && deepBTCPrice > 0 {
		s.CBPremPct = (pythBTC.Price - deepBTCPrice) / deepBTCPrice * 100
	} else if pythPrice != nil && n > 0 {
		last := cls4h[n-1]
		if last > 0 {
			s.CBPremPct = (pythPrice.Price - last) / last * 100
		}
	}
	s.CBPremScore, s.CBPremLabel = calcCBPremScore(s.CBPremPct)

	// ── Funding (Futures) ──
	if fundingRate != nil {
		s.FundingAvail = true
		s.FundingRate = fundingRate.FundingRate
		fr := s.FundingRate
		switch {
		case fr < -0.001:
			s.FundingScore = 100
		case fr < -0.0005:
			s.FundingScore = 80
		case fr < 0:
			s.FundingScore = 60
		case fr < 0.0005:
			s.FundingScore = 40
		default:
			s.FundingScore = 20
		}
	} else {
		s.FundingScore = 40 // neutral fallback for spot-only
	}

	// ── OI ──
	if oi != nil && oi.OI > 0 {
		s.OIAvail = true
		s.OIScore = 50
	} else {
		s.OIScore = 30
	}

	// ── S/R ──
	s.SR = calcSR(candles4h, 3)

	// ── Projection ──
	s.Proj = calcProjection(candles4h, s.HMMResult, s.DCResult, s.SR)

	// ── Final weighted score ──
	totalW := Weights.Volume + Weights.OI + Weights.Funding + Weights.CVD +
		Weights.RSI + Weights.BB + Weights.Momentum + Weights.HMM + Weights.CBPrem
	s.PumpScore = (s.VolScore*Weights.Volume +
		s.OIScore*Weights.OI +
		s.FundingScore*Weights.Funding +
		s.CVDScore*Weights.CVD +
		s.RSIScore*Weights.RSI +
		s.BBScore*Weights.BB +
		s.MomScore*Weights.Momentum +
		s.HMMScore*Weights.HMM +
		s.CBPremScore*Weights.CBPrem) / totalW

	return s
}

func minF(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func calcMomentum(c4h, c1h []deepcoin.Candle) float64 {
	if len(c1h) < 6 || len(c4h) < 42 {
		return 30
	}
	n := len(c4h)
	n1 := len(c1h)
	chg1h := (c1h[n1-1].Close/c1h[n1-6].Close - 1) * 100
	chg24h := (c4h[n-1].Close/c4h[n-6].Close - 1) * 100
	chg7d := (c4h[n-1].Close/c4h[n-42].Close - 1) * 100
	score := 0.0
	if chg1h > 0 && chg1h < 5 {
		score += 40
	} else if chg1h >= 5 {
		score += 20
	}
	if chg24h > -10 && chg24h < 3 {
		score += 40
	}
	if chg7d < 0 {
		score += 20
	}
	return score
}

func calcHMMScore(r hmm.RegimeResult) float64 {
	base := 45.0
	switch r.Current {
	case hmm.RegimeBull:
		base = 70
	case hmm.RegimeBear:
		base = 15
	}
	return clip(base+(r.BullProb-r.BearProb)*30, 0, 100)
}

func calcCBPremScore(pct float64) (float64, string) {
	switch {
	case pct > 0.1:
		return 90, "STRONG_BULL"
	case pct > 0.05:
		return 75, "MILD_BULL"
	case pct > 0:
		return 60, "MILD_BULL"
	case pct > -0.05:
		return 45, "NEUTRAL"
	default:
		return 25, "MILD_BEAR"
	}
}

// ─── S/R ─────────────────────────────────────────────────────────────────────

type SRLevels struct {
	Supports    []float64
	Resistances []float64
	NextS       float64
	NextSPct    float64
	NextR       float64
	NextRPct    float64
}

func calcSR(candles []deepcoin.Candle, n int) SRLevels {
	if len(candles) < 20 {
		return SRLevels{}
	}
	price := candles[len(candles)-1].Close
	window := 5

	var highs, lows []float64
	for i := window; i < len(candles)-window; i++ {
		isH, isL := true, true
		for k := i - window; k <= i+window; k++ {
			if k == i {
				continue
			}
			if candles[k].High >= candles[i].High {
				isH = false
			}
			if candles[k].Low <= candles[i].Low {
				isL = false
			}
		}
		if isH {
			highs = append(highs, candles[i].High)
		}
		if isL {
			lows = append(lows, candles[i].Low)
		}
	}

	cluster := func(levels []float64, tol float64) []float64 {
		if len(levels) == 0 {
			return nil
		}
		sort.Float64s(levels)
		var cls []float64
		grp := []float64{levels[0]}
		for _, v := range levels[1:] {
			if grp[len(grp)-1] > 0 && (v-grp[len(grp)-1])/grp[len(grp)-1] < tol {
				grp = append(grp, v)
			} else {
				cls = append(cls, mean(grp))
				grp = []float64{v}
			}
		}
		cls = append(cls, mean(grp))
		return cls
	}

	allHighs := cluster(highs, 0.005)
	allLows := cluster(lows, 0.005)

	var supports, resistances []float64
	for _, v := range allLows {
		if v < price {
			supports = append(supports, v)
		}
	}
	for _, v := range allHighs {
		if v > price {
			resistances = append(resistances, v)
		}
	}
	sort.Slice(supports, func(i, j int) bool { return supports[i] > supports[j] })
	sort.Float64s(resistances)

	atr := calcATR(candles, 14)
	for len(supports) < n {
		if len(supports) == 0 {
			supports = append(supports, price-atr)
		} else {
			supports = append(supports, supports[len(supports)-1]*0.98)
		}
	}
	for len(resistances) < n {
		if len(resistances) == 0 {
			resistances = append(resistances, price+atr)
		} else {
			resistances = append(resistances, resistances[len(resistances)-1]*1.02)
		}
	}

	sr := SRLevels{
		Supports:    supports[:n],
		Resistances: resistances[:n],
		NextS:       supports[0],
		NextR:       resistances[0],
	}
	if price > 0 {
		sr.NextSPct = (sr.NextS/price - 1) * 100
		sr.NextRPct = (sr.NextR/price - 1) * 100
	}
	return sr
}

func calcATR(candles []deepcoin.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	trs := make([]float64, len(candles))
	for i := 1; i < len(candles); i++ {
		hl := candles[i].High - candles[i].Low
		hpc := math.Abs(candles[i].High - candles[i-1].Close)
		lpc := math.Abs(candles[i].Low - candles[i-1].Close)
		trs[i] = math.Max(hl, math.Max(hpc, lpc))
	}
	return mean(trs[len(trs)-period:])
}

// ─── Projection ──────────────────────────────────────────────────────────────

type TFProjection struct {
	UpProb    float64
	DownProb  float64
	RangeLow  float64
	RangeHigh float64
	Target    float64
	HasTarget bool
}

type Projection struct {
	H1  TFProjection
	H4  TFProjection
	H24 TFProjection
}

func calcProjection(candles []deepcoin.Candle, regime hmm.RegimeResult, dc hmm.DCResult, sr SRLevels) Projection {
	if len(candles) < 2 {
		return Projection{}
	}
	price := candles[len(candles)-1].Close

	var logRets []float64
	for i := 1; i < len(candles); i++ {
		if candles[i-1].Close > 0 {
			logRets = append(logRets, math.Log(candles[i].Close/candles[i-1].Close))
		}
	}
	volPerCandle := stddev(logRets)
	drift := regime.BullProb*0.003 + regime.BearProb*(-0.003)

	volAmp := 1.0
	if dc.Label == "TRENDING" {
		volAmp = 1.3
	} else if dc.Label == "RANGING" {
		volAmp = 0.8
	}

	upProb := clip(regime.BullProb+drift*10, 0.05, 0.95)

	tf := func(nCandles float64) TFProjection {
		v := volPerCandle * math.Sqrt(nCandles) * volAmp
		hi := price * math.Exp(drift*nCandles+v)
		lo := price * math.Exp(drift*nCandles-v)
		if sr.NextR > 0 {
			hi = math.Min(hi, sr.NextR*1.005)
		}
		if sr.NextS > 0 {
			lo = math.Max(lo, sr.NextS*0.995)
		}
		var target float64
		hasTarget := false
		if regime.BullProb > 0.6 && sr.NextR > 0 {
			target = sr.NextR
			hasTarget = true
		} else if regime.BearProb > 0.6 && sr.NextS > 0 {
			target = sr.NextS
			hasTarget = true
		}
		return TFProjection{
			UpProb: upProb * 100, DownProb: (1 - upProb) * 100,
			RangeLow: lo, RangeHigh: hi,
			Target: target, HasTarget: hasTarget,
		}
	}
	return Projection{H1: tf(0.25), H4: tf(1.0), H24: tf(6.0)}
}

// ─── Trade plan ──────────────────────────────────────────────────────────────

type TradePlan struct {
	Price   float64
	Entry   float64
	SL      float64
	TP1     float64
	TP2     float64
	TP3     float64
	SLPct   float64
	TP1Pct  float64
	TP2Pct  float64
	TP3Pct  float64
	RR      float64
	ATR     float64
	Window  string
	DexLink string
}

func CalcTradePlan(candles []deepcoin.Candle, score float64, sr SRLevels, symbol string) TradePlan {
	if len(candles) == 0 {
		return TradePlan{}
	}
	close := candles[len(candles)-1].Close
	atr := calcATR(candles, 14)

	support := sr.NextS
	if support == 0 {
		support = close * 0.97
	}
	resistance := sr.NextR
	if resistance == 0 {
		resistance = close * 1.05
	}

	entry := math.Max(close, support*1.005)
	sl := support - atr*0.5
	tp1 := entry + atr*1.5
	tp2 := entry + atr*3.0
	tp3 := resistance * 0.98

	rr := 0.0
	if entry-sl > 0 {
		rr = (tp1 - entry) / (entry - sl)
	}

	window := "14-30 hari (speculative)"
	switch {
	case score >= 80:
		window = "1-3 hari"
	case score >= 65:
		window = "3-7 hari"
	case score >= 50:
		window = "7-14 hari"
	}

	pct := func(t float64) float64 {
		if close == 0 {
			return 0
		}
		return (t/close - 1) * 100
	}

	return TradePlan{
		Price:   close,
		Entry:   entry,
		SL:      sl,
		TP1:     tp1,
		TP2:     tp2,
		TP3:     tp3,
		SLPct:   pct(sl),
		TP1Pct:  pct(tp1),
		TP2Pct:  pct(tp2),
		TP3Pct:  pct(tp3),
		RR:      rr,
		ATR:     atr,
		Window:  window,
		DexLink: "https://dexscreener.com/search?q=" + symbol,
	}
}

// ─── Coin result ─────────────────────────────────────────────────────────────

type CoinResult struct {
	Symbol    string
	HasSpot   bool
	HasFutures bool
	VolUSD    float64
	Change24h float64
	Signals   Signals
	Plan      TradePlan
	UpdatedAt time.Time
}

func (c CoinResult) ScoreLabel() string {
	switch {
	case c.Signals.PumpScore >= HotThreshold:
		return "HOT"
	case c.Signals.PumpScore >= WarmThreshold:
		return "WARM"
	default:
		return "EARLY"
	}
}