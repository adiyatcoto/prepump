// internal/tui/tui.go — v4
//
// Changes from v3:
//   ✓ Sort by PumpScore only (no active/dimmed separation)
//   ✓ UI repaints every 1s reliably via MsgUITick → tea.Batch
//   ✓ MsgTickerUpdate now carries full SignalSnapshot for live stats
//   ✓ Deep view reads live price AND live signal deltas from liveData map
//   ✓ Price flash animation on change (▲ green / ▼ red indicator)

package tui

import (
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/you/prepump/internal/scanner"
)

// ─── Colours ─────────────────────────────────────────────────────────────────

var (
	colBG      = lipgloss.Color("#0f172a")
	colSurface = lipgloss.Color("#1e293b")
	colBorder  = lipgloss.Color("#334155")
	colMuted   = lipgloss.Color("#475569")
	colDimText = lipgloss.Color("#2d3748")
	colText    = lipgloss.Color("#e2e8f0")
	colGreen   = lipgloss.Color("#22c55e")
	colRed     = lipgloss.Color("#ef4444")
	colYellow  = lipgloss.Color("#f59e0b")
	colCyan    = lipgloss.Color("#22d3ee")
	colOrange  = lipgloss.Color("#fb923c")
	colWhite   = lipgloss.Color("#ffffff")

	styleHeader = lipgloss.NewStyle().Background(colSurface).Foreground(colCyan).Bold(true).Padding(0, 1)
	styleFooter = lipgloss.NewStyle().Background(colSurface).Foreground(colMuted).Padding(0, 1)
	styleHot    = lipgloss.NewStyle().Background(colRed).Foreground(colWhite).Bold(true).Padding(0, 1)
	styleWarm   = lipgloss.NewStyle().Background(colYellow).Foreground(lipgloss.Color("#000")).Bold(true).Padding(0, 1)
	styleEarly  = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	styleBull   = lipgloss.NewStyle().Foreground(colGreen).Bold(true)
	styleBear   = lipgloss.NewStyle().Foreground(colRed).Bold(true)
	styleSide   = lipgloss.NewStyle().Foreground(colYellow)
	styleGreen  = lipgloss.NewStyle().Foreground(colGreen)
	styleRed    = lipgloss.NewStyle().Foreground(colRed)
	styleYellow = lipgloss.NewStyle().Foreground(colYellow)
	styleCyan   = lipgloss.NewStyle().Foreground(colCyan)
	styleMuted  = lipgloss.NewStyle().Foreground(colMuted)
	styleDim    = lipgloss.NewStyle().Foreground(colDimText)
	styleBold   = lipgloss.NewStyle().Foreground(colText).Bold(true)
	styleOrange = lipgloss.NewStyle().Foreground(colOrange)

	styleBullRow = lipgloss.NewStyle().Foreground(colGreen).Background(lipgloss.Color("#052e16"))
	styleDimRow  = lipgloss.NewStyle().Foreground(colDimText).Background(colBG)
	styleSel     = lipgloss.NewStyle().Foreground(lipgloss.Color("#000")).Background(colCyan).Bold(true)
	styleSelDim  = lipgloss.NewStyle().Foreground(colMuted).Background(colBorder).Bold(true)
)

// ─── Messages ─────────────────────────────────────────────────────────────────

type MsgScanDone struct {
	Results  []scanner.CoinResult
	BTCPrice float64
	FGValue  int
	FGLabel  string
}

// MsgTickerUpdate carries a live price tick (sub-second from SSE/WS).
// OldPrice is used to render ▲/▼ flash indicator.
type MsgTickerUpdate struct {
	Symbol   string
	Price    float64
	OldPrice float64
}

// MsgLiveStats carries a full live signal snapshot from a background goroutine
// (e.g. a 500ms re-calc on fresh 1m candles). Optional — only sent when available.
type MsgLiveStats struct {
	Symbol  string
	Signals scanner.Signals
}

type MsgError  struct{ Err error }
type MsgTick   time.Time
type MsgUITick time.Time

// ─── Live price state ─────────────────────────────────────────────────────────

type liveTick struct {
	price    float64
	prevPrice float64
	updatedAt time.Time
}

// ─── Persistent coin entry ────────────────────────────────────────────────────

type coinEntry struct {
	result      scanner.CoinResult
	active      bool
	firstSeen   time.Time // wall-clock time of very first scan appearance
	lastSeen    time.Time
	appearCount int // total scans this coin appeared in
	streak      int // consecutive scan streak (resets on disappearance)

	// Bull regime tracking across scans
	bullStreak    int // consecutive scans where HMM = BULL
	bullStreakMax int // peak bull streak ever reached for this coin

	// liveSignals is set when a MsgLiveStats arrives between scans
	liveSignals *scanner.Signals
}

type viewMode int

const (
	viewTable viewMode = iota
	viewDeep
)

// ─── Model ────────────────────────────────────────────────────────────────────

