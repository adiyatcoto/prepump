// internal/hmm/engine.go
//
// Pure-Go implementation of:
//   1. Gaussian HMM (Baum-Welch EM) — 3-state BULL/BEAR/SIDEWAYS
//   2. DC-HSMM (Directional Change events) — TRENDING vs RANGING
//   3. Viterbi decoding
//   4. Forward algorithm for posterior probabilities

package hmm

import (
	"math"
	"sort"

	"github.com/you/prepump/internal/deepcoin"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type Regime int

const (
	RegimeBull Regime = iota
	RegimeSideways
	RegimeBear
)

func (r Regime) String() string {
	switch r {
	case RegimeBull:
		return "BULL"
	case RegimeSideways:
		return "SIDEWAYS"
	case RegimeBear:
		return "BEAR"
	}
	return "UNKNOWN"
}

type RegimeResult struct {
	Current    Regime
	BullProb   float64
	SideProb   float64
	BearProb   float64
	Confidence float64 // max(bull,side,bear)
	States     []Regime
}

type DCResult struct {
	Label      string  // "TRENDING" or "RANGING"
	Conf       float64 // confidence %
	TrendPct   float64
	RangePct   float64
	NEvents    int
	Dwell      int     // candles since last event
	AvgOS      float64 // average overshoot %
	LikelyNext string
}

// ─── Feature extraction ───────────────────────────────────────────────────────

func logReturns(candles []deepcoin.Candle) []float64 {
	r := make([]float64, len(candles))
	for i := 1; i < len(candles); i++ {
		if candles[i-1].Close > 0 {
			r[i] = math.Log(candles[i].Close / candles[i-1].Close)
		}
	}
	return r
}

func rollStd(returns []float64, window int) []float64 {
	std := make([]float64, len(returns))
	for i := range returns {
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		slice := returns[start : i+1]
		mean := 0.0
		for _, v := range slice {
			mean += v
		}
		mean /= float64(len(slice))
		variance := 0.0
		for _, v := range slice {
			d := v - mean
			variance += d * d
		}
		if len(slice) > 1 {
			std[i] = math.Sqrt(variance / float64(len(slice)-1))
		}
	}
	return std
}

// ─── Gaussian distribution ────────────────────────────────────────────────────

type Gaussian struct {
	Mean float64
	Std  float64
}

func (g Gaussian) LogPDF(x float64) float64 {
	if g.Std < 1e-10 {
		return -1e9
	}
	z := (x - g.Mean) / g.Std
	return -0.5*z*z - math.Log(g.Std) - 0.5*math.Log(2*math.Pi)
}

// ─── HMM Model ───────────────────────────────────────────────────────────────

type Model struct {
	N       int         // number of states
	Pi      []float64   // initial distribution
	A       [][]float64 // transition matrix [from][to]
	Emit    []Gaussian  // emission per state (univariate on log-return)
}

// NewModel initialises a 3-state HMM with sensible defaults for crypto regimes.
// States: 0=BULL, 1=SIDEWAYS, 2=BEAR
func NewModel() *Model {
	return &Model{
		N:  3,
		Pi: []float64{0.33, 0.34, 0.33},
		A: [][]float64{
			{0.80, 0.15, 0.05}, // BULL → mostly stays BULL
			{0.15, 0.70, 0.15}, // SIDEWAYS
			{0.05, 0.15, 0.80}, // BEAR → mostly stays BEAR
		},
		Emit: []Gaussian{
			{Mean: 0.003, Std: 0.012},  // BULL: positive drift
			{Mean: 0.000, Std: 0.008},  // SIDEWAYS: low vol, no drift
			{Mean: -0.003, Std: 0.015}, // BEAR: negative drift, high vol
		},
	}
}

// ─── Baum-Welch EM ────────────────────────────────────────────────────────────

// Train runs Baum-Welch EM for `iters` iterations on the observation sequence.
func (m *Model) Train(obs []float64, iters int) {
	N := m.N
	T := len(obs)
	if T < N+2 {
		return
	}

	for iter := 0; iter < iters; iter++ {
		// Forward
		alpha, logScale := m.forward(obs)
		// Backward
		beta := m.backward(obs, logScale)
		_ = logScale

		// Gamma: P(state i at t | obs)
		gamma := make([][]float64, T)
		for t := range gamma {
			gamma[t] = make([]float64, N)
			sum := 0.0
			for i := 0; i < N; i++ {
				gamma[t][i] = alpha[t][i] * beta[t][i]
				sum += gamma[t][i]
			}
			if sum > 0 {
				for i := 0; i < N; i++ {
					gamma[t][i] /= sum
				}
			}
		}

		// Xi: P(state i at t, state j at t+1 | obs)
		xi := make([][][]float64, T-1)
		for t := 0; t < T-1; t++ {
			xi[t] = make([][]float64, N)
			sum := 0.0
			for i := 0; i < N; i++ {
				xi[t][i] = make([]float64, N)
				for j := 0; j < N; j++ {
					v := alpha[t][i] * m.A[i][j] * m.emitProb(j, obs[t+1]) * beta[t+1][j]
					xi[t][i][j] = v
					sum += v
				}
			}
			if sum > 0 {
				for i := 0; i < N; i++ {
					for j := 0; j < N; j++ {
						xi[t][i][j] /= sum
					}
				}
			}
		}

		// Update Pi
		for i := 0; i < N; i++ {
			m.Pi[i] = gamma[0][i]
		}

		// Update A
		for i := 0; i < N; i++ {
			denom := 0.0
			for t := 0; t < T-1; t++ {
				denom += gamma[t][i]
			}
			for j := 0; j < N; j++ {
				num := 0.0
				for t := 0; t < T-1; t++ {
					num += xi[t][i][j]
				}
				if denom > 0 {
					m.A[i][j] = num / denom
				}
			}
		}

		// Update emission Gaussians
		for i := 0; i < N; i++ {
			denom := 0.0
			for t := 0; t < T; t++ {
				denom += gamma[t][i]
			}
			if denom < 1e-10 {
				continue
			}
			// Mean
			newMean := 0.0
			for t := 0; t < T; t++ {
				newMean += gamma[t][i] * obs[t]
			}
			newMean /= denom
			// Std
			newVar := 0.0
			for t := 0; t < T; t++ {
				d := obs[t] - newMean
				newVar += gamma[t][i] * d * d
			}
			newVar /= denom
			newStd := math.Sqrt(newVar)
			if newStd < 1e-5 {
				newStd = 1e-5
			}
			m.Emit[i] = Gaussian{Mean: newMean, Std: newStd}
		}
	}
}

func (m *Model) emitProb(state int, obs float64) float64 {
	v := math.Exp(m.Emit[state].LogPDF(obs))
	if v < 1e-300 {
		return 1e-300
	}
	return v
}

func (m *Model) forward(obs []float64) ([][]float64, []float64) {
	N := m.N
	T := len(obs)
	alpha := make([][]float64, T)
	scales := make([]float64, T)

	alpha[0] = make([]float64, N)
	sum := 0.0
	for i := 0; i < N; i++ {
		alpha[0][i] = m.Pi[i] * m.emitProb(i, obs[0])
		sum += alpha[0][i]
	}
	scales[0] = sum
	if sum > 0 {
		for i := 0; i < N; i++ {
			alpha[0][i] /= sum
		}
	}

	for t := 1; t < T; t++ {
		alpha[t] = make([]float64, N)
		sum = 0
		for j := 0; j < N; j++ {
			v := 0.0
			for i := 0; i < N; i++ {
				v += alpha[t-1][i] * m.A[i][j]
			}
			alpha[t][j] = v * m.emitProb(j, obs[t])
			sum += alpha[t][j]
		}
		scales[t] = sum
		if sum > 0 {
			for j := 0; j < N; j++ {
				alpha[t][j] /= sum
			}
		}
	}
	return alpha, scales
}

func (m *Model) backward(obs []float64, scales []float64) [][]float64 {
	N := m.N
	T := len(obs)
	beta := make([][]float64, T)
	beta[T-1] = make([]float64, N)
	for i := 0; i < N; i++ {
		beta[T-1][i] = 1.0
	}
	for t := T - 2; t >= 0; t-- {
		beta[t] = make([]float64, N)
		for i := 0; i < N; i++ {
			sum := 0.0
			for j := 0; j < N; j++ {
				sum += m.A[i][j] * m.emitProb(j, obs[t+1]) * beta[t+1][j]
			}
			beta[t][i] = sum
		}
		if s := scales[t]; s > 0 {
			for i := 0; i < N; i++ {
				beta[t][i] /= s
			}
		}
	}
	return beta
}

// ─── Viterbi + labeling ───────────────────────────────────────────────────────

// Viterbi returns the most likely state sequence.
func (m *Model) Viterbi(obs []float64) []int {
	N := m.N
	T := len(obs)
	delta := make([][]float64, T)
	psi   := make([][]int, T)

	delta[0] = make([]float64, N)
	psi[0] = make([]int, N)
	for i := 0; i < N; i++ {
		delta[0][i] = math.Log(m.Pi[i]+1e-300) + m.Emit[i].LogPDF(obs[0])
	}

	for t := 1; t < T; t++ {
		delta[t] = make([]float64, N)
		psi[t] = make([]int, N)
		for j := 0; j < N; j++ {
			best, bestI := math.Inf(-1), 0
			for i := 0; i < N; i++ {
				v := delta[t-1][i] + math.Log(m.A[i][j]+1e-300)
				if v > best {
					best, bestI = v, i
				}
			}
			delta[t][j] = best + m.Emit[j].LogPDF(obs[t])
			psi[t][j] = bestI
		}
	}

	// Backtrack
	path := make([]int, T)
	path[T-1] = 0
	for i := 1; i < N; i++ {
		if delta[T-1][i] > delta[T-1][path[T-1]] {
			path[T-1] = i
		}
	}
	for t := T - 2; t >= 0; t-- {
		path[t] = psi[t+1][path[t+1]]
	}
	return path
}

// labelStates maps raw HMM state indices to BULL/SIDEWAYS/BEAR by mean return.
func (m *Model) labelStates() map[int]Regime {
	type idx struct {
		i    int
		mean float64
	}
	sorted := make([]idx, m.N)
	for i := 0; i < m.N; i++ {
		sorted[i] = idx{i, m.Emit[i].Mean}
	}
	sort.Slice(sorted, func(a, b int) bool {
		return sorted[a].mean < sorted[b].mean
	})
	out := map[int]Regime{}
	out[sorted[0].i] = RegimeBear
	out[sorted[m.N-1].i] = RegimeBull
	for k := 1; k < m.N-1; k++ {
		out[sorted[k].i] = RegimeSideways
	}
	return out
}

// Posterior returns posterior probabilities for each state at the last observation.
func (m *Model) Posterior(obs []float64) []float64 {
	alpha, scales := m.forward(obs)
	_ = scales
	last := alpha[len(alpha)-1]
	sum := 0.0
	for _, v := range last {
		sum += v
	}
	out := make([]float64, m.N)
	if sum > 0 {
		for i := range last {
			out[i] = last[i] / sum
		}
	}
	return out
}

// ─── Public API ───────────────────────────────────────────────────────────────

// AnalyseRegime trains an HMM on candle data and returns regime classification.
func AnalyseRegime(candles []deepcoin.Candle) RegimeResult {
	if len(candles) < 30 {
		return RegimeResult{Current: RegimeSideways, BullProb: 0.33, SideProb: 0.34, BearProb: 0.33}
	}

	returns := logReturns(candles)
	model := NewModel()
	model.Train(returns, 80)

	labeling := model.labelStates()
	path := model.Viterbi(returns)
	posterior := model.Posterior(returns)

	// Map posteriors to named regimes
	bullP, sideP, bearP := 0.0, 0.0, 0.0
	for i, p := range posterior {
		switch labeling[i] {
		case RegimeBull:
			bullP += p
		case RegimeSideways:
			sideP += p
		case RegimeBear:
			bearP += p
		}
	}

	// Current regime from Viterbi last state
	current := labeling[path[len(path)-1]]

	// Build states slice
	states := make([]Regime, len(path))
	for i, s := range path {
		states[i] = labeling[s]
	}

	conf := math.Max(bullP, math.Max(sideP, bearP))
	return RegimeResult{
		Current:    current,
		BullProb:   bullP,
		SideProb:   sideP,
		BearProb:   bearP,
		Confidence: conf,
		States:     states,
	}
}

// ─── DC-HSMM ─────────────────────────────────────────────────────────────────

// AnalyseDC runs Directional Change event analysis on candle closes.
func AnalyseDC(candles []deepcoin.Candle, threshold float64) DCResult {
	if threshold <= 0 {
		threshold = 0.005 // 0.5%
	}

	closes := make([]float64, len(candles))
	for i, c := range candles {
		closes[i] = c.Close
	}

	type Event struct {
		Idx int
		Dir int // +1 up, -1 down
	}

	var events []Event
	direction := 0
	extremeVal := closes[0]

	for i := 1; i < len(closes); i++ {
		c := closes[i]
		if direction != 1 {
			if c >= extremeVal*(1+threshold) {
				events = append(events, Event{i, 1})
				direction = 1
				extremeVal = c
			} else if c < extremeVal {
				extremeVal = c
			}
		}
		if direction != -1 {
			if c <= extremeVal*(1-threshold) {
				events = append(events, Event{i, -1})
				direction = -1
				extremeVal = c
			} else if c > extremeVal {
				extremeVal = c
			}
		}
	}

	if len(events) < 4 {
		return DCResult{
			Label: "RANGING", Conf: 50, TrendPct: 50, RangePct: 50,
			NEvents: len(events), LikelyNext: "RANGING",
		}
	}

	trendCount, rangeCount := 0, 0
	for j := 1; j < len(events); j++ {
		if events[j].Dir == events[j-1].Dir {
			trendCount++
		} else {
			rangeCount++
		}
	}
	total := trendCount + rangeCount
	trendPct := 50.0
	if total > 0 {
		trendPct = float64(trendCount) / float64(total) * 100
	}
	rangePct := 100 - trendPct

	// Dwell: candles since last event
	dwell := len(closes) - events[len(events)-1].Idx

	// Average overshoot
	osTotal := 0.0
	for k := 0; k+1 < len(events); k++ {
		i0, i1 := events[k].Idx, events[k+1].Idx
		if closes[i0] > 0 {
			osTotal += math.Abs(closes[i1]-closes[i0]) / closes[i0]
		}
	}
	avgOS := 0.0
	if len(events) > 1 {
		avgOS = osTotal / float64(len(events)-1) * 100
	}

	label := "RANGING"
	if trendPct >= 50 {
		label = "TRENDING"
	}
	conf := trendPct
	if rangePct > trendPct {
		conf = rangePct
	}
	likelyNext := "RANGING"
	if label == "RANGING" {
		likelyNext = "TRENDING"
	}

	return DCResult{
		Label:      label,
		Conf:       conf,
		TrendPct:   trendPct,
		RangePct:   rangePct,
		NEvents:    len(events),
		Dwell:      dwell,
		AvgOS:      avgOS,
		LikelyNext: likelyNext,
	}
}
