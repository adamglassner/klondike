// The claim register: a local top-10 leaderboard, and the nickname box on
// the win screen. Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"image"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	boardSize   = 10
	maxNameLen  = 16
	defaultName = "Prospector"
)

type scoreEntry struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
	Secs  int    `json:"secs"`
	Moves int    `json:"moves"`
	Draw  int    `json:"draw"`
	Date  string `json:"date"` // YYYY-MM-DD
}

// sortScores orders the board: best score first, then the faster win, then
// the earlier one.
func sortScores(s []scoreEntry) {
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].Score != s[j].Score {
			return s[i].Score > s[j].Score
		}
		return s[i].Secs < s[j].Secs
	})
}

// qualifies reports whether a score would make the board.
func qualifies(board []scoreEntry, score, secs int) bool {
	if len(board) < boardSize {
		return true
	}
	last := board[len(board)-1]
	return score > last.Score || (score == last.Score && secs < last.Secs)
}

// addScore puts e on the board and returns its place (1-based), or 0.
func addScore(board []scoreEntry, e scoreEntry) ([]scoreEntry, int) {
	board = append(board, e)
	idx := len(board) - 1
	sortScores(board)
	for i := range board {
		if board[i] == e {
			idx = i
			break
		}
	}
	if len(board) > boardSize {
		board = board[:boardSize]
	}
	if idx >= boardSize {
		return board, 0
	}
	return board, idx + 1
}

func cleanName(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > maxNameLen {
		r = r[:maxNameLen]
	}
	return strings.TrimSpace(string(r))
}

/* ------------------------------------------------------------ app state */

type winEntry struct {
	open   bool // the nickname box is showing
	name   []rune
	result scoreEntry
	rank   int // place on the board once saved
	saved  bool
}

// prepareEntry is called when a game is won.
func (a *App) prepareEntry(score, secs, moves int) {
	e := scoreEntry{Score: score, Secs: secs, Moves: moves, Draw: a.g.Draw, Date: time.Now().Format("2006-01-02")}
	name := a.sv.Nickname
	if name == "" {
		name = defaultName
	}
	a.entry = winEntry{open: qualifies(a.sv.Scores, score, secs), name: []rune(name), result: e}
	a.entrySelAll = true
}

func (a *App) saveEntry() {
	w := &a.entry
	if !w.open || w.saved {
		return
	}
	name := cleanName(string(w.name))
	if name == "" {
		name = defaultName
	}
	w.result.Name = name
	a.sv.Nickname = name
	a.sv.Scores, w.rank = addScore(a.sv.Scores, w.result)
	w.saved = true
	a.sv.write()
	a.highlight = w.rank
	a.dirty = true
}

// typing reports whether keys go to the nickname box.
func (a *App) typing() bool { return a.panel == panelWin && a.entry.open && !a.entry.saved }

// char handles a typed character (WM_CHAR on Windows).
func (a *App) char(r rune) {
	if !a.typing() {
		return
	}
	w := &a.entry
	switch {
	case r == '\r' || r == '\n':
		a.saveEntry()
	case r == '\b':
		if a.entrySelAll {
			w.name = nil
		} else if len(w.name) > 0 {
			w.name = w.name[:len(w.name)-1]
		}
	case r >= 32 && r != 127 && unicode.IsPrint(r):
		if a.entrySelAll {
			w.name = nil
		}
		if len(w.name) < maxNameLen && !(r == ' ' && len(w.name) == 0) {
			w.name = append(w.name, r)
		}
	default:
		return
	}
	a.entrySelAll = false
	a.caretAt = a.now
	a.dirty = true
}

/* ------------------------------------------------------------- drawing */