type Model struct {
	mode     viewMode
	selected int
	deepSym  string

	coinMap map[string]*coinEntry
	display []*coinEntry

	lastScan  time.Time
	scanning  bool
	errMsg    string
	scanCount int

	width, height int
	scrollOffset  int

	btcPrice float64
	fgValue  int
	fgLabel  string

	// livePrices holds sub-second price ticks keyed by symbol (e.g. "BTC-USDT")
	livePrices map[string]liveTick

	spin   spinner.Model
	ScanFn func() tea.Msg
}

func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colCyan)
	return Model{
		coinMap:    map[string]*coinEntry{},
		livePrices: map[string]liveTick{},
		spin:       sp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, uiTick(), tea.EnterAltScreen)
}

func uiTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return MsgUITick(t) })
}

func autoScanTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return MsgTick(t) })
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case MsgScanDone:
		m.scanning = false
		m.scanCount++
		m.btcPrice = msg.BTCPrice
		m.fgValue = msg.FGValue
		m.fgLabel = msg.FGLabel
		m.lastScan = time.Now()
		m.errMsg = ""
		m.mergeScan(msg.Results)
		m.rebuildDisplay()
		return m, autoScanTick()

	case MsgError:
		m.scanning = false
		m.errMsg = msg.Err.Error()
		return m, autoScanTick()

	case MsgTickerUpdate:
		prev := m.livePrices[msg.Symbol]
		m.livePrices[msg.Symbol] = liveTick{
			price:     msg.Price,
			prevPrice: prev.price,
			updatedAt: time.Now(),
		}
		// Also update BTC header price inline
		if msg.Symbol == "BTC-USDT" {
			m.btcPrice = msg.Price
		}
		// No rebuildDisplay — UI tick handles repaint

	case MsgLiveStats:
		if e, ok := m.coinMap[msg.Symbol]; ok {
			sigs := msg.Signals
			e.liveSignals = &sigs
		}

	// MsgUITick: repaint every second. Spinner advances via its own tick.
	case MsgUITick:
		return m, uiTick()

	case MsgTick:
		if !m.scanning && m.ScanFn != nil {
			m.scanning = true
			return m, tea.Cmd(m.ScanFn)
		}
		return m, autoScanTick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}
	return m, nil
}

// mergeScan upserts scan results into coinMap.
func (m *Model) mergeScan(results []scanner.CoinResult) {
	// Mark all inactive; reset per-scan streak but keep bull streak
	for _, e := range m.coinMap {
		e.active = false
		e.streak = 0
		// Bull streak resets if coin doesn't re-appear (handled below)
	}
	// Upsert new results
	for _, r := range results {
		isBull := r.Signals.HMMResult.Current.String() == "BULL"
		if e, ok := m.coinMap[r.Symbol]; ok {
			e.result = r
			e.active = true
			e.lastSeen = time.Now()
			e.appearCount++
			e.streak++
			e.liveSignals = nil // new scan data is authoritative

			// Bull streak: increment if still BULL, else reset
			if isBull {
				e.bullStreak++
				if e.bullStreak > e.bullStreakMax {
					e.bullStreakMax = e.bullStreak
				}
			} else {
				e.bullStreak = 0
			}
		} else {
			bs := 0
			if isBull {
				bs = 1
			}
			m.coinMap[r.Symbol] = &coinEntry{
				result:        r,
				active:        true,
				firstSeen:     time.Now(),
				lastSeen:      time.Now(),
				appearCount:   1,
				streak:        1,
				bullStreak:    bs,
				bullStreakMax: bs,
			}
		}
	}
	// Coins that didn't appear this scan: reset their bull streak
	for _, e := range m.coinMap {
		if !e.active {
			e.bullStreak = 0
		}
	}
	// Expire coins unseen > 10 minutes
	for sym, e := range m.coinMap {
		if !e.active && time.Since(e.lastSeen) > 10*time.Minute {
			delete(m.coinMap, sym)
		}
	}
}

// rebuildDisplay sorts ALL entries by PumpScore descending, regardless of
// active/dimmed status. Dimmed rows appear inline at their natural score rank.
func (m *Model) rebuildDisplay() {
	all := make([]*coinEntry, 0, len(m.coinMap))
	for _, e := range m.coinMap {
		all = append(all, e)
	}
	sort.Slice(all, func(i, j int) bool {
		si := all[i].result.Signals.PumpScore
		sj := all[j].result.Signals.PumpScore
		if si != sj {
			return si > sj
		}
		// Tiebreak: active before dimmed, then by symbol
		if all[i].active != all[j].active {
			return all[i].active
		}
		return all[i].result.Symbol < all[j].result.Symbol
	})
	m.display = all
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visRows := m.visibleRows()

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "r":
		if !m.scanning && m.ScanFn != nil {
			m.scanning = true
			return m, tea.Cmd(m.ScanFn)
		}

	case "up", "k":
		if m.mode == viewTable && m.selected > 0 {
			m.selected--
			if m.selected < m.scrollOffset {
				m.scrollOffset = m.selected
			}
		}

	case "down", "j":
		if m.mode == viewTable {
			if m.selected < len(m.display)-1 {
				m.selected++
			}
			if m.selected >= m.scrollOffset+visRows {
				m.scrollOffset = m.selected - visRows + 1
			}
		}

	case "pgup":
		m.selected -= visRows
		if m.selected < 0 {
			m.selected = 0
		}
		m.scrollOffset = m.selected

	case "pgdown":
		m.selected += visRows
		if m.selected >= len(m.display) {
			m.selected = len(m.display) - 1
		}
		if m.selected >= m.scrollOffset+visRows {
			m.scrollOffset = m.selected - visRows + 1
		}

	case "enter":
		if m.mode == viewTable && m.selected < len(m.display) {
			m.deepSym = m.display[m.selected].result.Symbol
			m.mode = viewDeep
		}

	case "esc", "backspace":
		m.mode = viewTable
		m.deepSym = ""

	case "d":
		link := ""
		if m.mode == viewDeep {
			if e, ok := m.coinMap[m.deepSym]; ok {
				link = e.result.Plan.DexLink
			}
		} else if m.selected < len(m.display) {
			link = m.display[m.selected].result.Plan.DexLink
		}
		if link != "" {
			openBrowser(link)
		}
	}

	return m, nil
}

