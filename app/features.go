// Deals, records and helpers: new / restart / daily deals (optionally only
// winnable ones), statistics and streaks, assay stamps, the Daily Claim,
// Vegas scoring, save-and-resume, hints, auto-collect and keyboard play.
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

/* ------------------------------------------------------------------ deals */

type dealKind int

const (
	dealNew dealKind = iota
	dealRestart
	dealDaily
)

type dealReady struct {
	deck   []int
	daily  string
	draw   int
	vegas  bool
	proven bool
}

// deal starts a new deal. Winnable deals and the Daily Claim are searched
// for in the background; the table keeps its cards until one is found.
func (a *App) deal(kind dealKind) {
	if a.searching {
		return
	}
	a.endGame()
	switch kind {
	case dealRestart:
		g := a.g
		a.applyDeal(dealReady{deck: g.Deck, daily: g.Daily, draw: g.Draw, vegas: g.Vegas})
	case dealDaily:
		day := today()
		a.searching = true
		a.toast("Fetching today's claim…")
		go func() {
			a.dealCh <- dealReady{deck: dailyDeck(day), daily: day, draw: 1}
		}()
	default:
		draw, vegas := a.sv.Draw, a.sv.Vegas
		if !a.sv.Winnable {
			a.applyDeal(dealReady{deck: a.rnd.Perm(52), draw: draw, vegas: vegas})
			return
		}
		a.searching = true
		seed := a.rnd.Int63()
		go func() {
			deck, ok := winnableDeck(rand.New(rand.NewSource(seed)), draw, 40)
			a.dealCh <- dealReady{deck: deck, draw: draw, vegas: vegas, proven: ok}
		}()
		a.after(.35, func() {
			if a.searching {
				a.toast("Finding a winnable deal…")
			}
		})
	}
}

func today() string { return time.Now().Format("2006-01-02") }

// dailyDeck is the same for everyone on a given day: the first winnable
// shuffle from that day's seed.
func dailyDeck(day string) []int {
	t, _ := time.Parse("2006-01-02", day)
	base := int64(t.Year()*10000+int(t.Month())*100+t.Day()) * 1000003
	for k := int64(0); k < 400; k++ {
		deck := rand.New(rand.NewSource(base + k)).Perm(52)
		if winnable(deck, 1, solveLimit) {
			return deck
		}
	}
	return rand.New(rand.NewSource(base)).Perm(52)
}

// applyDeal puts a found deal on the table, dealing it out.
func (a *App) applyDeal(d dealReady) {
	a.searching = false
	a.cel = nil
	if a.panel == panelWin {
		a.panel = panelNone
	}
	a.timers = nil
	a.g = dealGame(d.draw, d.deck, d.vegas)
	a.g.Daily = d.daily
	a.bindSound(a.g)
	a.entry = winEntry{}
	a.clearHint()
	a.kb.held = false
	for i := range a.disp {
		a.disp[i] = dispCard{x: a.stockX(), y: a.top0, f0: -1}
	}
	for i, id := range a.g.dealOrder() {
		a.disp[id].wait = a.now + .25 + float64(i)*.055
	}
	a.drag = nil
	a.toastAt = -10
	if d.daily != "" {
		a.toast("The Daily Claim for " + prettyShortDate(d.daily) + " — Draw One.")
	}
	removeResume()
	mix.play("shuffle", 0)
	a.dirty = true
}

// endGame settles the game on the table before another is dealt: an
// unfinished game that was played counts as a loss, and Vegas money is paid.
func (a *App) endGame() {
	g := a.g
	if g == nil || !a.started {
		return
	}
	if g.Counted && !g.Won {
		a.sv.mode(g.Draw).Streak = 0
		a.sv.Streak = 0
	}
	a.settleVegas()
	a.sv.write()
}

func (a *App) settleVegas() {
	g := a.g
	if !g.Vegas || g.Settled || !g.Counted {
		return
	}
	g.Settled = true
	a.sv.Poke += g.Score
	if g.Score > 0 {
		a.stamp("vegas")
	}
}

