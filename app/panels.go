// The Options panel, and the Daily Claim, Statistics and Assay Stamps tabs
// of the records panel. Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"image"
	"math"
	"time"
)

// tabs draws a centred row of tab buttons with ids prefix+index.
func (a *App) tabs(cv *image.RGBA, labels []string, prefix string, on int, cx, y float64) {
	s := a.s
	st := btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
	gap := 6 * s
	total := -gap
	for _, l := range labels {
		total += a.buttonW(l, st) + gap
	}
	x := cx - total/2
	for i, l := range labels {
		b := st
		b.on = i == on
		x += a.button(cv, prefix+itoa(i), l, x, y, b, false) + gap
	}
}

func (a *App) drawRecordTab(cv *image.RGBA, x0, y, w, h float64) {
	switch a.recTab {
	case 1:
		a.drawDailyTab(cv, x0, y, w, h)
	case 2:
		a.drawStatsTab(cv, x0, y, w, h)
	case 3:
		a.drawStampsTab(cv, x0, y, w, h)
	}
}

/* -------------------------------------------------------------- daily tab */

func (a *App) drawDailyTab(cv *image.RGBA, x0, y, w, h float64) {
	s := a.s
	ink, note, red := hex("#3a2412"), hex("#7a5a30"), hex("#7a2b16")
	cx := x0 + w/2
	day := today()
	r, played := a.sv.Daily[day]
	// today's box
	boxH := 118 * s
	fillRRect(cv, x0, y, w, boxH, 8*s, uni(col(107, 74, 34, .1)))
	strokeRRect(cv, x0, y, w, boxH, 8*s, 1*s, uni(col(107, 74, 34, .35)))
	drawText(cv, "TODAY · "+prettyShortDate(day), fontSpec{Size: 13 * s, Bold: true, Spacing: 1 * s}, hex("#6b4a22"), cx, y+24*s, alignCenter)
	status, label := "Not struck yet. Everyone gets the same winnable deal today — Draw One.", "Play Today's Claim"
	if played && r.Won {
		status = "Struck in " + clock(r.Secs) + " with " + itoa(r.Moves) + " moves · " + commas(r.Score) + " points"
		label = "Play It Again"
	}
	drawText(cv, status, fontSpec{Size: 15 * s}, ink, cx, y+50*s, alignCenter)
	st := btnStyle{size: 15 * s, padX: 18 * s, padY: 7 * s}
	if a.touchUI() {
		st.padY = 11 * s
	}
	bw := a.buttonW(label, st)
	a.button(cv, "playdaily", label, cx-bw/2, y+66*s, st, false)
	y += boxH + 18*s
	// streaks
	cur, best := a.dailyStreaks()
	days := func(n int) string {
		if n == 1 {
			return "1 day"
		}
		return itoa(n) + " days"
	}
	line := []span{{Text: "Daily streak: ", Color: ink}, {Text: days(cur), Bold: true, Color: red}, {Text: "   ·   Best: ", Color: ink}, {Text: days(best), Bold: true, Color: red}}
	drawParagraph(cv, line, 16*s, 22*s, x0, y, w, alignCenter)
	y += 34 * s
	// the last two weeks
	drawText(cv, "THE LAST TWO WEEKS", fontSpec{Size: 12 * s, Bold: true, Spacing: 1 * s}, hex("#6b4a22"), cx, y+12*s, alignCenter)
	y += 22 * s
	avail := h - (boxH + 18*s + 34*s + 22*s)
	cell := math.Min(math.Min(w/7, avail/2), 64*s)
	gx := cx - cell*3.5
	t := time.Now().AddDate(0, 0, -13)
	for i := 0; i < 14; i++ {
		d := t.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		cxi := gx + float64(i%7)*cell + cell/2
		cyi := y + float64(i/7)*cell + cell/2
		rr := cell*.5 - 5*s
		res := a.sv.Daily[key]
		switch {
		case res.Won:
			fillCircle(cv, cxi, cyi, rr, newRad(cxi-rr*.3, cyi-rr*.3, 0, rr*1.4, stop{0, hex("#f6dc8e")}, stop{1, hex("#b8860b")}))
			strokeRRect(cv, cxi-rr, cyi-rr, 2*rr, 2*rr, rr, 1.2*s, uni(hex("#6b4a22")))
		case key == today():
			strokeRRect(cv, cxi-rr, cyi-rr, 2*rr, 2*rr, rr, 2*s, uni(hex("#b8860b")))
		default:
			strokeRRect(cv, cxi-rr, cyi-rr, 2*rr, 2*rr, rr, 1*s, uni(col(107, 74, 34, .35)))
		}
		c := note
		if res.Won {
			c = hex("#3a2412")
		}
		drawText(cv, d.Format("Mon")[:2], fontSpec{Size: 10 * s, Spacing: .5 * s}, c, cxi, cyi-3*s, alignCenter)
		drawText(cv, itoa(d.Day()), fontSpec{Size: 14 * s, Bold: true}, c, cxi, cyi+13*s, alignCenter)
	}
}

