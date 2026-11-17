package main

import (
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"
)

func started(t *testing.T, w, h int, s float64) *App {
	setup(t)
	a := newApp(w, h, s)
	a.rnd = rand.New(rand.NewSource(11))
	a.click("start")
	step(a, 2.5)
	return a
}

// waitDeal steps until a background deal lands.
func waitDeal(t *testing.T, a *App) {
	for i := 0; a.searching && i < 2000; i++ {
		time.Sleep(5 * time.Millisecond)
		a.update(1.0 / 60)
	}
	if a.searching {
		t.Fatal("deal never arrived")
	}
	step(a, 2.5)
}

func TestUndoRedoRestart(t *testing.T) {
	a := started(t, 1280, 800, 1)
	g := a.g
	deck := append([]int(nil), g.Deck...)
	before := g.board.clone()
	g.clickStock()
	g.clickStock()
	after := g.board.clone()
	a.undo()
	a.undo()
	if !reflect.DeepEqual(a.g.board, before) {
		t.Fatal("undo didn't restore the board")
	}
	a.redo()
	a.redo()
	if !reflect.DeepEqual(a.g.board, after) {
		t.Fatal("redo didn't replay")
	}
	a.click("restart")
	if !reflect.DeepEqual(a.g.Deck, deck) || len(a.g.Stock) != 24 || a.g.Moves != 0 {
		t.Fatal("restart isn't the same deal")
	}
}

func TestVegas(t *testing.T) {
	setup(t)
	g := dealGame(1, rand.New(rand.NewSource(5)).Perm(52), true)
	if g.Score != -52 {
		t.Fatal("buy-in")
	}
	for len(g.Stock) > 0 {
		g.clickStock()
	}
	if g.canRecycle() || g.clickStock() {
		t.Fatal("Draw One Vegas allows only one pass")
	}
	g3 := dealGame(3, rand.New(rand.NewSource(5)).Perm(52), true)
	passes := 0
	for g3.clickStock() {
		if len(g3.Stock) == 0 && len(g3.Waste) > 0 {
			passes++
		}
		if passes > 10 {
			break
		}
	}
	if g3.Recycles != 2 {
		t.Fatalf("Draw Three Vegas: %d recycles, want 2", g3.Recycles)
	}
}

func TestWinnableAndDaily(t *testing.T) {
	a := started(t, 1280, 800, 1)
	a.sv.Winnable = true
	a.click("new")
	waitDeal(t, a)
	if !winnable(a.g.Deck, a.g.Draw, solveLimit) {
		t.Fatal("winnable deal isn't")
	}
	a.click("daily")
	waitDeal(t, a)
	if a.g.Daily != today() || a.g.Draw != 1 {
		t.Fatal("not today's daily")
	}
	if !reflect.DeepEqual(a.g.Deck, dailyDeck(today())) {
		t.Fatal("daily deal isn't deterministic")
	}
}

func TestHintsAndCollect(t *testing.T) {
	a := started(t, 1280, 800, 1)
	if len(a.g.hints()) == 0 {
		t.Fatal("no hints on a fresh deal")
	}
	a.hint()
	if !a.hintOn.on || !a.g.UsedHint {
		t.Fatal("hint not shown")
	}
	// collect: put an ace on top of a column
	g := a.g
	g.Tab[0] = []card{{ID: 0, Up: true}}
	if !g.canCollect() {
		t.Fatal("ace should collect")
	}
	a.collect()
	step(a, 1)
	if len(g.Found[0])+len(g.Found[1])+len(g.Found[2])+len(g.Found[3]) == 0 {
		t.Fatal("collect moved nothing")
	}
}

func TestResume(t *testing.T) {
	a := started(t, 1280, 800, 1)
	a.g.clickStock()
	a.afterMove()
	a.g.Elapsed = 42
	want := a.g.board.clone()
	a.quit()
	b := newApp(1280, 800, 1)
	if b.resumable == nil {
		t.Fatal("nothing to resume")
	}
	b.click("continue")
	step(b, 2)
	if !reflect.DeepEqual(b.g.board, want) || int(b.g.Elapsed) < 42 || !b.g.canUndo() {
		t.Fatal("resumed game differs")
	}
	b.newGame()
	if _, err := os.Stat(resumePath()); err == nil {
		t.Fatal("resume file kept after a new deal")
	}
}