// pokeNow is the Vegas balance including the deal on the table.
func (a *App) pokeNow() int {
	p := a.sv.Poke
	if g := a.g; g != nil && g.Vegas && !g.Settled && a.started {
		p += g.Score
	}
	return p
}

func dollars(n int) string {
	if n < 0 {
		return "-$" + commas(-n)
	}
	return "$" + commas(n)
}

// counted marks the game as played once its first move is made.
func (a *App) countGame() {
	g := a.g
	if g.Counted || !g.Started {
		return
	}
	g.Counted = true
	a.sv.Played++
	a.sv.mode(g.Draw).Played++
	a.sv.write()
}

/* ---------------------------------------------------------------- winning */

// recordWin updates statistics, streaks, the Daily Claim and stamps.
func (a *App) recordWin(secs int) {
	g := a.g
	a.countGame()
	m := a.sv.mode(g.Draw)
	m.Won++
	m.Streak++
	m.BestStreak = max(m.BestStreak, m.Streak)
	if m.BestSecs == 0 || secs < m.BestSecs {
		m.BestSecs = secs
	}
	m.WinSecs += secs
	if !g.Vegas {
		m.BestScore = max(m.BestScore, g.Score)
	}
	a.sv.Won++
	a.sv.Streak++
	a.sv.BestStreak = max(a.sv.BestStreak, a.sv.Streak)
	if g.Daily != "" {
		old, had := a.sv.Daily[g.Daily]
		r := dailyResult{Won: true, Score: g.Score, Secs: secs, Moves: g.Moves}
		if !had || !old.Won || r.Score > old.Score || (r.Score == old.Score && r.Secs < old.Secs) {
			a.sv.Daily[g.Daily] = r
		}
	}
	a.settleVegas()
	// stamps
	a.stamp("first")
	if a.sv.Won >= 10 {
		a.stamp("ten")
	}
	if a.sv.Won >= 50 {
		a.stamp("fifty")
	}
	if secs < 180 {
		a.stamp("quick")
	}
	if secs < 120 {
		a.stamp("lightning")
	}
	if g.Draw == 3 {
		a.stamp("three")
	}
	if !g.UsedUndo {
		a.stamp("noundo")
	}
	if !g.UsedHint {
		a.stamp("nohint")
	}
	if a.sv.Streak >= 3 {
		a.stamp("streak3")
	}
	if a.sv.Streak >= 5 {
		a.stamp("streak5")
	}
	if g.Daily != "" {
		a.stamp("daily")
		if cur, _ := a.dailyStreaks(); cur >= 7 {
			a.stamp("week")
		}
	}
	if !g.Vegas && g.Score >= 5000 {
		a.stamp("big")
	}
	// newly unlocked backs and tables
	for _, l := range cardBacks {
		if l.wins == a.sv.Won && l.wins > 0 {
			a.toastLater("New card back unlocked: " + l.name + " (Options)")
		}
	}
	for _, l := range tables {
		if l.wins == a.sv.Won && l.wins > 0 {
			a.toastLater("New table unlocked: " + l.name + " (Options)")
		}
	}
	removeResume()
	a.sv.write()
}