func (m Model) visibleRows() int {
	v := m.height - 10
	if v < 5 {
		return 5
	}
	return v
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return "initialising…"
	}
	header := m.renderHeader()
	footer := m.renderFooter()
	var body string
	if m.mode == viewDeep {
		body = m.renderDeep()
	} else {
		body = m.renderTable()
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) renderHeader() string {
	btcStr := ""
	if m.btcPrice > 0 {
		tick := m.livePrices["BTC-USDT"]
		price := m.btcPrice
		if tick.price > 0 {
			price = tick.price
		}
		arrow := priceArrow(tick)
		btcStr = styleGreen.Render("BTC $"+commaFloat(price, 2)) + arrow
	}
	fgStr := ""
	if m.fgLabel != "" {
		fc := colGreen
		if m.fgValue <= 30 {
			fc = colRed
		} else if m.fgValue <= 50 {
			fc = colYellow
		}
		fgStr = "F&G: " + lipgloss.NewStyle().Foreground(fc).Render(
			fmt.Sprintf("%d %s", m.fgValue, m.fgLabel))
	}
	status := ""
	if m.scanning {
		status = m.spin.View() + styleMuted.Render(" scanning…")
	} else if !m.lastScan.IsZero() {
		age := int(time.Since(m.lastScan).Seconds())
		status = styleMuted.Render(fmt.Sprintf("scan #%d  %ds ago  %d coins tracked",
			m.scanCount, age, len(m.coinMap)))
	}
	if m.errMsg != "" {
		status = styleRed.Render("⚠ " + truncate(m.errMsg, 60))
	}

	parts := []string{
		styleBold.Render("⚡ PRE-PUMP v4"),
		styleMuted.Render("Deepcoin·Spot+Futures  Pyth·SSE  HMM·DC-HSMM"),
	}
	if btcStr != "" {
		parts = append(parts, btcStr)
	}
	if fgStr != "" {
		parts = append(parts, fgStr)
	}
	if status != "" {
		parts = append(parts, status)
	}
	return styleHeader.Width(m.width).Render(strings.Join(parts, "  ·  "))
}

func (m Model) renderFooter() string {
	keys := "↑↓/jk  PgUp/Dn  ENTER=detail  d=DexScreener  r=rescan  q=quit    ★=3+ scans  BULL#=bull streak (* per scan)  AGE=first seen"
	if m.mode == viewDeep {
		keys = "ESC=back  d=DexScreener  r=rescan  q=quit"
	}
	ts := time.Now().Format("15:04:05")

	// Show live tick count in footer
	liveCount := 0
	for _, lt := range m.livePrices {
		if time.Since(lt.updatedAt) < 5*time.Second {
			liveCount++
		}
	}
	liveStr := ""
	if liveCount > 0 {
		liveStr = styleCyan.Render(fmt.Sprintf("  ⬤ %d live", liveCount))
	}

	gap := m.width - len(keys) - len(ts) - 4
	if gap < 1 {
		gap = 1
	}
	return styleFooter.Width(m.width).Render(
		keys + liveStr + strings.Repeat(" ", gap) + ts)
}

// ─── Table ────────────────────────────────────────────────────────────────────

