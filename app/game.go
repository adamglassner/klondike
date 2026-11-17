// Klondike rules: a direct port of the original game's logic, plus Vegas
// scoring, redo, hints and auto-collect.
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import "math/rand"

// Card ids 0..51: suit = id/13 (0 spades, 1 hearts, 2 diamonds, 3 clubs), rank = id%13+1.
func suitOf(id int) int { return id / 13 }
func rankOf(id int) int { return id%13 + 1 }
func isRed(id int) bool { s := suitOf(id); return s == 1 || s == 2 }

type card struct {
	ID int
	Up bool
}

// Pile kinds.
const (
	pStock = iota
	pWaste
	pFound
	pTab
)

type pileRef struct{ Kind, I int }

// board is everything undo needs to restore.
type board struct {
	Stock, Waste []card
	Found        [4][]card
	Tab          [7][]card
	Score, Moves int
	Recycles     int // times the waste has been turned back over
}

func (b *board) clone() board {
	c := *b
	cp := func(s []card) []card { return append([]card(nil), s...) }
	c.Stock, c.Waste = cp(b.Stock), cp(b.Waste)
	for i := range b.Found {
		c.Found[i] = cp(b.Found[i])
	}
	for i := range b.Tab {
		c.Tab[i] = cp(b.Tab[i])
	}
	return c
}

type game struct {
	board
	Draw    int
	Deck    []int   // the shuffled deck this deal came from (for Restart)
	Vegas   bool    // Vegas scoring: Score is dollars won or lost this deal
	Daily   string  // the Daily Claim's date, if this is one
	Started bool    // timer running (first move made)
	Elapsed float64 // seconds
	Won     bool
	Auto    bool // auto-finish or auto-collect in progress
	Collect bool // auto-collect (right click): any card that can go up
	Counted bool // counted as played (a move was made)
	Settled bool // Vegas winnings paid into the poke
	// for the assay stamps
	UsedUndo, UsedHint bool
	undo, redo         []board
	sound              func(name string, delay float64)
}

const vegasBuyIn = 52

func newGame(draw int, rnd *rand.Rand) *game { return dealGame(draw, rnd.Perm(52), false) }

// dealGame deals deck (a permutation of 0..51) the way the original did.
func dealGame(draw int, deck []int, vegas bool) *game {
	g := &game{Draw: draw, Deck: append([]int(nil), deck...), Vegas: vegas, sound: func(string, float64) {}}
	k := 0
	for row := 0; row < 7; row++ {
		for col := row; col < 7; col++ {
			g.Tab[col] = append(g.Tab[col], card{ID: deck[k], Up: row == col})
			k++
		}
	}
	for ; k < 52; k++ {
		g.Stock = append(g.Stock, card{ID: deck[k]})
	}
	if vegas {
		g.Score = -vegasBuyIn
	}
	return g
}

// dealOrder lists tableau card ids in the order they are dealt.
func (g *game) dealOrder() []int {
	var out []int
	for row := 0; row < 7; row++ {
		for col := row; col < 7; col++ {
			out = append(out, g.Tab[col][row].ID)
		}
	}
	return out
}

func (g *game) pile(p pileRef) *[]card {
	switch p.Kind {
	case pStock:
		return &g.Stock
	case pWaste:
		return &g.Waste
	case pFound:
		return &g.Found[p.I]
	default:
		return &g.Tab[p.I]
	}
}

func top(s []card) card { return s[len(s)-1] }

func (g *game) canFound(id, f int) bool {
	p := g.Found[f]
	if len(p) == 0 {
		return rankOf(id) == 1
	}
	t := top(p).ID
	return suitOf(t) == suitOf(id) && rankOf(id) == rankOf(t)+1
}

func (g *game) canTab(id, ti int) bool {
	p := g.Tab[ti]
	if len(p) == 0 {
		return rankOf(id) == 13
	}
	t := top(p)
	return t.Up && isRed(t.ID) != isRed(id) && rankOf(id) == rankOf(t.ID)-1
}

func (g *game) pushUndo() {
	g.undo = append(g.undo, g.board.clone())
	g.redo = nil
}

func (g *game) canUndo() bool { return !g.Won && !g.Auto && len(g.undo) > 0 }
func (g *game) canRedo() bool { return !g.Won && !g.Auto && len(g.redo) > 0 }

func (g *game) doUndo() bool {
	if !g.canUndo() {
		return false
	}
	g.redo = append(g.redo, g.board.clone())
	g.board = g.undo[len(g.undo)-1]
	g.undo = g.undo[:len(g.undo)-1]
	g.UsedUndo = true
	g.sound("place", 0)
	return true
}