// drawEntry draws the "sign the claim register" row on the win panel and
// returns its height.
func (a *App) drawEntry(cv *image.RGBA, cx, y float64) float64 {
	s := a.s
	w := &a.entry
	ink, note := hex("#3a2412"), hex("#7a5a30")
	if !w.open {
		top := ""
		if len(a.sv.Scores) > 0 {
			top = "  The top ten starts at " + commas(a.sv.Scores[len(a.sv.Scores)-1].Score) + "."
		}
		drawText(cv, "Not quite enough for the claim register this time."+top, fontSpec{Size: 14 * s}, note, cx, y+14*s, alignCenter)
		return 22 * s
	}
	if w.saved {
		msg := "Signed as " + w.result.Name + " — number " + itoa(w.rank) + " on the claim register."
		drawText(cv, msg, fontSpec{Size: 15 * s, Bold: true}, ink, cx, y+15*s, alignCenter)
		return 24 * s
	}
	label := "Sign the claim register:"
	lf := fontSpec{Size: 15 * s}
	lw := measure(label, lf)
	fw := 230 * s
	st := btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
	bw := a.buttonW("Save Score", st)
	fh := a.buttonH(st) + 4*s
	total := lw + 10*s + fw + 8*s + bw
	x := cx - total/2
	mid := y + fh/2
	drawText(cv, label, lf, ink, x, mid+15*s*.34, alignLeft)
	x += lw + 10*s
	// the text box
	a.addHit("name", x, y, fw, fh)
	fillRRect(cv, x, y, fw, fh, 4*s, uni(hex("#fffaf0")))
	strokeRRect(cv, x, y, fw, fh, 4*s, 1.5*s, uni(hex("#6b4a22")))
	tf := fontSpec{Size: 16 * s}
	name := string(w.name)
	tx := x + 10*s
	tw := measure(name, tf)
	if a.entrySelAll && name != "" {
		fillRect(cv, image.Rect(int(tx-1*s), int(mid-11*s), int(tx+tw+1*s), int(mid+11*s)), col(201, 164, 100, .55))
	}
	drawText(cv, name, tf, ink, tx, mid+16*s*.34, alignLeft)
	if math.Mod(a.now-a.caretAt, 1.06) < .53 {
		cxp := tx + tw + 1*s
		fillRect(cv, image.Rect(int(cxp), int(mid-10*s), int(cxp+math.Max(1, 1.5*s)), int(mid+10*s)), ink)
	}
	x += fw + 8*s
	a.button(cv, "savescore", "Save Score", x, y+2*s, st, false)
	return fh
}

func prettyShortDate(d string) string {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	return t.Format("Jan 2, 2006")
}