/* -------------------------------------------------------------- stats tab */

func (a *App) drawStatsTab(cv *image.RGBA, x0, y, w, h float64) {
	s := a.s
	ink, head, note, red := hex("#3a2412"), hex("#6b4a22"), hex("#7a5a30"), hex("#7a2b16")
	d1, d3 := a.sv.Draw1, a.sv.Draw3
	rate := func(won, played int) string {
		if played == 0 {
			return "—"
		}
		return itoa(int(math.Round(100*float64(won)/float64(played)))) + "%"
	}
	tm := func(secs int) string {
		if secs <= 0 {
			return "—"
		}
		return clock(secs)
	}
	avg := func(total, won int) string {
		if won == 0 || total == 0 {
			return "—"
		}
		return clock(total / won)
	}
	num := func(n int) string {
		if n <= 0 {
			return "—"
		}
		return commas(n)
	}
	minNZ := func(a, b int) int {
		if a == 0 {
			return b
		}
		if b == 0 {
			return a
		}
		return min(a, b)
	}
	rows := [][4]string{
		{"Games played", itoa(a.sv.Played), itoa(d1.Played), itoa(d3.Played)},
		{"Games won", itoa(a.sv.Won), itoa(d1.Won), itoa(d3.Won)},
		{"Win rate", rate(a.sv.Won, a.sv.Played), rate(d1.Won, d1.Played), rate(d3.Won, d3.Played)},
		{"Current streak", itoa(a.sv.Streak), itoa(d1.Streak), itoa(d3.Streak)},
		{"Longest streak", itoa(a.sv.BestStreak), itoa(d1.BestStreak), itoa(d3.BestStreak)},
		{"Fastest win", tm(minNZ(d1.BestSecs, d3.BestSecs)), tm(d1.BestSecs), tm(d3.BestSecs)},
		{"Average win", avg(d1.WinSecs+d3.WinSecs, d1.Won+d3.Won), avg(d1.WinSecs, d1.Won), avg(d3.WinSecs, d3.Won)},
		{"Best score", num(max(d1.BestScore, d3.BestScore)), num(d1.BestScore), num(d3.BestScore)},
	}
	colX := []float64{0, w * .62, w * .81, w}
	hf := fontSpec{Size: 12 * s, Bold: true, Spacing: 1 * s}
	for i, l := range []string{"", "ALL GAMES", "DRAW ONE", "DRAW THREE"} {
		if i > 0 {
			drawText(cv, l, hf, head, x0+colX[i], y+14*s, alignRight)
		}
	}
	y += 24 * s
	fillRect(cv, image.Rect(int(x0), int(y-4*s), int(x0+w), int(y-4*s+math.Max(1, s))), col(107, 74, 34, .5))
	rowH := math.Min(36*s, (h-24*s-60*s)/float64(len(rows)))
	f, fb := fontSpec{Size: 16 * s}, fontSpec{Size: 16 * s, Bold: true}
	for i, r := range rows {
		ry := y + float64(i)*rowH
		if i%2 == 1 {
			fillRect(cv, image.Rect(int(x0), int(ry), int(x0+w), int(ry+rowH)), col(107, 74, 34, .07))
		}
		base := ry + rowH/2 + 16*s*.34
		drawText(cv, r[0], f, ink, x0+10*s, base, alignLeft)
		drawText(cv, r[1], fb, red, x0+colX[1], base, alignRight)
		drawText(cv, r[2], f, ink, x0+colX[2], base, alignRight)
		drawText(cv, r[3], f, ink, x0+colX[3]-10*s, base, alignRight)
	}
	y += float64(len(rows))*rowH + 18*s
	poke := []span{{Text: "Vegas poke: ", Color: ink}, {Text: dollars(a.pokeNow()), Bold: true, Color: red}}
	if a.sv.Poke == 0 && !a.sv.Vegas {
		poke = []span{{Text: "Try Vegas scoring in Options to start a poke of gold dust.", Color: note}}
	}
	drawParagraph(cv, poke, 15*s, 20*s, x0, y, w, alignCenter)
}

