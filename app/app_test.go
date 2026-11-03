package main

import (
	"image"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, m *image.RGBA, name string) {
	c := image.NewRGBA(m.Rect)
	copy(c.Pix, m.Pix)
	swapRB(c)
	dir := os.Getenv("KL_SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, c)
}

func setup(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if A.table == nil {
		if err := loadArt(); err != nil {
			t.Fatal(err)
		}
	}
}

func step(a *App, secs float64) {
	for i := 0; i < int(secs*60); i++ {
		a.update(1.0 / 60)
	}
}

func TestScreens(t *testing.T) {
	setup(t)
	a := newApp(1440, 870, 1)
	a.rnd = rand.New(rand.NewSource(7))
	step(a, .5)
	writePNG(t, a.render(), "1-splash.png")
	a.render()
	a.click("start")
	step(a, 3)
	writePNG(t, a.render(), "2-dealt.png")
	// play: click stock a few times, and smart-move whatever we can
	for i := 0; i < 40; i++ {
		moved := false
		for _, tg := range a.targets() {
			if tg.c.Up && tg.pile.Kind != pFound && a.g.smartMove(tg.pile, tg.idx) {
				moved = true
				break
			}
		}
		if !moved {
			a.g.clickStock()
		}
		a.afterMove()
		step(a, .1)
	}
	step(a, 1)
	a.hover = "new"
	writePNG(t, a.render(), "3-play.png")
	// mid-drag
	for _, tg := range a.targets() {
		if tg.pile.Kind == pTab && tg.c.Up {
			a.render()
			x, y := tg.x+a.cw/2, tg.y+a.bar+10
			a.pointerDown(x, y)
			a.pointerMove(x+60, y+90)
			a.pointerMove(x+120, y+180)
			writePNG(t, a.render(), "4-drag.png")
			a.pointerUp(x+120, y+180)
			break
		}
	}
	a.click("about")
	writePNG(t, a.render(), "5-credits.png")
	a.click("tab1")
	writePNG(t, a.render(), "6-changelog.png")
	a.click("close")
	a.click("draw")
	writePNG(t, a.render(), "7-dropdown.png")
	a.ddOpen = false
	// force a win
	g := a.g
	for s := 0; s < 4; s++ {
		g.Found[s] = nil
		for r := 0; r < 13; r++ {
			g.Found[s] = append(g.Found[s], card{ID: s*13 + r, Up: true})
		}
	}
	g.Stock, g.Waste = nil, nil
	for i := range g.Tab {
		g.Tab[i] = nil
	}
	g.Elapsed = 187
	a.afterMove()
	step(a, 1.2)
	writePNG(t, a.render(), "8a-celebrate.png")
	step(a, 1.8)
	writePNG(t, a.render(), "8b-celebrate.png")
	step(a, 1)
	writePNG(t, a.render(), "9-win.png")
	for _, r := range "Dawson Dan" {
		a.char(r)
	}
	a.update(.01)
	writePNG(t, a.render(), "10-name.png")
	a.char('\r')
	writePNG(t, a.render(), "11-saved.png")
	if len(a.sv.Scores) != 1 || a.sv.Scores[0].Name != "Dawson Dan" || a.entry.rank != 1 {
		t.Fatalf("scores = %+v", a.sv.Scores)
	}
	// a few more for the board
	for i, n := range []string{"Klondike Kate", "Skookum Jim", "Soapy Smith", "Swiftwater Bill"} {
		a.sv.Scores, _ = addScore(a.sv.Scores, scoreEntry{Name: n, Score: 5200 - i*900, Secs: 140 + i*40, Moves: 90 + i*7, Draw: 1 + 2*(i%2), Date: "2026-11-0" + itoa(1+i%3)})
	}
	a.highlight = 3
	a.click("board")
	writePNG(t, a.render(), "12-board.png")
	a.click("close")
	if a.panel != panelWin {
		t.Fatal("back should return to the win screen")
	}
	// the save keeps the board
	b := loadSave()
	if len(b.Scores) != 1 || b.Nickname != "Dawson Dan" {
		t.Fatalf("saved = %+v", b)
	}
	a.highlight = 0
	a.sv.Scores = nil
	a.click("scores")
	writePNG(t, a.render(), "13-board-empty.png")
	if a.sv.Won != 1 {
		t.Fatalf("won = %d", a.sv.Won)
	}
}

func TestRules(t *testing.T) {
	g := newGame(1, rand.New(rand.NewSource(1)))
	n := len(g.Stock)
	for _, p := range g.Tab {
		n += len(p)
	}
	if n != 52 || len(g.Stock) != 24 {
		t.Fatal("bad deal")
	}
	// 5 of hearts (id 17) on black 6 (spades id 5)
	g.Tab[0] = []card{{5, true}}
	if !g.canTab(17, 0) || g.canTab(18+13, 0) {
		t.Fatal("canTab")
	}
	g.Tab[1] = nil
	if !g.canTab(12, 1) || g.canTab(11, 1) {
		t.Fatal("kings only on empty")
	}
	if !g.canFound(0, 0) || g.canFound(1, 0) {
		t.Fatal("aces first")
	}
	if winBonus(100) != 7000 || winBonus(20) != 0 {
		t.Fatal("bonus")
	}
}

func TestBoard(t *testing.T) {
	var b []scoreEntry
	for i := 0; i < 12; i++ {
		b, _ = addScore(b, scoreEntry{Name: "P" + itoa(i), Score: i * 100, Secs: 100})
	}
	if len(b) != boardSize || b[0].Score != 1100 || b[9].Score != 200 {
		t.Fatalf("board = %+v", b)
	}
	if qualifies(b, 150, 100) || !qualifies(b, 200, 90) || !qualifies(b, 300, 500) {
		t.Fatal("qualifies")
	}
	_, r := addScore(b, scoreEntry{Name: "X", Score: 1000, Secs: 50})
	if r != 2 {
		t.Fatalf("rank = %d", r)
	}
	if cleanName("  Gold   Rush  Gus the Great Prospector ") != "Gold Rush Gus th" {
		t.Fatalf("clean = %q", cleanName("  Gold   Rush  Gus the Great Prospector "))
	}
}