func (m Model) renderTable() string {
	if m.scanning && len(m.display) == 0 {
		return lipgloss.Place(m.width, m.height-6,
			lipgloss.Center, lipgloss.Center,
			m.spin.View()+" fetching market data…")
	}
	if len(m.display) == 0 {
		return lipgloss.Place(m.width, m.height-6,
			lipgloss.Center, lipgloss.Center,
			styleMuted.Render("no data — press r to scan"))
	}

	var sb strings.Builder

	type col struct {
		t string
		w int
	}
	cols := []col{
		{"#", 4}, {"COIN", 11}, {"MKT", 4}, {"SCORE", 6}, {"STAT", 5},
		{"HMM", 8}, {"BULL%", 6}, {"BULL#", 5}, {"PRICE", 14}, {"CHG", 5}, {"24H%", 7},
		{"TP1%", 7}, {"SL%", 7}, {"R:R", 5},
		{"RSI", 5}, {"BB", 5}, {"★", 2}, {"AGE", 8}, {"WINDOW", 14},
	}

	hdrParts := []string{}
	for _, c := range cols {
		hdrParts = append(hdrParts, fmt.Sprintf("%-*s", c.w, c.t))
	}
	sb.WriteString(styleHeader.Background(colSurface).Render(
		" "+strings.Join(hdrParts, " ")) + "\n")
	sb.WriteString(styleDim.Render(strings.Repeat("─", m.width)) + "\n")

	visRows := m.visibleRows()

	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if len(m.display) > visRows && m.scrollOffset > len(m.display)-visRows {
		m.scrollOffset = len(m.display) - visRows
	}

	end := m.scrollOffset + visRows
	if end > len(m.display) {
		end = len(m.display)
	}

	for i := m.scrollOffset; i < end; i++ {
		e := m.display[i]
		r := e.result
		s := r.Signals

		// Apply live signals if available (between-scan update)
		if e.liveSignals != nil {
			s = *e.liveSignals
		}

		// Apply live price tick
		tick := m.livePrices[r.Symbol]
		livePrice := r.Plan.Price
		if tick.price > 0 {
			livePrice = tick.price
		}

		isBull := s.HMMResult.Current.String() == "BULL" && e.active
		isDim := !e.active
		isSel := i == m.selected

		mkt := "S"
		if r.HasFutures && r.HasSpot {
			mkt = "B"
		} else if r.HasFutures {
			mkt = "F"
		}

		starStr := " "
		if e.appearCount >= 3 && e.active {
			starStr = "★"
		}

		// ── Bull streak badge ──────────────────────────────────────────────
		// Shows consecutive scans where HMM = BULL (each dot = 1 scan).
		// Rendered after padding to avoid ANSI/multi-byte width corruption.
		var bullStreakRendered string
		if e.active && e.bullStreak > 0 {
			n := e.bullStreak
			if n > 5 {
				n = 5
			}
			filled := strings.Repeat("*", n)
			empty := strings.Repeat("-", 5-n)
			col := colMuted
			if e.bullStreak >= 3 {
				col = colGreen
			} else if e.bullStreak >= 2 {
				col = colYellow
			}
			bullStreakRendered = lipgloss.NewStyle().Foreground(col).Render(filled + empty)
		} else {
			bullStreakRendered = styleDim.Render("-----")
		}

// ── Coin age since firstSeen ───────────────────────────────────────
		fs := e.firstSeen
		if fs.IsZero() {
			fs = e.lastSeen // graceful fallback for pre-existing entries
		}
		ageStr := formatAge(fs)
		// Colour: new (< 5 min) = cyan, young (< 30 min) = yellow, old = muted
		var ageStyled string
		age := time.Since(fs)
		switch {
		case age < 5*time.Minute:
			ageStyled = styleCyan.Render(fmt.Sprintf("%-8s", ageStr)) // brand new
		case age < 30*time.Minute:
			ageStyled = styleYellow.Render(fmt.Sprintf("%-8s", ageStr)) // young
		default:
			ageStyled = styleMuted.Render(fmt.Sprintf("%-8s", ageStr))
		}

		// Price change arrow (flash indicator — visible only when recently updated)
		chgStr := "    "
		if time.Since(tick.updatedAt) < 3*time.Second && tick.prevPrice > 0 {
			if tick.price > tick.prevPrice {
				chgStr = styleGreen.Render(" ▲  ")
			} else if tick.price < tick.prevPrice {
				chgStr = styleRed.Render(" ▼  ")
			}
		}

		fields := []string{
			fmt.Sprintf("%-4d", i+1),
			fmt.Sprintf("%-11s", truncate(strings.Replace(r.Symbol, "-USDT", "", 1), 11)),
			fmt.Sprintf("%-4s", mkt),
			fmt.Sprintf("%-6.1f", s.PumpScore),
			fmt.Sprintf("%-5s", truncate(r.ScoreLabel(), 5)),
			fmt.Sprintf("%-8s", truncate(s.HMMResult.Current.String(), 8)),
			fmt.Sprintf("%-6s", fmt.Sprintf("%.0f%%", s.HMMResult.BullProb*100)),
			bullStreakRendered, // already 5 chars wide, pre-rendered
			fmt.Sprintf("%-14s", "$"+commaFloat(livePrice, priceDecimals(livePrice))),
			chgStr,
			fmt.Sprintf("%-7s", fmt.Sprintf("%+.1f%%", r.Change24h)),
			fmt.Sprintf("%-7s", fmt.Sprintf("%+.1f%%", r.Plan.TP1Pct)),
			fmt.Sprintf("%-7s", fmt.Sprintf("%.1f%%", r.Plan.SLPct)),
			fmt.Sprintf("%-5s", fmt.Sprintf("%.1f", r.Plan.RR)),
			fmt.Sprintf("%-5s", fmt.Sprintf("%.0f", s.RSINow)),
			fmt.Sprintf("%-5s", fmt.Sprintf("%.0f", s.BBScore)),
			fmt.Sprintf("%-2s", starStr),
			ageStyled,
			truncate(translateWindow(r.Plan.Window), 14),
		}

		line := " " + strings.Join(fields, " ")

		var rendered string
		switch {
		case isSel && isDim:
			rendered = styleSelDim.Width(m.width).Render(line)
		case isSel:
			rendered = styleSel.Width(m.width).Render(line)
		case isDim:
			rendered = styleDimRow.Width(m.width).Render(line)
		case isBull:
			rendered = styleBullRow.Width(m.width).Render(line)
		default:
			rendered = lipgloss.NewStyle().Background(colBG).Foreground(colText).
				Width(m.width).Render(line)
		}

		sb.WriteString(rendered + "\n")
	}

	active := 0
	for _, e := range m.coinMap {
		if e.active {
			active++
		}
	}
	sb.WriteString(styleMuted.Render(fmt.Sprintf(
		"  %d/%d active  %d dimmed  row %d/%d  [PgUp/PgDn]  sorted by score",
		active, len(m.coinMap), len(m.coinMap)-active,
		m.selected+1, len(m.display),
	)))

	return sb.String()
}