/* ------------------------------------------------------------- stamps tab */

func (a *App) drawStampsTab(cv *image.RGBA, x0, y, w, h float64) {
	s := a.s
	ink, note := hex("#3a2412"), hex("#7a5a30")
	n := len(stampDefs)
	rowsN := (n + 1) / 2
	rowH := h / float64(rowsN)
	colW := w / 2
	earned := 0
	for i, d := range stampDefs {
		cx := x0 + float64(i%2)*colW
		cy := y + float64(i/2)*rowH
		date, ok := a.sv.Stamps[d.id]
		if ok {
			earned++
		}
		r := math.Min(rowH*.42, 20*s)
		sx, sy := cx+r+6*s, cy+rowH/2
		if ok {
			fillCircle(cv, sx, sy, r, newRad(sx-r*.35, sy-r*.35, 0, r*1.5, stop{0, hex("#f6dc8e")}, stop{1, hex("#a8790f")}))
			strokeRRect(cv, sx-r, sy-r, 2*r, 2*r, r, 1.5*s, uni(hex("#6b4a22")))
			strokeRRect(cv, sx-r*.72, sy-r*.72, 1.44*r, 1.44*r, r*.72, 1*s, uni(col(107, 74, 34, .6)))
		} else {
			strokeRRect(cv, sx-r, sy-r, 2*r, 2*r, r, 1.2*s, uni(col(107, 74, 34, .35)))
		}
		initial := string([]rune(d.name)[0])
		ic := col(107, 74, 34, .35)
		if ok {
			ic = hex("#5a3a12")
		}
		drawText(cv, initial, fontSpec{Size: r * 1.05, Bold: true}, ic, sx, sy+r*.37, alignCenter)
		tx := sx + r + 10*s
		nc, dc := ink, note
		if !ok {
			nc, dc = col(58, 36, 18, .55), col(122, 90, 48, .6)
		}
		drawText(cv, d.name, fontSpec{Size: 15 * s, Bold: true}, nc, tx, sy-3*s, alignLeft)
		sub := d.desc
		if ok {
			sub += "  " + prettyShortDate(date)
		}
		drawText(cv, sub, fontSpec{Size: 12.5 * s}, dc, tx, sy+14*s, alignLeft)
	}
	_ = earned
}

/* ----------------------------------------------------------------- options */