// dailyStreaks is the current and best run of consecutive Daily Claim wins.
// Today not yet played doesn't break the current run.
func (a *App) dailyStreaks() (cur, best int) {
	won := func(t time.Time) bool { return a.sv.Daily[t.Format("2006-01-02")].Won }
	t := time.Now()
	if !won(t) {
		t = t.AddDate(0, 0, -1)
	}
	for won(t) {
		cur++
		t = t.AddDate(0, 0, -1)
	}
	// best: walk every day that has a win
	run := 0
	var prev time.Time
	days := make([]string, 0, len(a.sv.Daily))
	for d, r := range a.sv.Daily {
		if r.Won {
			days = append(days, d)
		}
	}
	sortStrings(days)
	for _, d := range days {
		tt, err := time.Parse("2006-01-02", d)
		if err != nil {
			continue
		}
		if !prev.IsZero() && tt.Sub(prev) <= 25*time.Hour {
			run++
		} else {
			run = 1
		}
		best = max(best, run)
		prev = tt
	}
	return cur, max(best, cur)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

/* ----------------------------------------------------------------- stamps */

type stampDef struct{ id, name, desc string }

var stampDefs = []stampDef{
	{"first", "First Strike", "Win your first game."},
	{"ten", "Sourdough", "Win 10 games."},
	{"fifty", "Old-Timer", "Win 50 games."},
	{"quick", "Quick Draw", "Win in under 3 minutes."},
	{"lightning", "Lightning Claim", "Win in under 2 minutes."},
	{"three", "Three-Card Monte", "Win at Draw Three."},
	{"noundo", "No Going Back", "Win without using Undo."},
	{"nohint", "Self-Made", "Win without asking for a hint."},
	{"streak3", "Hot Streak", "Win 3 games in a row."},
	{"streak5", "Bonanza", "Win 5 games in a row."},
	{"daily", "Daily Prospector", "Win a Daily Claim."},
	{"week", "Week of Gold", "Win the Daily Claim 7 days running."},
	{"vegas", "High Roller", "Finish a Vegas deal in profit."},
	{"big", "Mother Lode", "Score 5,000 or more in one game."},
}

func (a *App) stamp(id string) {
	if _, ok := a.sv.Stamps[id]; ok {
		return
	}
	a.sv.Stamps[id] = today()
	for _, d := range stampDefs {
		if d.id == id {
			a.toastLater("Assay stamp earned: " + d.name)
		}
	}
}

/* ------------------------------------------------------------------ toast */

func (a *App) toast(msg string) {
	a.toastText, a.toastAt = msg, a.now
	a.dirty = true
}

// toastLater queues messages so several (stamps, unlocks) show in turn.
func (a *App) toastLater(msg string) {
	a.toastQueue = append(a.toastQueue, msg)
}

const toastLife = 3.2

func (a *App) toastStep() bool {
	if a.now-a.toastAt > toastLife && len(a.toastQueue) > 0 && a.panel != panelWin {
		a.toast(a.toastQueue[0])
		a.toastQueue = a.toastQueue[1:]
	}
	return a.now-a.toastAt < toastLife+.1
}

/* ------------------------------------------------------------------ hints */

type hintShow struct {
	on bool
	m  hintMove
	t0 float64
}

func (a *App) clearHint() { a.hintOn = hintShow{}; a.hintIdx = 0 }

func (a *App) hint() {
	g := a.g
	if !a.started || g == nil || g.Won || g.Auto || a.searching {
		return
	}
	hs := g.hints()
	if len(hs) == 0 {
		a.toast("No moves left on this claim. Try Restart, or a New Game.")
		return
	}
	g.UsedHint = true
	m := hs[a.hintIdx%len(hs)]
	a.hintIdx++
	a.hintOn = hintShow{on: true, m: m, t0: a.now}
	if m.src.Kind == pStock {
		if len(g.Stock) > 0 {
			a.toast("Draw from the stock.")
		} else {
			a.toast("Turn the waste back over and draw again.")
		}
	}
	a.dirty = true
}

const hintLife = 2.6

/* ----------------------------------------------------------- auto-collect */

// collect sends every card that can go up to the foundations (right click,
// or a long press on a touch screen).
func (a *App) collect() {
	g := a.g
	if !a.started || g == nil || g.Won || g.Auto || a.searching || !g.canCollect() {
		return
	}
	g.pushUndo()
	g.Auto, g.Collect = true, true
	a.nextAuto = a.now
	a.drag = nil
	a.dirty = true
}

/* ----------------------------------------------------------------- resume */

type resumeData struct {
	Board    board   `json:"board"`
	Draw     int     `json:"draw"`
	Deck     []int   `json:"deck"`
	Vegas    bool    `json:"vegas,omitempty"`
	Daily    string  `json:"daily,omitempty"`
	Elapsed  float64 `json:"elapsed"`
	Counted  bool    `json:"counted"`
	UsedUndo bool    `json:"usedUndo,omitempty"`
	UsedHint bool    `json:"usedHint,omitempty"`
	Undo     []board `json:"undo,omitempty"`
	Redo     []board `json:"redo,omitempty"`
}

func resumePath() string { return filepath.Join(dataDir(), "resume.json") }

func removeResume() { _ = os.Remove(resumePath()) }

// saveResume keeps the game on the table so the next launch picks it up.
func (a *App) saveResume() {
	g := a.g
	if g == nil || !a.started || !g.Started || g.Won || a.searching {
		removeResume()
		return
	}
	undo := g.undo
	if len(undo) > 500 {
		undo = undo[len(undo)-500:]
	}
	r := resumeData{Board: g.board, Draw: g.Draw, Deck: g.Deck, Vegas: g.Vegas, Daily: g.Daily, Elapsed: g.Elapsed,
		Counted: g.Counted, UsedUndo: g.UsedUndo, UsedHint: g.UsedHint, Undo: undo, Redo: g.redo}
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	_ = os.MkdirAll(dataDir(), 0o755)
	tmp := resumePath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, resumePath())
	}
}