func TestStatsAndStamps(t *testing.T) {
	a := started(t, 1280, 800, 1)
	a.g.clickStock()
	a.afterMove()
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
	g.Elapsed = 100
	a.afterMove()
	if a.sv.Draw1.Won != 1 || a.sv.Draw1.Played != 1 || a.sv.Streak != 1 || a.sv.Draw1.BestSecs != 100 {
		t.Fatalf("stats: %+v streak %d", a.sv.Draw1, a.sv.Streak)
	}
	for _, id := range []string{"first", "quick", "lightning", "noundo", "nohint"} {
		if _, ok := a.sv.Stamps[id]; !ok {
			t.Errorf("stamp %s missing", id)
		}
	}
	// an abandoned played game breaks the streak
	step(a, 4)
	a.newGame()
	a.g.clickStock()
	a.afterMove()
	a.newGame()
	if a.sv.Streak != 0 || a.sv.Draw1.Played != 2 {
		t.Fatalf("loss not counted: streak %d played %d", a.sv.Streak, a.sv.Draw1.Played)
	}
}

func TestKeyboard(t *testing.T) {
	a := started(t, 1280, 800, 1)
	a.key(keyRight) // shows the cursor
	if !a.kb.on {
		t.Fatal("cursor not shown")
	}
	n := len(a.g.Waste)
	a.key(keySpace)
	if len(a.g.Waste) != n+1 {
		t.Fatal("space didn't draw")
	}
	// walk to every pile without panicking
	for i := 0; i < 20; i++ {
		a.key([]int{keyLeft, keyRight, keyUp, keyDown}[i%4])
		a.render()
	}
}

func TestFeatureScreens(t *testing.T) {
	a := started(t, 1440, 900, 1)
	a.hint()
	step(a, .5)
	writePNG(t, a.render(), "f1-hint.png")
	a.kbMove(0, 0)
	a.kbMove(1, 0)
	a.kbMove(1, 0)
	writePNG(t, a.render(), "f2-cursor.png")
	a.kb.on = false
	a.click("options")
	writePNG(t, a.render(), "f3-options.png")
	a.sv.Won = 30
	a.click("opt:four")
	a.click("opt:left")
	a.click("opt:back2")
	a.click("opt:table2")
	writePNG(t, a.render(), "f4-options-changed.png")
	a.click("close")
	step(a, 1.5)
	writePNG(t, a.render(), "f5-left-four-felt.png")
	a.click("opt:large")
	a.click("opt:table3")
	a.click("opt:back1")
	a.click("opt:right")
	step(a, 1.5)
	writePNG(t, a.render(), "f6-large-blue.png")
	a.click("opt:table1")
	a.click("opt:back3")
	a.click("opt:normal")
	a.click("opt:two")
	step(a, 1.5)
	writePNG(t, a.render(), "f7-walnut-sable.png")
	a.sv.Draw1 = modeStats{Played: 40, Won: 17, BestSecs: 141, WinSecs: 17 * 260, BestScore: 5120, Streak: 2, BestStreak: 5}
	a.sv.Draw3 = modeStats{Played: 12, Won: 3, BestSecs: 233, WinSecs: 3 * 320, BestScore: 4300, BestStreak: 1}
	a.sv.Played, a.sv.Won, a.sv.Streak, a.sv.BestStreak = 52, 20, 2, 5
	a.sv.Stamps = map[string]string{"first": "2026-11-01", "quick": "2026-11-03", "noundo": "2026-11-09", "three": "2026-11-12", "daily": "2026-11-15"}
	for i := 1; i <= 9; i += 2 {
		a.sv.Daily[time.Now().AddDate(0, 0, -i).Format("2006-01-02")] = dailyResult{Won: true, Score: 3000, Secs: 200, Moves: 90}
	}
	a.sv.Daily[time.Now().AddDate(0, 0, -2).Format("2006-01-02")] = dailyResult{Won: true}
	a.click("scores")
	for i := 1; i < 4; i++ {
		a.click("rtab" + itoa(i))
		writePNG(t, a.render(), "f8-records-"+itoa(i)+".png")
	}
	a.click("close")
	a.menu = "game"
	writePNG(t, a.render(), "f9-gamemenu.png")
	a.menu = ""
	// touch-sized controls, on a small screen
	b := started(t, 1100, 700, 1.25)
	b.sv.Controls = 1
	b.refreshLook()
	b.render()
	writePNG(t, b.render(), "f10-touch.png")
	b.click("options")
	writePNG(t, b.render(), "f11-touch-options.png")
	// Vegas
	c := started(t, 1280, 800, 1)
	c.sv.Vegas = true
	c.newGame()
	step(c, 3)
	writePNG(t, c.render(), "f12-vegas.png")
	// splash with a game to resume
	c.g.clickStock()
	c.afterMove()
	c.quit()
	d := newApp(1280, 800, 1)
	step(d, .3)
	writePNG(t, d.render(), "f13-splash-resume.png")
}