func (a *App) option(id string) {
	sv := &a.sv
	inGame := a.started && a.g != nil && a.g.Counted && !a.g.Won
	nextDeal := func() {
		if inGame {
			a.toast("That takes effect on your next deal.")
		}
	}
	switch id {
	case "standard", "vegas":
		if sv.Vegas != (id == "vegas") {
			sv.Vegas = id == "vegas"
			nextDeal()
			if !inGame && a.g != nil && a.g.Moves == 0 && a.g.Daily == "" && a.started {
				a.g.Vegas = sv.Vegas
				a.g.Score = map[bool]int{true: -vegasBuyIn, false: 0}[sv.Vegas]
			}
		}
	case "any", "winnable":
		if sv.Winnable != (id == "winnable") {
			sv.Winnable = id == "winnable"
			nextDeal()
		}
	case "right", "left":
		sv.LeftHanded = id == "left"
		a.refreshLook()
	case "two", "four":
		sv.FourColor = id == "four"
		a.refreshLook()
	case "normal", "large":
		sv.LargeCards = id == "large"
		a.refreshLook()
	case "auto", "touch", "mouse":
		sv.Controls = map[string]int{"auto": 0, "touch": 1, "mouse": 2}[id]
		a.refreshLook()
	default:
		if len(id) == 5 && id[:4] == "back" {
			i := int(id[4] - '0')
			if unlocked(cardBacks[i], sv.Won) {
				sv.Back = i
				a.refreshLook()
			} else {
				a.toast(cardBacks[i].name + " unlocks after " + itoa(cardBacks[i].wins) + " wins.")
			}
		}
		if len(id) == 6 && id[:5] == "table" {
			i := int(id[5] - '0')
			if unlocked(tables[i], sv.Won) {
				sv.Table = i
				a.refreshLook()
			} else {
				a.toast(tables[i].name + " unlocks after " + itoa(tables[i].wins) + " wins.")
			}
		}
	}
	sv.write()
}

var thumbCache = map[[3]int]*image.RGBA{}

func thumb(kind, i, w int) *image.RGBA {
	k := [3]int{kind, i, w}
	if t := thumbCache[k]; t != nil {
		return t
	}
	var t *image.RGBA
	if kind == 0 { // card back, without its shadow margin
		src := backArt(i)
		p := A.meta.Pad
		cw, ch := A.meta.CW, int(float64(A.meta.CW)*1.4)
		crop := image.NewRGBA(image.Rect(0, 0, cw, ch))
		for y := 0; y < ch; y++ {
			copy(crop.Pix[y*crop.Stride:y*crop.Stride+cw*4], src.Pix[(y+p)*src.Stride+p*4:(y+p)*src.Stride+(p+cw)*4])
		}
		t = scaleImage(crop, w, int(math.Round(float64(w)*1.4)), true)
	} else {
		t = scaleImage(tableSource(i), w, int(math.Round(float64(w)*9/16)), true)
	}
	thumbCache[k] = t
	return t
}