func loadResume() *game {
	b, err := os.ReadFile(resumePath())
	if err != nil {
		return nil
	}
	var r resumeData
	if json.Unmarshal(b, &r) != nil || len(r.Deck) != 52 {
		return nil
	}
	n := len(r.Board.Stock) + len(r.Board.Waste)
	for _, p := range r.Board.Found {
		n += len(p)
	}
	for _, p := range r.Board.Tab {
		n += len(p)
	}
	if n != 52 {
		return nil
	}
	g := &game{board: r.Board, Draw: r.Draw, Deck: r.Deck, Vegas: r.Vegas, Daily: r.Daily, Started: true,
		Elapsed: r.Elapsed, Counted: r.Counted, UsedUndo: r.UsedUndo, UsedHint: r.UsedHint, undo: r.Undo, redo: r.Redo}
	if g.Draw != 3 {
		g.Draw = 1
	}
	return g
}

// resumeGame continues the game saved last time.
func (a *App) resumeGame() {
	g := a.resumable
	a.resumable = nil
	if g == nil {
		a.start()
		return
	}
	a.panel = panelNone
	a.started = true
	a.g = g
	a.bindSound(g)
	for _, t := range a.targets() {
		a.disp[t.c.ID] = dispCard{x: a.stockX(), y: a.top0, up: false, f0: -1}
	}
	for i, t := range a.targets() {
		a.disp[t.c.ID].wait = a.now + .15 + float64(i)*.012
	}
	mix.play("shuffle", 0)
	a.toast("Welcome back — your claim is just as you left it.")
	a.dirty = true
}

// quit is called when the window closes.
func (a *App) quit() {
	a.saveResume()
	a.sv.write()
}

/* --------------------------------------------------------------- keyboard */

// The keyboard cursor: arrows move between piles (and up and down a
// column), Enter picks cards up and puts them down, Space draws.
type kbState struct {
	on   bool
	at   pileRef
	idx  int // card index in a column
	held bool
	src  pileRef
	sidx int
}

func (a *App) kbSpots() [][]pileRef {
	top := []pileRef{{pStock, 0}, {pWaste, 0}, {pFound, 0}, {pFound, 1}, {pFound, 2}, {pFound, 3}}
	// order the top row by where the piles are drawn
	x := func(p pileRef) float64 {
		switch p.Kind {
		case pStock:
			return a.stockX()
		case pWaste:
			return a.wasteX()
		}
		return a.foundX(p.I)
	}
	for i := 1; i < len(top); i++ {
		for j := i; j > 0 && x(top[j]) < x(top[j-1]); j-- {
			top[j], top[j-1] = top[j-1], top[j]
		}
	}
	var tab []pileRef
	for i := 0; i < 7; i++ {
		tab = append(tab, pileRef{pTab, i})
	}
	return [][]pileRef{top, tab}
}

