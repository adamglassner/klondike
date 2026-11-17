// A Klondike solver, used to deal only winnable games. It plays with full
// knowledge of the deck (it can see face-down cards), searching depth-first
// with safe moves to the foundations played automatically. It gives up after
// a fixed amount of work, so a deal it can't crack in time is passed over.
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"bytes"
	"math/rand"
	"sort"
)

type sState struct {
	tab   [7][]int8 // each column, face-down cards first
	down  [7]int8   // how many of them are face down
	found [4]int8   // cards on each suit's foundation
	tal   []int8    // stock and waste, in the order they'd be dealt to the waste
	w     int       // cards dealt to the waste so far (top is tal[w-1])
}

func (s *sState) clone() *sState {
	c := *s
	for i := range s.tab {
		c.tab[i] = append([]int8(nil), s.tab[i]...)
	}
	c.tal = append([]int8(nil), s.tal...)
	return &c
}

func (s *sState) key(buf *bytes.Buffer) string {
	buf.Reset()
	var cols [7][]byte
	for i := range s.tab {
		b := make([]byte, 0, len(s.tab[i])+1)
		b = append(b, byte(s.down[i]))
		for _, c := range s.tab[i] {
			b = append(b, byte(c))
		}
		cols[i] = b
	}
	sort.Slice(cols[:], func(i, j int) bool { return bytes.Compare(cols[i], cols[j]) < 0 })
	for _, c := range cols {
		buf.Write(c)
		buf.WriteByte(0xFF)
	}
	for _, f := range s.found {
		buf.WriteByte(byte(f))
	}
	buf.WriteByte(byte(s.w))
	for _, c := range s.tal {
		buf.WriteByte(byte(c))
	}
	return buf.String()
}

func (s *sState) fits(c int8) bool { return s.found[suitOf(int(c))] == int8(rankOf(int(c))-1) }

func (s *sState) onTab(c int8, j int) bool {
	col := s.tab[j]
	if len(col) == 0 {
		return rankOf(int(c)) == 13
	}
	t := int(col[len(col)-1])
	return int(s.down[j]) < len(col) && isRed(t) != isRed(int(c)) && rankOf(int(c)) == rankOf(t)-1
}

// safe: nothing on the table could ever need this card to build on.
func (s *sState) safe(c int8) bool {
	r := rankOf(int(c))
	if r <= 2 {
		return true
	}
	o1, o2 := 0, 3
	if !isRed(int(c)) {
		o1, o2 = 1, 2
	}
	return int(s.found[o1]) >= r-1 && int(s.found[o2]) >= r-1
}

func (s *sState) fixColumn(i int) {
	if n := int8(len(s.tab[i])); n > 0 && s.down[i] >= n {
		s.down[i] = n - 1
	}
}

func (s *sState) takeTalon(p int) int8 {
	c := s.tal[p-1]
	s.tal = append(s.tal[:p-1:p-1], s.tal[p:]...)
	s.w = p - 1
	return c
}

// sMove records a solver move so a solution can be replayed (in tests).
type sMove struct {
	kind int // 0 column to foundation, 1 waste to foundation, 2 column to column, 3 waste to column
	i, j int // columns
	idx  int // index in the source column
	card int8
}

// autoplay plays safe foundation moves until there are none.
func (s *sState) autoplay(log *[]sMove) {
	for moved := true; moved; {
		moved = false
		for i := range s.tab {
			col := s.tab[i]
			if len(col) > int(s.down[i]) {
				c := col[len(col)-1]
				if s.fits(c) && s.safe(c) {
					if log != nil {
						*log = append(*log, sMove{kind: 0, i: i, card: c})
					}
					s.found[suitOf(int(c))]++
					s.tab[i] = col[:len(col)-1]
					s.fixColumn(i)
					moved = true
				}
			}
		}
		if s.w > 0 {
			c := s.tal[s.w-1]
			if s.fits(c) && s.safe(c) {
				if log != nil {
					*log = append(*log, sMove{kind: 1, card: c})
				}
				s.takeTalon(s.w)
				s.found[suitOf(int(c))]++
				moved = true
			}
		}
	}
}

func (s *sState) won() bool { return s.found[0]+s.found[1]+s.found[2]+s.found[3] == 52 }