func (g *game) doRedo() bool {
	if !g.canRedo() {
		return false
	}
	g.undo = append(g.undo, g.board.clone())
	g.board = g.redo[len(g.redo)-1]
	g.redo = g.redo[:len(g.redo)-1]
	g.sound("place", 0)
	return true
}

// move moves the cards from index idx of src onto pile dst.
func (g *game) move(src pileRef, idx int, dst pileRef, record bool) {
	if record {
		g.pushUndo()
	}
	from := g.pile(src)
	cards := append([]card(nil), (*from)[idx:]...)
	*from = (*from)[:idx]
	to := g.pile(dst)
	*to = append(*to, cards...)
	if g.Vegas {
		// $5 for every card sent up, and it's paid back if it comes down
		if dst.Kind == pFound && src.Kind != pFound {
			g.Score += 5
		}
		if src.Kind == pFound && dst.Kind != pFound {
			g.Score -= 5
		}
	} else {
		if src.Kind == pWaste && dst.Kind == pTab {
			g.Score += 5
		}
		if dst.Kind == pFound && src.Kind != pFound {
			g.Score += 10
		}
		if src.Kind == pFound && dst.Kind == pTab {
			g.Score -= 15
		}
	}
	if src.Kind == pTab {
		if t := &g.Tab[src.I]; len(*t) > 0 && !(*t)[len(*t)-1].Up {
			(*t)[len(*t)-1].Up = true
			if !g.Vegas {
				g.Score += 5
			}
			g.sound("flip", .09)
		}
	}
	if g.Score < 0 && !g.Vegas {
		g.Score = 0
	}
	g.Moves++
	g.Started = true
	g.sound("place", 0)
}

// maxRecycles is how often the waste may be turned over: unlimited normally;
// in Vegas, one pass through the deck at Draw One and three at Draw Three.
func (g *game) maxRecycles() int {
	if !g.Vegas {
		return -1
	}
	if g.Draw == 3 {
		return 2
	}
	return 0
}

func (g *game) canRecycle() bool {
	m := g.maxRecycles()
	return len(g.Waste) > 0 && (m < 0 || g.Recycles < m)
}

// clickStock draws from the stock, or turns the waste over when it's empty.
func (g *game) clickStock() bool {
	if len(g.Stock) > 0 {
		g.pushUndo()
		n := min(g.Draw, len(g.Stock))
		for i := 0; i < n; i++ {
			c := g.Stock[len(g.Stock)-1]
			g.Stock = g.Stock[:len(g.Stock)-1]
			c.Up = true
			g.Waste = append(g.Waste, c)
		}
		g.sound("flip", 0)
	} else if g.canRecycle() {
		g.pushUndo()
		for len(g.Waste) > 0 {
			c := g.Waste[len(g.Waste)-1]
			g.Waste = g.Waste[:len(g.Waste)-1]
			c.Up = false
			g.Stock = append(g.Stock, c)
		}
		g.Recycles++
		if g.Draw == 1 && !g.Vegas {
			g.Score = max(0, g.Score-100)
		}
		g.sound("riffle", 0)
	} else {
		return false
	}
	g.Moves++
	g.Started = true
	return true
}

func (g *game) allFound() bool {
	for _, f := range g.Found {
		if len(f) != 13 {
			return false
		}
	}
	return true
}

// readyToAutoFinish: everything is face up and the stock and waste are empty.
func (g *game) readyToAutoFinish() bool {
	if len(g.Stock) > 0 || len(g.Waste) > 0 {
		return false
	}
	for _, p := range g.Tab {
		for _, c := range p {
			if !c.Up {
				return false
			}
		}
	}
	return true
}

// autoStep moves the lowest playable card to a foundation: from the
// tableau, and also from the waste when collecting.
func (g *game) autoStep() bool {
	bestF, bestR := -1, 99
	var bestSrc pileRef
	bestIdx := 0
	try := func(src pileRef, p []card) {
		if len(p) == 0 || !top(p).Up {
			return
		}
		id := top(p).ID
		for f := 0; f < 4; f++ {
			if g.canFound(id, f) && rankOf(id) < bestR {
				bestSrc, bestIdx, bestF, bestR = src, len(p)-1, f, rankOf(id)
			}
		}
	}
	for i, p := range g.Tab {
		try(pileRef{pTab, i}, p)
	}
	if g.Collect {
		try(pileRef{pWaste, 0}, g.Waste)
	}
	if bestF < 0 {
		return false
	}
	g.move(bestSrc, bestIdx, pileRef{pFound, bestF}, false)
	return true
}