func (a *App) pileX(p pileRef) float64 {
	switch p.Kind {
	case pStock:
		return a.stockX()
	case pWaste:
		return a.wasteX()
	case pFound:
		return a.foundX(p.I)
	}
	return a.colX(p.I)
}

// firstUp is the index of the first face-up card in column i (len if none).
func (a *App) firstUp(i int) int {
	p := a.g.Tab[i]
	for k, c := range p {
		if c.Up {
			return k
		}
	}
	return len(p)
}

func (a *App) kbFixIdx() {
	k := &a.kb
	if k.at.Kind != pTab {
		return
	}
	n := len(a.g.Tab[k.at.I])
	u := a.firstUp(k.at.I)
	if k.idx >= n || k.idx < u {
		k.idx = max(0, n-1)
	}
}

func (a *App) kbMove(dx, dy int) {
	k := &a.kb
	if !k.on {
		k.on = true
		k.at, k.idx = pileRef{pTab, 0}, 0
		a.kbFixIdx()
		return
	}
	rows := a.kbSpots()
	row, col := 0, 0
	for r, ps := range rows {
		for c, p := range ps {
			if p == k.at {
				row, col = r, c
			}
		}
	}
	switch {
	case dx != 0:
		col = (col + dx + len(rows[row])) % len(rows[row])
		k.at = rows[row][col]
		k.idx = 1 << 20
	case dy < 0 && row == 1 && k.idx > a.firstUp(k.at.I) && k.idx < len(a.g.Tab[k.at.I]):
		k.idx-- // up the column, through the face-up cards
	case dy > 0 && row == 1 && k.idx < len(a.g.Tab[k.at.I])-1:
		k.idx++
	case dy != 0:
		// to the other row, the nearest pile across
		x := a.pileX(k.at)
		other := rows[1-row]
		best := other[0]
		for _, p := range other {
			if math.Abs(a.pileX(p)-x) < math.Abs(a.pileX(best)-x) {
				best = p
			}
		}
		k.at = best
		k.idx = 1 << 20
	}
	a.kbFixIdx()
}

// kbSelect is Enter: pick up, put down, or draw.
func (a *App) kbSelect() {
	g := a.g
	k := &a.kb
	if !k.on {
		a.kbMove(0, 0)
		return
	}
	if g.Won || g.Auto || a.searching {
		return
	}
	if !k.held {
		switch k.at.Kind {
		case pStock:
			if g.clickStock() {
				a.afterMove()
			}
			return
		case pWaste, pFound:
			if p := *g.pile(k.at); len(p) > 0 {
				k.held, k.src, k.sidx = true, k.at, len(p)-1
			}
		case pTab:
			p := g.Tab[k.at.I]
			if len(p) > 0 && k.idx < len(p) && p[k.idx].Up {
				k.held, k.src, k.sidx = true, k.at, k.idx
			}
		}
		a.dirty = true
		return
	}
	// putting down
	k.held = false
	src, idx := k.src, k.sidx
	p := *g.pile(src)
	if idx >= len(p) {
		return
	}
	if k.at == src { // pressed twice: send it where it fits best
		if g.smartMove(src, idx) {
			a.afterMove()
		}
		return
	}
	id := p[idx].ID
	switch k.at.Kind {
	case pFound:
		if idx == len(p)-1 && g.canFound(id, k.at.I) {
			g.move(src, idx, k.at, true)
			a.afterMove()
			return
		}
	case pTab:
		if g.canTab(id, k.at.I) {
			g.move(src, idx, k.at, true)
			a.afterMove()
			a.kbFixIdx()
			return
		}
	}
	a.toast("That card can't go there.")
}

func (a *App) kbDraw() {
	if g := a.g; a.started && g != nil && !g.Won && !g.Auto && !a.searching {
		if g.clickStock() {
			a.afterMove()
		}
	}
}