func (a *App) drawScores(cv *image.RGBA) {
	s := a.s
	cw := 600 * s
	pw := cw + 80*s + 6*s
	rowH := 30 * s
	st := btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
	ph := 3*s + 32*s + 32*s*1.15 + 6*s + 16*s*1.15 + 20*s + 24*s + boardSize*rowH + 20*s + 34*s + 18*s + a.buttonH(st) + 30*s + 3*s
	if lim := float64(a.H) - 20*s; ph > lim { // short windows: squeeze the rows
		rowH -= (ph - lim) / boardSize
		ph = lim
	}
	px, py := math.Round((float64(a.W)-pw)/2), math.Round((float64(a.H)-ph)/2)
	a.panelBox(cv, px, py, pw, ph)
	a.addHit("panel", px, py, pw, ph)
	cx := px + pw/2
	x0 := px + 3*s + 40*s
	y := py + 3*s + 32*s
	y += a.h1(cv, "CLAIM REGISTER", 32, cx, y) + 6*s
	y += a.h2(cv, "TOP TEN STRIKES ON THIS COMPUTER", cx, y) + 20*s

	ink, head, note := hex("#3a2412"), hex("#6b4a22"), hex("#7a5a30")
	cols := []struct {
		label string
		x     float64
		al    align
	}{{"#", 18, alignRight}, {"PROSPECTOR", 40, alignLeft}, {"SCORE", 290, alignRight}, {"TIME", 360, alignRight}, {"MOVES", 435, alignRight}, {"DRAW", 500, alignRight}, {"DATE", 600, alignRight}}
	hf := fontSpec{Size: 12 * s, Bold: true, Spacing: 1 * s}
	for _, c := range cols {
		drawText(cv, c.label, hf, head, x0+c.x*s, y+14*s, c.al)
	}
	y += 24 * s
	fillRect(cv, image.Rect(int(x0), int(y-4*s), int(x0+cw), int(y-4*s+math.Max(1, s))), col(107, 74, 34, .5))
	f, fb := fontSpec{Size: 15 * s}, fontSpec{Size: 15 * s, Bold: true}
	if len(a.sv.Scores) == 0 {
		drawText(cv, "No claims staked yet.", fontSpec{Size: 17 * s}, ink, cx, y+boardSize*rowH/2-8*s, alignCenter)
		drawText(cv, "Win a game and sign your name to get on the register.", fontSpec{Size: 14 * s}, note, cx, y+boardSize*rowH/2+16*s, alignCenter)
	}
	for i := 0; i < boardSize; i++ {
		ry := y + float64(i)*rowH
		if i%2 == 1 {
			fillRect(cv, image.Rect(int(x0), int(ry), int(x0+cw), int(ry+rowH)), col(107, 74, 34, .07))
		}
		if i >= len(a.sv.Scores) {
			continue
		}
		e := a.sv.Scores[i]
		if a.highlight == i+1 {
			fillRRect(cv, x0, ry+1*s, cw, rowH-2*s, 4*s, uni(col(232, 198, 106, .55)))
		}
		base := ry + rowH/2 + 15*s*.34
		medal := []string{"#b8860b", "#8c8c8c", "#a0522d"}
		rk := itoa(i + 1)
		if i < 3 {
			fillCircle(cv, x0+12*s, ry+rowH/2, 10*s, uni(hex(medal[i])))
			drawText(cv, rk, fontSpec{Size: 13 * s, Bold: true}, hex("#fff6d8"), x0+12*s, ry+rowH/2+13*s*.34, alignCenter)
		} else {
			drawText(cv, rk, f, note, x0+cols[0].x*s, base, alignRight)
		}
		drawText(cv, e.Name, fb, ink, x0+cols[1].x*s, base, alignLeft)
		drawText(cv, commas(e.Score), fb, hex("#7a2b16"), x0+cols[2].x*s, base, alignRight)
		drawText(cv, clock(e.Secs), f, ink, x0+cols[3].x*s, base, alignRight)
		drawText(cv, itoa(e.Moves), f, ink, x0+cols[4].x*s, base, alignRight)
		dm := "One"
		if e.Draw == 3 {
			dm = "Three"
		}
		drawText(cv, dm, f, ink, x0+cols[5].x*s, base, alignRight)
		drawText(cv, prettyShortDate(e.Date), fontSpec{Size: 13 * s}, note, x0+cols[6].x*s, base, alignRight)
	}
	y += boardSize*rowH + 20*s
	// online leaderboard: coming soon
	pill := "COMING SOON"
	pf := fontSpec{Size: 11 * s, Bold: true, Spacing: 1.5 * s}
	msg := "Online leaderboard"
	mf := fontSpec{Size: 15 * s, Italic: true}
	pwid := measure(pill, pf) + 16*s
	tot := measure(msg, mf) + 10*s + pwid
	bx := cx - tot/2
	fillRRect(cv, x0, y, cw, 34*s, 6*s, uni(col(107, 74, 34, .1)))
	drawText(cv, msg, mf, note, bx, y+17*s+15*s*.34, alignLeft)
	px2 := bx + measure(msg, mf) + 10*s
	fillRRect(cv, px2, y+7*s, pwid, 20*s, 10*s, uni(hex("#7a2b16")))
	drawText(cv, pill, pf, hex("#f7ecd2"), px2+pwid/2+.75*s, y+17*s+11*s*.36, alignCenter)
	y += 34*s + 18*s
	a.bigButtonSmall(cv, "close", "Back to the Table", cx, y)
}