// ─── Deep View ────────────────────────────────────────────────────────────────

func (m Model) renderDeep() string {
	e, ok := m.coinMap[m.deepSym]
	if !ok {
		return styleMuted.Render("\n  coin not found — press ESC")
	}

	r := e.result

	// Apply live signals if available
	s := r.Signals
	if e.liveSignals != nil {
		s = *e.liveSignals
	}
	p := r.Plan

	// Apply live price — and recompute plan levels so TP/SL/Entry/RR stay current
	tick := m.livePrices[r.Symbol]
	if tick.price > 0 && tick.price != p.Price {
		p = recomputePlan(p, tick.price, s.SR)
	}

	// Price direction flash
	priceFlash := ""
	if time.Since(tick.updatedAt) < 3*time.Second && tick.prevPrice > 0 {
		if tick.price > tick.prevPrice {
			priceFlash = styleGreen.Render(" ▲ " + fmt.Sprintf("(was $%s)", commaFloat(tick.prevPrice, priceDecimals(tick.prevPrice))))
		} else if tick.price < tick.prevPrice {
			priceFlash = styleRed.Render(" ▼ " + fmt.Sprintf("(was $%s)", commaFloat(tick.prevPrice, priceDecimals(tick.prevPrice))))
		}
	}

	// Tick age indicator
	tickAge := ""
	if !tick.updatedAt.IsZero() {
		ms := time.Since(tick.updatedAt).Milliseconds()
		if ms < 1000 {
			tickAge = styleCyan.Render(fmt.Sprintf("  ⬤ live (%dms ago)", ms))
		} else {
			tickAge = styleMuted.Render(fmt.Sprintf("  ○ %ds ago", ms/1000))
		}
	}

	sep := "  " + styleMuted.Render(strings.Repeat("─", minI(m.width-6, 82))) + "\n"
	coinName := strings.Replace(r.Symbol, "-USDT", "", 1)

	var mktBadge string
	if r.HasFutures && r.HasSpot {
		mktBadge = styleYellow.Render("[SPOT+FUTURES]")
	} else if r.HasFutures {
		mktBadge = styleOrange.Render("[FUTURES]")
	} else {
		mktBadge = styleMuted.Render("[SPOT]")
	}

	activeBadge := ""
	if !e.active {
		activeBadge = styleDim.Render("  ⚠ not in latest scan")
	}
	streakBadge := ""
	if e.appearCount >= 3 && e.active {
		streakBadge = styleOrange.Render(fmt.Sprintf("  ★×%d scans", e.appearCount))
	}
	liveBadge := ""
	if e.liveSignals != nil {
		liveBadge = styleCyan.Render("  ⬤ signals live")
	}

	// ── Bull streak badge ────────────────────────────────────────────────────
	bullBadge := ""
	if e.bullStreak >= 2 {
		n := e.bullStreak
		if n > 5 {
			n = 5
		}
		dots := strings.Repeat("*", n)
		col := colYellow
		if e.bullStreak >= 3 {
			col = colGreen
		}
		bullBadge = "  " + lipgloss.NewStyle().Foreground(col).Bold(true).
			Render(fmt.Sprintf("BULL*%d %s", e.bullStreak, dots))
	} else if e.bullStreakMax >= 3 && e.bullStreak < 2 {
		bullBadge = styleMuted.Render(fmt.Sprintf("  (peak BULL*%d)", e.bullStreakMax))
	}

	// ── Age badge ────────────────────────────────────────────────────────────
	fs := e.firstSeen
	if fs.IsZero() {
		fs = e.lastSeen
	}
	ageDur := time.Since(fs)
	var ageBadge string
	switch {
	case ageDur < 5*time.Minute:
		ageBadge = styleCyan.Render(fmt.Sprintf("  🆕 NEW (%s)", formatAge(fs)))
	case ageDur < 30*time.Minute:
		ageBadge = styleYellow.Render(fmt.Sprintf("  ⏱ young (%s)", formatAge(fs)))
	default:
		ageBadge = styleMuted.Render(fmt.Sprintf("  ⏱ %s", formatAge(fs)))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  %s  %s  %s  %s  %.1f/100%s%s%s%s%s\n",
		styleBold.Render("▶ "+coinName+" / USDT"),
		mktBadge,
		renderScoreBadge(s.PumpScore),
		styleMuted.Render(fmt.Sprintf("updated %s", e.result.UpdatedAt.Format("15:04:05"))),
		s.PumpScore,
		streakBadge,
		bullBadge,
		ageBadge,
		activeBadge,
		liveBadge,
	))
	sb.WriteString("\n")

	// HMM
	sb.WriteString(fmt.Sprintf("  HMM: %s (%d%% conf)  bull:%s  side:%s  bear:%s\n",
		renderRegimeBadge(s.HMMResult.Current.String()),
		int(math.Round(s.HMMResult.Confidence*100)),
		styleGreen.Render(fmt.Sprintf("%.0f%%", s.HMMResult.BullProb*100)),
		styleYellow.Render(fmt.Sprintf("%.0f%%", s.HMMResult.SideProb*100)),
		styleRed.Render(fmt.Sprintf("%.0f%%", s.HMMResult.BearProb*100)),
	))

	// Bull streak detail
	if e.bullStreak > 0 || e.bullStreakMax > 0 {
		bullBar := renderBullBar(e.bullStreak, e.bullStreakMax)
		sb.WriteString(fmt.Sprintf("  Bull streak : current %s×%d  peak ×%d  (each ● = 1 scan)\n",
			bullBar, e.bullStreak, e.bullStreakMax))
	}

	// Coin age / first seen detail
	sb.WriteString(fmt.Sprintf("  First seen  : %s  (%s ago)  ·  appeared in %d scan(s)\n",
		styleMuted.Render(fs.Format("02 Jan 15:04:05")),
		formatAge(fs),
		e.appearCount,
	))

	// Volume
	vc := styleGreen
	if s.VolState == "exhausted" {
		vc = styleRed
	} else if s.VolState == "normal" {
		vc = styleMuted
	}
	sb.WriteString(fmt.Sprintf("  Volume: %.2fx avg  →  %s\n", s.VolRatio, vc.Render(s.VolState)))

	// DC-HSMM
	dcCol := styleGreen
	if s.DCResult.Label == "RANGING" {
		dcCol = styleYellow
	}
	sb.WriteString(fmt.Sprintf(
		"  DC-HSMM: %s (%.0f%% conf, dwell:%d, ~0 more)  θ=%.2f%%  ·  %d events  ·  trend:%.0f%% / range:%.0f%%  →  likely %s\n",
		dcCol.Render(s.DCResult.Label),
		s.DCResult.Conf, s.DCResult.Dwell, s.DCResult.AvgOS,
		s.DCResult.NEvents, s.DCResult.TrendPct, s.DCResult.RangePct,
		styleCyan.Render(s.DCResult.LikelyNext),
	))

	// CBPrem
	cbCol := styleGreen
	if strings.Contains(s.CBPremLabel, "BEAR") {
		cbCol = styleRed
	} else if s.CBPremLabel == "NEUTRAL" {
		cbCol = styleMuted
	}
	sb.WriteString(fmt.Sprintf("  CBPrem: %s  %+.3f%%\n",
		cbCol.Render(s.CBPremLabel), s.CBPremPct))

	// Funding
	if s.FundingAvail {
		fc := styleGreen
		if s.FundingRate > 0.001 {
			fc = styleRed
		}
		sb.WriteString(fmt.Sprintf("  Funding: %s\n",
			fc.Render(fmt.Sprintf("%+.5f", s.FundingRate))))
	} else {
		sb.WriteString("  Funding: " + styleMuted.Render("N/A (spot only)") + "\n")
	}

	// F&G
	fgCol := styleGreen
	if s.FGValue <= 30 {
		fgCol = styleRed
	} else if s.FGValue <= 50 {
		fgCol = styleMuted
	}
	sb.WriteString(fmt.Sprintf("  F&G:   %s  (%s)\n",
		fgCol.Render(strconv.Itoa(s.FGValue)), fgCol.Render(s.FGLabel)))

	sb.WriteString(sep)

	// S/R
	srSup, srRes := "", ""
	for _, sv := range s.SR.Supports {
		srSup += styleGreen.Render("$"+commaFloat(sv, priceDecimals(sv))) + "   "
	}
	for _, rv := range s.SR.Resistances {
		srRes += styleRed.Render("$"+commaFloat(rv, priceDecimals(rv))) + "   "
	}
	sb.WriteString(fmt.Sprintf("  Support:    %s\n", srSup))
	sb.WriteString(fmt.Sprintf("  Resistance: %s\n", srRes))
	sb.WriteString(fmt.Sprintf("    next S:   %s (%s)\n",
		styleGreen.Render("$"+commaFloat(s.SR.NextS, priceDecimals(s.SR.NextS))),
		styleGreen.Render(fmt.Sprintf("%+.2f%%", s.SR.NextSPct))))
	sb.WriteString(fmt.Sprintf("    next R:   %s (%s)\n",
		styleRed.Render("$"+commaFloat(s.SR.NextR, priceDecimals(s.SR.NextR))),
		styleRed.Render(fmt.Sprintf("%+.2f%%", s.SR.NextRPct))))

	sb.WriteString(sep)

	// Projection
	sb.WriteString("  " + styleBold.Render("PROJECTION") +
		"  " + styleMuted.Render("(HMM-weighted, vol-amplified, S/R-anchored):") + "\n")
	rfn := func(lbl string, tf scanner.TFProjection) {
		uc := styleGreen
		if tf.UpProb < 50 {
			uc = styleRed
		}
		tgt := ""
		if tf.HasTarget {
			tgt = "  →  target " + styleCyan.Render("$"+commaFloat(tf.Target, priceDecimals(tf.Target)))
		}
		sb.WriteString(fmt.Sprintf("    Next %-4s  %s vs %.1f%% down    range $%s–$%s%s\n",
			lbl,
			uc.Render(fmt.Sprintf("%.1f%% UP", tf.UpProb)),
			tf.DownProb,
			commaFloat(tf.RangeLow, priceDecimals(tf.RangeLow)),
			commaFloat(tf.RangeHigh, priceDecimals(tf.RangeHigh)),
			tgt,
		))
	}
	rfn("1h", s.Proj.H1)
	rfn("4h", s.Proj.H4)
	rfn("24h", s.Proj.H24)

	sb.WriteString(sep)

	// Signals breakdown
	sb.WriteString("  " + styleBold.Render("SIGNALS") + "\n")
	sig := func(name string, score float64, extra string) {
		bar := renderBar(score, 100, 18)
		sb.WriteString(fmt.Sprintf("    %-14s %s %5.1f/100  %s\n",
			name, bar, score, styleMuted.Render(extra)))
	}
	sig("Volume", s.VolScore, fmt.Sprintf("%.2fx avg  %s", s.VolRatio, s.VolState))
	sig("CVD", s.CVDScore, fmt.Sprintf("trend %+.0f", s.CVDTrend))
	sig("RSI", s.RSIScore, fmt.Sprintf("%.1f%s", s.RSINow, divStr(s.Divergence)))
	sig("BB Compr", s.BBScore, "")
	sig("Momentum", s.MomScore, "")
	sig("HMM", s.HMMScore, s.HMMResult.Current.String())
	sig("CBPrem", s.CBPremScore, fmt.Sprintf("%+.3f%%  %s", s.CBPremPct, s.CBPremLabel))
	sig("Funding", s.FundingScore, fundingStr(s))
	sig("OI", s.OIScore, oiStr(s))

	sb.WriteString(sep)

	// Trade plan
	sb.WriteString("  " + styleBold.Render("TRADING PLAN") + "\n")
	prFmt := func(v float64) string { return commaFloat(v, priceDecimals(v)) }
	sb.WriteString(fmt.Sprintf("  Current Price : $%s%s%s\n",
		prFmt(p.Price), priceFlash, tickAge))
	sb.WriteString(fmt.Sprintf("  %s : $%s\n",
		styleYellow.Render("Entry        "), prFmt(p.Entry)))
	sb.WriteString(fmt.Sprintf("  %s : $%s  (+%.1f%%)\n",
		styleGreen.Render("TP1          "), prFmt(p.TP1), p.TP1Pct))
	sb.WriteString(fmt.Sprintf("  %s : $%s  (+%.1f%%)\n",
		styleGreen.Render("TP2          "), prFmt(p.TP2), p.TP2Pct))
	sb.WriteString(fmt.Sprintf("  %s : $%s  (+%.1f%%)\n",
		styleGreen.Render("TP3          "), prFmt(p.TP3), p.TP3Pct))
	sb.WriteString(fmt.Sprintf("  %s : $%s  (%.1f%%)\n",
		styleRed.Render("Stop Loss    "), prFmt(p.SL), p.SLPct))
	sb.WriteString(fmt.Sprintf("  Risk:Reward   : 1:%.2f\n", p.RR))
	sb.WriteString(fmt.Sprintf("  Pump Window   : %s\n", translateWindow(p.Window)))
	sb.WriteString(fmt.Sprintf("  DexScreener   : %s\n", styleCyan.Render(p.DexLink)))

	sb.WriteString(sep)
	sb.WriteString("  " + styleMuted.Render(
		"Disclaimer: this is a structured read of indicators, not financial advice.") + "\n")

	return sb.String()
}