// talonTops lists the waste positions that can be brought to the top by
// drawing (and turning the waste over, as often as needed).
func talonTops(n, w, draw int) []int {
	if n == 0 {
		return nil
	}
	seen := make([]bool, n+1)
	var out []int
	add := func(p int) {
		if p > 0 && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	add(w)
	for p := w + draw; ; p += draw {
		if p >= n {
			add(n)
			break
		}
		add(p)
	}
	for p := draw; p < n; p += draw {
		add(p)
	}
	add(n)
	return out
}

type sChild struct {
	s *sState
	m sMove
}

type solver struct {
	record bool
	path   []sMove // the winning line, last move first (when recording)
	draw   int
	seen   map[string]struct{}
	nodes  int
	limit  int
	buf    bytes.Buffer
}

func (v *solver) children(s *sState) []sChild {
	var first, mid, last []sChild
	tops := talonTops(len(s.tal), s.w, v.draw)
	// to the foundations
	for i := range s.tab {
		col := s.tab[i]
		if len(col) > int(s.down[i]) && s.fits(col[len(col)-1]) {
			c := s.clone()
			c.found[suitOf(int(col[len(col)-1]))]++
			c.tab[i] = c.tab[i][:len(col)-1]
			c.fixColumn(i)
			first = append(first, sChild{c, sMove{kind: 0, i: i, card: col[len(col)-1]}})
		}
	}
	for _, p := range tops {
		if s.fits(s.tal[p-1]) {
			c := s.clone()
			card := c.takeTalon(p)
			c.found[suitOf(int(card))]++
			first = append(first, sChild{c, sMove{kind: 1, card: card}})
		}
	}
	// column to column
	for i := range s.tab {
		col := s.tab[i]
		u := int(s.down[i])
		for idx := u; idx < len(col); idx++ {
			partial := idx > u
			if partial && !s.fits(col[idx-1]) {
				continue
			}
			if !partial && u == 0 && len(col) > 0 && rankOf(int(col[0])) == 13 {
				continue // a King already at the bottom
			}
			for j := range s.tab {
				if j == i || !s.onTab(col[idx], j) {
					continue
				}
				c := s.clone()
				c.tab[j] = append(c.tab[j], col[idx:]...)
				c.tab[i] = c.tab[i][:idx]
				c.fixColumn(i)
				m := sChild{c, sMove{kind: 2, i: i, j: j, idx: idx, card: col[idx]}}
				if !partial && u > 0 {
					first = append(first, m)
				} else {
					last = append(last, m)
				}
				if len(s.tab[j]) == 0 {
					break // one empty column is as good as another
				}
			}
		}
	}
	// waste to a column
	for _, p := range tops {
		card := s.tal[p-1]
		for j := range s.tab {
			if s.onTab(card, j) {
				c := s.clone()
				c.takeTalon(p)
				c.tab[j] = append(c.tab[j], card)
				mid = append(mid, sChild{c, sMove{kind: 3, j: j, card: card}})
				if len(s.tab[j]) == 0 {
					break
				}
			}
		}
	}
	return append(append(first, mid...), last...)
}

// search reports 1 for solved, 0 for no solution here, -1 when out of time.
func (v *solver) search(s *sState) int {
	var auto []sMove
	var log *[]sMove
	if v.record {
		log = &auto
	}
	s.autoplay(log)
	won := func() int {
		for i := len(auto) - 1; i >= 0; i-- {
			v.path = append(v.path, auto[i])
		}
		return 1
	}
	if s.won() {
		return won()
	}
	k := s.key(&v.buf)
	if _, ok := v.seen[k]; ok {
		return 0
	}
	v.seen[k] = struct{}{}
	v.nodes++
	if v.nodes > v.limit {
		return -1
	}
	for _, c := range v.children(s) {
		r := v.search(c.s)
		if r == 1 {
			v.path = append(v.path, c.m)
			return won()
		}
		if r != 0 {
			return r
		}
	}
	return 0
}

// winnable reports whether the solver finds a win for deck within limit
// positions (false if it runs out of time).
func winnable(deck []int, draw, limit int) bool {
	ok, _ := solve(deck, draw, limit, false)
	return ok
}

// solve is winnable that can also return the winning line, in play order.
func solve(deck []int, draw, limit int, record bool) (bool, []sMove) {
	s := &sState{}
	k := 0
	for row := 0; row < 7; row++ {
		for col := row; col < 7; col++ {
			s.tab[col] = append(s.tab[col], int8(deck[k]))
			k++
		}
	}
	for i := range s.tab {
		s.down[i] = int8(i)
	}
	for i := 51; i >= 28; i-- { // the stock is dealt from the end
		s.tal = append(s.tal, int8(deck[i]))
	}
	v := &solver{draw: draw, seen: map[string]struct{}{}, limit: limit, record: record}
	ok := v.search(s) == 1
	for i, j := 0, len(v.path)-1; i < j; i, j = i+1, j-1 {
		v.path[i], v.path[j] = v.path[j], v.path[i]
	}
	return ok, v.path
}

const solveLimit = 40000

// winnableDeck shuffles until the solver wins one (up to tries deals); it
// returns the last shuffle if none was proven.
func winnableDeck(rnd *rand.Rand, draw, tries int) ([]int, bool) {
	var deck []int
	for i := 0; i < tries; i++ {
		deck = rnd.Perm(52)
		if winnable(deck, draw, solveLimit) {
			return deck, true
		}
	}
	return deck, false
}