// canCollect reports whether a right-click collect would move anything.
func (g *game) canCollect() bool {
	h := &game{board: g.clone(), Collect: true, sound: func(string, float64) {}}
	return h.autoStep()
}

// sendHome moves the card at idx of src to a foundation, if it can go.
func (g *game) sendHome(src pileRef, idx int) bool {
	p := *g.pile(src)
	if src.Kind == pFound || idx != len(p)-1 || !p[idx].Up {
		return false
	}
	for f := 0; f < 4; f++ {
		if g.canFound(p[idx].ID, f) {
			g.move(src, idx, pileRef{pFound, f}, true)
			return true
		}
	}
	return false
}

// smartMove sends the clicked cards to the best spot: a foundation first,
// else a tableau column (non-empty ones before empty ones).
func (g *game) smartMove(src pileRef, idx int) bool {
	p := *g.pile(src)
	cards := p[idx:]
	id := cards[0].ID
	if len(cards) == 1 && src.Kind != pFound {
		for f := 0; f < 4; f++ {
			if g.canFound(id, f) {
				g.move(src, idx, pileRef{pFound, f}, true)
				return true
			}
		}
	}
	order := make([]int, 0, 7)
	for t := 0; t < 7; t++ {
		if len(g.Tab[t]) > 0 {
			order = append(order, t)
		}
	}
	for t := 0; t < 7; t++ {
		if len(g.Tab[t]) == 0 {
			order = append(order, t)
		}
	}
	for _, t := range order {
		if src.Kind == pTab && src.I == t {
			continue
		}
		if len(g.Tab[t]) == 0 && src.Kind == pTab && idx == 0 {
			continue
		}
		if g.canTab(id, t) {
			g.move(src, idx, pileRef{pTab, t}, true)
			return true
		}
	}
	return false
}

/* ------------------------------------------------------------------ hints */

// hintMove is a suggested move. A Kind of pStock in src means "draw".
type hintMove struct {
	src pileRef
	idx int
	dst pileRef
}

// hints lists useful moves, best first. Moves that just shuffle cards back
// and forth between columns aren't suggested.
func (g *game) hints() []hintMove {
	var out, later []hintMove
	// to the foundations
	for i, p := range g.Tab {
		if len(p) > 0 && top(p).Up {
			for f := 0; f < 4; f++ {
				if g.canFound(top(p).ID, f) {
					out = append(out, hintMove{pileRef{pTab, i}, len(p) - 1, pileRef{pFound, f}})
					break
				}
			}
		}
	}
	if n := len(g.Waste); n > 0 {
		for f := 0; f < 4; f++ {
			if g.canFound(g.Waste[n-1].ID, f) {
				out = append(out, hintMove{pileRef{pWaste, 0}, n - 1, pileRef{pFound, f}})
				break
			}
		}
	}
	// column to column: moving a whole face-up run to turn up a card or to
	// clear a column, or part of one to free a card for the foundations
	for i, p := range g.Tab {
		u := len(p)
		for k := range p {
			if p[k].Up {
				u = k
				break
			}
		}
		for idx := u; idx < len(p); idx++ {
			useful := false
			if idx == u {
				useful = u > 0 || len(p) > 0 // turns a card up, or empties the column
			} else {
				below := p[idx-1].ID
				for f := 0; f < 4; f++ {
					if g.canFound(below, f) {
						useful = true
					}
				}
			}
			if !useful {
				continue
			}
			for j := 0; j < 7; j++ {
				if j == i || !g.canTab(p[idx].ID, j) {
					continue
				}
				if len(g.Tab[j]) == 0 && idx == 0 {
					continue // a King from one empty spot to another
				}
				m := hintMove{pileRef{pTab, i}, idx, pileRef{pTab, j}}
				if idx == u && u > 0 {
					out = append(out, m)
				} else {
					later = append(later, m)
				}
				break
			}
		}
	}
	// waste to a column
	if n := len(g.Waste); n > 0 {
		for j := 0; j < 7; j++ {
			if g.canTab(g.Waste[n-1].ID, j) {
				out = append(out, hintMove{pileRef{pWaste, 0}, n - 1, pileRef{pTab, j}})
				break
			}
		}
	}
	out = append(out, later...)
	if len(g.Stock) > 0 || g.canRecycle() {
		out = append(out, hintMove{src: pileRef{pStock, 0}})
	}
	return out
}

// winBonus is the classic Windows time bonus.
func winBonus(secs int) int {
	if secs >= 30 {
		return 700000 / secs
	}
	return 0
}