// ─── Render helpers ───────────────────────────────────────────────────────────

// priceArrow returns a coloured ▲/▼ if the tick is fresh (< 3s old).
func priceArrow(t liveTick) string {
	if t.prevPrice == 0 || time.Since(t.updatedAt) >= 3*time.Second {
		return ""
	}
	if t.price > t.prevPrice {
		return styleGreen.Render("▲")
	}
	if t.price < t.prevPrice {
		return styleRed.Render("▼")
	}
	return ""
}

// commaFloat formats a float with comma thousands separator.
func commaFloat(f float64, decimals int) string {
	if math.IsNaN(f) || math.IsInf(f, 0) || f == 0 {
		return "0"
	}
	neg := f < 0
	if neg {
		f = -f
	}
	intPart := int64(f)
	fracPart := f - float64(intPart)

	s := strconv.FormatInt(intPart, 10)
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	result := string(out)
	if decimals > 0 {
		frac := fmt.Sprintf("%.*f", decimals, fracPart)
		result += frac[1:]
	}
	if neg {
		result = "-" + result
	}
	return result
}

func priceDecimals(p float64) int {
	switch {
	case p >= 1000:
		return 2
	case p >= 1:
		return 4
	case p >= 0.001:
		return 6
	default:
		return 8
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func renderScoreBadge(score float64) string {
	switch {
	case score >= scanner.HotThreshold:
		return styleHot.Render("HOT")
	case score >= scanner.WarmThreshold:
		return styleWarm.Render("WARM")
	default:
		return styleEarly.Render("EARLY")
	}
}

func renderRegimeBadge(regime string) string {
	switch regime {
	case "BULL":
		return "● " + styleBull.Render("BULL")
	case "BEAR":
		return "● " + styleBear.Render("BEAR")
	default:
		return "● " + styleSide.Render("SIDEWAYS")
	}
}

func renderBar(val, maxVal float64, width int) string {
	filled := int(val / maxVal * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	col := colGreen
	if val < 40 {
		col = colRed
	} else if val < 70 {
		col = colYellow
	}
	return lipgloss.NewStyle().Foreground(col).Render("[" + bar + "]")
}

func divStr(d bool) string {
	if d {
		return " ✓div"
	}
	return ""
}

func fundingStr(s scanner.Signals) string {
	if !s.FundingAvail {
		return "N/A"
	}
	return fmt.Sprintf("%+.5f", s.FundingRate)
}

func oiStr(s scanner.Signals) string {
	if !s.OIAvail {
		return "N/A (spot only)"
	}
	return "available"
}

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "windows":
		cmd = "start"
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	exec.Command(cmd, url).Start() //nolint:errcheck
}

// translateWindow maps Indonesian window strings (legacy) to English.
func translateWindow(w string) string {
	switch w {
	case "1-3 hari":
		return "1-3 days"
	case "3-7 hari":
		return "3-7 days"
	case "7-14 hari":
		return "7-14 days"
	case "14-30 hari (speculative)":
		return "14-30 days (speculative)"
	}
	return w // already English or unknown — pass through
}

// recomputePlan recalculates entry, TP and SL levels relative to a new live
// price, keeping the same ATR and S/R structure from the original scan plan.
// This keeps deep-view levels honest when price moves between scans.
func recomputePlan(orig scanner.TradePlan, livePrice float64, sr scanner.SRLevels) scanner.TradePlan {
	p := orig
	p.Price = livePrice

	// Reanchor entry to live price (or nearest support above it)
	support := sr.NextS
	if support == 0 || support >= livePrice {
		support = livePrice * 0.97
	}
	resistance := sr.NextR
	if resistance == 0 || resistance <= livePrice {
		resistance = livePrice * 1.05
	}

	p.Entry = livePrice // enter at market
	p.SL = support - orig.ATR*0.5
	p.TP1 = p.Entry + orig.ATR*1.5
	p.TP2 = p.Entry + orig.ATR*3.0
	p.TP3 = resistance * 0.98

	pct := func(t float64) float64 {
		if livePrice == 0 {
			return 0
		}
		return (t/livePrice - 1) * 100
	}
	p.SLPct = pct(p.SL)
	p.TP1Pct = pct(p.TP1)
	p.TP2Pct = pct(p.TP2)
	p.TP3Pct = pct(p.TP3)

	risk := p.Entry - p.SL
	if risk > 0 {
		p.RR = (p.TP1 - p.Entry) / risk
	}
	return p
}

// formatAge returns a human-readable duration since t, compact format.
// e.g. "2m30s", "1h15m", "3h", "2d4h"
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		if h == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, h)
	}
}

// renderBullBar renders a coloured dot-bar showing current bull streak vs peak.
// Uses ASCII '*' and 'o' to avoid multi-byte rune width issues in terminal.
func renderBullBar(current, peak int) string {
	if peak <= 0 {
		return ""
	}
	cap := peak
	if cap > 10 {
		cap = 10
	}
	cur := current
	if cur > cap {
		cur = cap
	}
	filled := styleGreen.Render(strings.Repeat("*", cur))
	empty := styleDim.Render(strings.Repeat("o", cap-cur))
	return filled + empty
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}