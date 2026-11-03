// Klondike rules: a direct port of the original game's logic.
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
	Started bool    // timer running (first move made)
	Elapsed float64 // seconds
	Won     bool
	Auto    bool // auto-finish in progress
	undo    []board
	sound   func(name string, delay float64)
}

func newGame(draw int, rnd *rand.Rand) *game {
	g := &game{Draw: draw, sound: func(string, float64) {}}
	deck := rnd.Perm(52)
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
	if len(g.undo) > 600 {
		g.undo = g.undo[1:]
	}
}

func (g *game) canUndo() bool { return !g.Won && !g.Auto && len(g.undo) > 0 }

func (g *game) doUndo() bool {
	if !g.canUndo() {
		return false
	}
	g.board = g.undo[len(g.undo)-1]
	g.undo = g.undo[:len(g.undo)-1]
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
	if src.Kind == pWaste && dst.Kind == pTab {
		g.Score += 5
	}
	if dst.Kind == pFound && src.Kind != pFound {
		g.Score += 10
	}
	if src.Kind == pFound && dst.Kind == pTab {
		g.Score -= 15
	}
	if src.Kind == pTab {
		if t := &g.Tab[src.I]; len(*t) > 0 && !(*t)[len(*t)-1].Up {
			(*t)[len(*t)-1].Up = true
			g.Score += 5
			g.sound("flip", .09)
		}
	}
	if g.Score < 0 {
		g.Score = 0
	}
	g.Moves++
	g.Started = true
	g.sound("place", 0)
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
	} else if len(g.Waste) > 0 {
		g.pushUndo()
		for len(g.Waste) > 0 {
			c := g.Waste[len(g.Waste)-1]
			g.Waste = g.Waste[:len(g.Waste)-1]
			c.Up = false
			g.Stock = append(g.Stock, c)
		}
		if g.Draw == 1 {
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

// autoStep moves the lowest playable tableau card to a foundation.
func (g *game) autoStep() bool {
	bestI, bestF, bestR := -1, -1, 99
	for i, p := range g.Tab {
		if len(p) == 0 {
			continue
		}
		id := top(p).ID
		for f := 0; f < 4; f++ {
			if g.canFound(id, f) && rankOf(id) < bestR {
				bestI, bestF, bestR = i, f, rankOf(id)
			}
		}
	}
	if bestI < 0 {
		return false
	}
	g.move(pileRef{pTab, bestI}, len(g.Tab[bestI])-1, pileRef{pFound, bestF}, false)
	return true
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

// winBonus is the classic Windows time bonus.
func winBonus(secs int) int {
	if secs >= 30 {
		return 700000 / secs
	}
	return 0
}
