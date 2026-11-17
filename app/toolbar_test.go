package main

import (
	"fmt"
	"math/rand"
	"testing"
)

// The toolbar must never overlap itself or run off the window.
func TestToolbarFits(t *testing.T) {
	setup(t)
	sizes := [][3]float64{{1920, 1080, 1}, {1440, 870, 1}, {1440, 900, 1.5}, {1280, 800, 1.25}, {1100, 760, 1}, {1366, 768, 1.5}, {990, 700, 1.25}, {825, 625, 1}, {660, 500, 1}, {990, 750, 1.5}}
	for _, sz := range sizes {
		a := newApp(int(sz[0]), int(sz[1]), sz[2])
		a.rnd = rand.New(rand.NewSource(3))
		a.click("start")
		step(a, 2)
		a.g.Score, a.g.Elapsed = 12345, 754
		a.sv.Won, a.sv.Played = 12, 104
		a.render()
		right := 0.0
		for _, h := range a.hits {
			if h.id == "vol" || len(h.id) > 0 && (h.id == "new" || h.id == "undo" || h.id == "draw" || h.id == "music" || h.id == "sfx" || h.id == "scores" || h.id == "about") {
				if h.r.x < 0 {
					t.Errorf("%v: %s starts off-window", sz, h.id)
				}
				right = max(right, h.r.x+h.r.w)
			}
		}
		limit := float64(a.W)
		if !a.tb.twoRows {
			limit = float64(a.W) - 14*a.s - a.tbStatsW(a.tb.k)
		}
		if right > limit+.5 {
			t.Errorf("%v: buttons end at %.0f, past %.0f (layout %+v)", sz, right, limit, a.tb)
		}
		if a.cw < 40 {
			t.Errorf("%v: cards too small (%v)", sz, a.cw)
		}
		writePNG(t, a.canvas, fmt.Sprintf("tb-%.0fx%.0f@%.2f.png", sz[0], sz[1], sz[2]))
	}
}