func (a *App) drawOptions(cv *image.RGBA) {
	S := a.s
	// lay out at full size, then shrink to fit small windows
	natW, natH := 900.0, 520.0
	if a.touchUI() {
		natH = 600
	}
	k := math.Min(1, math.Min((float64(a.W)-20*S)/(natW*S), (float64(a.H)-16*S)/(natH*S)))
	s := S * k
	pw, ph := natW*s, natH*s
	px, py := math.Round((float64(a.W)-pw)/2), math.Round((float64(a.H)-ph)/2)
	a.panelBox(cv, px, py, pw, ph)
	a.addHit("panel", px, py, pw, ph)
	cx := px + pw/2
	y := py + 28*s
	f := fontSpec{Size: 32 * s, Bold: true, Spacing: 6 * s}
	drawTextShadow(cv, "OPTIONS", f, hex("#7a2b16"), col(255, 255, 255, .53), cx+3*s, y+32*s*.92, 0, 1*s, alignCenter)
	y += 32*s*1.15 + 20*s
	ink, head, note := hex("#3a2412"), hex("#6b4a22"), hex("#7a5a30")
	st := btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
	rowH := 44 * s
	if a.touchUI() {
		st = btnStyle{size: 15 * s, padX: 14 * s, padY: 10 * s}
		rowH = 54 * s
	}
	section := func(x, y float64, t string, w float64) {
		drawText(cv, t, fontSpec{Size: 12 * s, Bold: true, Spacing: 1.5 * s}, head, x, y+12*s, alignLeft)
		fillRect(cv, image.Rect(int(x), int(y+18*s), int(x+w), int(y+18*s+math.Max(1, s))), col(107, 74, 34, .4))
	}
	seg := func(x, y float64, label string, opts [][2]string, on int) {
		drawText(cv, label, fontSpec{Size: 15 * s}, ink, x, y+a.buttonH(st)/2+15*s*.34, alignLeft)
		bx := x + 112*s
		for i, o := range opts {
			b := st
			b.on = i == on
			bx += a.button(cv, "opt:"+o[0], o[1], bx, y, b, false) + 4*s
		}
	}
	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	// left: gameplay and cards
	lx := px + 46*s
	ly := y
	section(lx, ly, "GAMEPLAY", 400*s)
	ly += 30 * s
	seg(lx, ly, "Scoring", [][2]string{{"standard", "Standard"}, {"vegas", "Vegas"}}, b2i(a.sv.Vegas))
	ly += rowH
	seg(lx, ly, "Deals", [][2]string{{"any", "Any Deal"}, {"winnable", "Winnable Only"}}, b2i(a.sv.Winnable))
	ly += rowH
	seg(lx, ly, "Layout", [][2]string{{"right", "Right-Handed"}, {"left", "Left-Handed"}}, b2i(a.sv.LeftHanded))
	ly += rowH + 12*s
	section(lx, ly, "CARDS & CONTROLS", 400*s)
	ly += 30 * s
	seg(lx, ly, "Suits", [][2]string{{"two", "Two Colours"}, {"four", "Four Colours"}}, b2i(a.sv.FourColor))
	ly += rowH
	seg(lx, ly, "Card size", [][2]string{{"normal", "Standard"}, {"large", "Large"}}, b2i(a.sv.LargeCards))
	ly += rowH
	seg(lx, ly, "Controls", [][2]string{{"auto", "Automatic"}, {"touch", "Touch"}, {"mouse", "Mouse"}}, a.sv.Controls)
	ly += rowH
	tip := "Vegas: a $52 stake buys each deal, every card sent up pays $5."
	if !a.sv.Vegas {
		tip = "Winnable Only deals a game the solver has proven can be won."
	}
	drawText(cv, tip, fontSpec{Size: 12.5 * s, Italic: true}, note, lx, ly+8*s, alignLeft)
	// right: card back and table
	rx := px + pw - 46*s - 330*s
	ry := y
	section(rx, ry, "CARD BACK", 330*s)
	ry += 30 * s
	tw := 62 * s
	for i, l := range cardBacks {
		x := rx + float64(i)*84*s
		a.swatch(cv, "opt:back"+itoa(i), thumb(0, i, int(tw)), x, ry, l, a.sv.Back == i, s)
	}
	ry += tw*1.4 + 44*s
	section(rx, ry, "TABLE", 330*s)
	ry += 30 * s
	for i, l := range tables {
		x := rx + float64(i)*84*s
		a.swatch(cv, "opt:table"+itoa(i), thumb(1, i, int(74*s)), x, ry, l, a.sv.Table == i, s)
	}
	// back button
	bst := btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
	if a.touchUI() {
		bst.padY = 10 * s
	}
	bw := a.buttonW("Back to the Table", bst)
	a.button(cv, "close", "Back to the Table", cx-bw/2, py+ph-30*s-a.buttonH(bst), bst, false)
}

// swatch is a clickable thumbnail of a card back or table, locked until won.
func (a *App) swatch(cv *image.RGBA, id string, img *image.RGBA, x, y float64, l look, sel bool, s float64) {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	a.addHit(id, x-4*s, y-4*s, w+8*s, h+34*s)
	ok := unlocked(l, a.sv.Won)
	if sel {
		fillRRect(cv, x-5*s, y-5*s, w+10*s, h+10*s, 6*s, uni(hex("#7a2b16")))
	} else if a.hover == id {
		fillRRect(cv, x-4*s, y-4*s, w+8*s, h+8*s, 6*s, uni(col(201, 164, 100, .6)))
	}
	if ok {
		blit(cv, img, int(x), int(y))
	} else {
		blitAlpha(cv, img, int(x), int(y), .5)
	}
	strokeRRect(cv, x, y, w, h, 3*s, 1*s, uni(col(58, 36, 18, .6)))
	name := l.name
	c := hex("#3a2412")
	if !ok {
		name = "Win " + itoa(l.wins)
		c = hex("#7a5a30")
	}
	drawText(cv, name, fontSpec{Size: 11.5 * s, Bold: sel}, c, x+w/2, y+h+16*s, alignCenter)
}
