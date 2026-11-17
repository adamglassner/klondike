package main

import (
	"math/rand"
	"testing"
	"time"
)

func TestSolverRates(t *testing.T) {
	for _, draw := range []int{1, 3} {
		rnd := rand.New(rand.NewSource(42))
		won, n := 0, 60
		t0 := time.Now()
		var worst time.Duration
		for i := 0; i < n; i++ {
			d := rnd.Perm(52)
			s := time.Now()
			if winnable(d, draw, solveLimit) {
				won++
			}
			worst = max(worst, time.Since(s))
		}
		t.Logf("draw %d: %d/%d proven winnable, avg %v, worst %v", draw, won, n, time.Since(t0)/time.Duration(n), worst)
		if won == 0 {
			t.Fatal("solver never wins")
		}
	}
}

// Every win the solver claims must replay move by move in the real game.
func TestSolverLinesReplay(t *testing.T) {
	for _, draw := range []int{1, 3} {
		rnd := rand.New(rand.NewSource(9))
		checked := 0
		for n := 0; n < 40; n++ {
			deck := rnd.Perm(52)
			ok, path := solve(deck, draw, solveLimit, true)
			if !ok {
				continue
			}
			checked++
			g := dealGame(draw, deck, false)
			for k, m := range path {
				fail := func(why string) { t.Fatalf("draw %d deal %d move %d %+v: %s", draw, n, k, m, why) }
				switch m.kind {
				case 1, 3: // bring the card to the top of the waste
					for tries := 0; len(g.Waste) == 0 || top(g.Waste).ID != int(m.card); tries++ {
						if tries > 60 || !g.clickStock() {
							fail("card never reached the waste")
						}
					}
					if m.kind == 1 {
						if !g.sendHome(pileRef{pWaste, 0}, len(g.Waste)-1) {
							fail("waste card can't go up")
						}
					} else {
						if !g.canTab(int(m.card), m.j) {
							fail("waste card can't go on column")
						}
						g.move(pileRef{pWaste, 0}, len(g.Waste)-1, pileRef{pTab, m.j}, true)
					}
				case 0:
					p := g.Tab[m.i]
					if len(p) == 0 || top(p).ID != int(m.card) || !g.sendHome(pileRef{pTab, m.i}, len(p)-1) {
						fail("column card can't go up")
					}
				case 2:
					p := g.Tab[m.i]
					if m.idx >= len(p) || p[m.idx].ID != int(m.card) || !p[m.idx].Up || !g.canTab(int(m.card), m.j) {
						fail("column move not legal")
					}
					g.move(pileRef{pTab, m.i}, m.idx, pileRef{pTab, m.j}, true)
				}
			}
			if !g.allFound() {
				t.Fatalf("draw %d deal %d: line ended without a win", draw, n)
			}
		}
		t.Logf("draw %d: replayed %d winning lines", draw, checked)
	}
}
