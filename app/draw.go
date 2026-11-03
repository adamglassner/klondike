// Rendering: the table, cards, toolbar and panels, matching the original
// game's canvas drawing and CSS. Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
)

type chromeCache struct {
	barBG  *image.RGBA
	veil   *image.RGBA
	shadow map[[2]int]*image.RGBA
}

func (a *App) chromeC() *chromeCache {
	if a.chrome == nil {
		c := &chromeCache{shadow: map[[2]int]*image.RGBA{}}
		bar := int(a.bar)
		c.barBG = image.NewRGBA(image.Rect(0, 0, a.W, bar+int(14*a.s)))
		g := newLin(0, 0, 0, a.bar, stop{0, hex("#5e3d24")}, stop{.7, hex("#3a2414")}, stop{1, hex("#2e1c10")})
		fillGrad(c.barBG, image.Rect(0, 0, a.W, bar), g, draw.Src)
		bw := int(math.Round(2 * a.s))
		fillRectSrc(c.barBG, image.Rect(0, bar-bw, a.W, bar), hex("#170d07"))
		// box-shadow: 0 3px 10px rgba(0,0,0,.6) on the table below
		sh := newLin(0, a.bar, 0, a.bar+13*a.s, stop{0, col(0, 0, 0, .5)}, stop{.35, col(0, 0, 0, .22)}, stop{1, col(0, 0, 0, 0)})
		fillGrad(c.barBG, image.Rect(0, bar, a.W, c.barBG.Rect.Dy()), sh, draw.Src)
		// the veil behind panels
		c.veil = image.NewRGBA(image.Rect(0, 0, a.W, a.H))
		r := math.Hypot(float64(a.W)/2, float64(a.H)/2)
		fillGrad(c.veil, c.veil.Rect, newRad(float64(a.W)/2, float64(a.H)/2, 0, r,
			stop{0, col(20, 10, 4, .35)}, stop{1, col(10, 5, 2, .8)}), draw.Src)
		a.chrome = c
	}
	return a.chrome
}

func (a *App) addHit(id string, x, y, w, h float64) {
	if !a.noHits {
		a.hits = append(a.hits, hit{id, rectF{x, y, w, h}})
	}
}

// render draws a complete frame into a.canvas.
func (a *App) render() *image.RGBA {
	a.hits = a.hits[:0]
	cv := a.canvas
	bar := int(a.bar)
	draw.Draw(cv, image.Rect(0, bar, a.W, a.H), a.table, image.Point{}, draw.Src)
	a.drawTable(cv)
	a.drawToolbar(cv)
	a.drawSeal(cv)
	switch a.panel {
	case panelSplash:
		a.drawVeil(cv)
		a.drawSplash(cv)
	case panelWin:
		a.drawVeil(cv)
		a.drawWin(cv)
	case panelAbout:
		a.drawVeil(cv)
		a.drawAbout(cv)
	}
	if a.ddOpen {
		a.drawDropdown(cv)
	}
	return cv
}

/* ------------------------------------------------------------- the table */

func (a *App) drawTable(cv *image.RGBA) {
	sp := a.spr
	oy := a.bar
	slot := func(x, y float64, which int) {
		blit(cv, sp.img[which], int(math.Round(x-sp.pad)), int(math.Round(y+oy-sp.pad)))
	}
	if a.g != nil && len(a.g.Waste) > 0 {
		slot(a.colX(0), a.top0, sprSlotR)
	} else {
		slot(a.colX(0), a.top0, sprSlot)
	}
	slot(a.colX(1), a.top0, sprSlot)
	for f := 0; f < 4; f++ {
		slot(a.colX(3+f), a.top0, sprSlotA)
	}
	for t := 0; t < 7; t++ {
		slot(a.colX(t), a.tabY, sprSlot)
	}
	if a.g == nil {
		return
	}
	T := a.targets()
	var moving, dragged []target
	for _, t := range T {
		id := t.c.ID
		d := a.disp[id]
		if a.cel != nil && t.pile.Kind == pFound {
			continue
		}
		if a.drag != nil && a.drag.ids[id] {
			dragged = append(dragged, t)
			continue
		}
		if math.Abs(t.x-d.x)+math.Abs(t.y-d.y) > 2 {
			moving = append(moving, t)
		} else {
			a.paint(cv, t, false)
		}
	}
	for _, t := range moving {
		a.paint(cv, t, false)
	}
	for _, t := range dragged {
		a.paint(cv, t, true)
	}
	if a.cel != nil {
		a.drawCelebration(cv)
	}
}

func (a *App) drawCelebration(cv *image.RGBA) {
	c := a.cel
	sp := a.spr
	for _, t := range a.celebrationCards() {
		blit(cv, sp.img[t.c.ID], int(math.Round(t.x-sp.pad)), int(math.Round(t.y+a.bar-sp.pad)))
	}
	for _, p := range c.sparks {
		age := (a.now - p.born) / p.life
		al := (1 - age*age) * (.65 + .35*math.Sin(a.now*18+p.phase))
		g := c.glow[p.size]
		h := float64(g.Rect.Dx()) / 2
		blitAlpha(cv, g, int(p.x-h), int(p.y+a.bar-h), clamp(al, 0, 1))
	}
}

// paint draws one card where it is displayed, mid-flip if it's turning.
func (a *App) paint(cv *image.RGBA, t target, shadow bool) {
	sp := a.spr
	d := a.disp[t.c.ID]
	ft := (a.now - d.f0) / .23
	sx, up := 1.0, d.up
	if d.f0 >= 0 && ft < 1 {
		sx = math.Abs(1 - 2*ft)
		if ft < .5 {
			up = !d.up
		}
	}
	img := sp.img[sprBack]
	if up {
		img = sp.img[t.c.ID]
	}
	x, y := d.x, d.y+a.bar
	if shadow {
		l := sp.img[sprLift]
		blit(cv, l, int(math.Round(x-sp.pad-sp.lift)), int(math.Round(y-sp.pad-sp.lift)))
	}
	if sx >= .999 {
		blit(cv, img, int(math.Round(x-sp.pad)), int(math.Round(y-sp.pad)))
		return
	}
	w := float64(img.Rect.Dx()) * math.Max(sx, .02)
	blitScaled(cv, img, x+a.cw/2-w/2, y-sp.pad, w, float64(img.Rect.Dy()))
}

func (a *App) drawSeal(cv *image.RGBA) {
	s := a.s
	t := "YUKON TERRITORY · 1898 · v" + version + " · © 2026 IRONITE"
	f := fontSpec{Size: 12 * s, Spacing: 2 * s}
	drawText(cv, t, f, col(232, 198, 106, .6), float64(a.W)-12*s, float64(a.H)-8*s-3*s, alignRight)
}

/* ---------------------------------------------------------------- toolbar */

// button draws a toolbar-style button (the original's CSS) and registers it.
type btnStyle struct {
	size float64 // font size
	padX float64
	padY float64
	off  bool
	on   bool // selected tab
	minW float64
}

func (a *App) buttonW(label string, st btnStyle) float64 {
	return math.Max(st.minW, measure(label, fontSpec{Size: st.size})+2*st.padX+2*a.s)
}
func (a *App) buttonH(st btnStyle) float64 { return math.Round(st.size*1.15 + 2*st.padY + 2*a.s) }

func (a *App) button(cv *image.RGBA, id, label string, x, y float64, st btnStyle, arrow bool) float64 {
	s := a.s
	w := a.buttonW(label, st)
	if arrow {
		w += 16 * s
	}
	h := a.buttonH(st)
	a.addHit(id, x, y, w, h)
	hover, down := a.hover == id, a.press == id && a.hover == id
	if down {
		y += 1 * s
	}
	top, bot, ink := hex("#f3e1b4"), hex("#c9a464"), hex("#2b1a0c")
	if st.off {
		top, bot, ink = hex("#8a7558"), hex("#6a5638"), hex("#e9dcc0")
	}
	if st.on {
		top, bot, ink = hex("#7a2b16"), hex("#5a1f10"), hex("#f7ecd2")
	}
	if hover {
		top, bot = bright(top, 1.08), bright(bot, 1.08)
	}
	r := 5 * s
	// 0 1px 3px rgba(0,0,0,.6)
	fillRRect(cv, x-.5*s, y+.5*s, w+s, h+1.5*s, r+1*s, uni(col(0, 0, 0, .22)))
	fillRRect(cv, x, y+1*s, w, h, r, uni(col(0, 0, 0, .3)))
	fillRRect(cv, x, y, w, h, r, newLin(0, y, 0, y+h, stop{0, top}, stop{1, bot}))
	fillRect(cv, image.Rect(int(x+r), int(y+1*s), int(x+w-r), int(y+2*s)), col(255, 255, 255, .45)) // inset highlight
	strokeRRect(cv, x, y, w, h, r, 1*s, uni(hex("#6b4a22")))
	tx := x + w/2
	if arrow {
		tx -= 8 * s
	}
	drawText(cv, label, fontSpec{Size: st.size}, ink, tx, y+h/2+st.size*.33, alignCenter)
	if arrow {
		ax, ay := x+w-st.padX-4*s, y+h/2
		fillPoly(cv, [][2]float64{{ax - 4.5*s, ay - 2.5*s}, {ax + 4.5*s, ay - 2.5*s}, {ax, ay + 3*s}}, uni(ink))
	}
	return w
}

func bright(c color.RGBA, f float64) color.RGBA {
	m := func(v uint8) uint8 { return uint8(math.Min(float64(c.A), float64(v)*f)) }
	return color.RGBA{m(c.R), m(c.G), m(c.B), c.A}
}

func (a *App) drawToolbar(cv *image.RGBA) {
	s := a.s
	ch := a.chromeC()
	blit(cv, ch.barBG, 0, 0)
	toolbar := a.panel == panelNone
	if !toolbar { // under a panel the toolbar can't be clicked
		a.noHits = true
		defer func() { a.noHits = false }()
	}
	mid := (a.bar - 2*s) / 2
	x := 14 * s
	// title: 22px bold, letter-spacing 5px, gold, text-shadow 0 1px 0 #000
	tf := fontSpec{Size: 22 * s, Bold: true, Spacing: 5 * s}
	base := mid + 22*s*.35
	drawText(cv, "KLONDIKE", tf, col(232, 198, 106, .22), x, base, alignLeft) // soft glow
	x += drawTextShadow(cv, "KLONDIKE", tf, hex("#e8c66a"), hex("#000000"), x, base, 0, 1*s, alignLeft)
	x += 6 * s
	x += drawText(cv, "GOLD RUSH SOLITAIRE", fontSpec{Size: 11 * s, Spacing: 2 * s}, hex("#c9a96a"), x, base, alignLeft)
	x += 12 * s
	st := btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
	bh := a.buttonH(st)
	by := math.Round(mid - bh/2)
	gap := 8 * s
	x += gap
	x += a.button(cv, "new", "New Game", x, by, st, false) + gap
	x += a.button(cv, "undo", "Undo", x, by, st, false) + gap
	drawLabel := "Draw One"
	if a.sv.Draw == 3 {
		drawLabel = "Draw Three"
	}
	dst := st
	dst.minW = a.buttonW("Draw Three", st)
	a.ddX, a.ddY, a.ddW = x, by+bh, dst.minW+16*s
	x += a.button(cv, "draw", drawLabel, x, by, dst, true) + gap
	ms := st
	ms.off = !a.sv.Music
	ml := "♪ Music"
	if !a.sv.Music {
		ml = "♪ Muted"
	}
	x += a.button(cv, "music", ml, x, by, ms, false) + gap
	// volume slider (input type=range, accent #c9a464)
	sw := 80 * s
	a.addHit("vol", x, by, sw, bh)
	v := float64(a.sv.Vol) / 100
	ty := mid
	fillRRect(cv, x, ty-2.5*s, sw, 5*s, 2.5*s, uni(col(255, 255, 255, .82)))
	fillRRect(cv, x, ty-2.5*s, math.Max(5*s, sw*v), 5*s, 2.5*s, uni(hex("#c9a464")))
	kx := x + 8*s + (sw-16*s)*v
	fillCircle(cv, kx, ty, 8*s, uni(hex("#c9a464")))
	fillCircle(cv, kx, ty, 8*s-1.2*s, uni(bright(hex("#c9a464"), map[bool]float64{true: 1.1, false: 1}[a.hover == "vol"])))
	x += sw + gap
	ss := st
	ss.off = !a.sv.Sfx
	x += a.button(cv, "sfx", "Sounds", x, by, ss, false) + gap
	x += a.button(cv, "about", "Credits", x, by, st, false) + gap
	// stats, right-aligned: Score, Moves, Time, Won
	g := a.g
	score, moves, secs := 0, 0, 0
	if g != nil {
		score, moves, secs = g.Score, g.Moves, int(g.Elapsed)
	}
	items := [][2]string{{"Score", commas(score)}, {"Moves", itoa(moves)}, {"Time", clock(secs)}, {"Won", itoa(a.sv.Won) + "/" + itoa(a.sv.Played)}}
	f, fb := fontSpec{Size: 15 * s}, fontSpec{Size: 15 * s, Bold: true}
	rx := float64(a.W) - 14*s
	for i := len(items) - 1; i >= 0; i-- {
		rx -= drawText(cv, items[i][1], fb, hex("#e8c66a"), rx, base, alignRight)
		rx -= drawText(cv, items[i][0]+" ", f, hex("#f1dfb8"), rx, base, alignRight)
		rx -= 20 * s
	}
}

func (a *App) drawDropdown(cv *image.RGBA) {
	s := a.s
	x, y, w := a.ddX, a.ddY+2*s, a.ddW
	ih := 26 * s
	fillRRect(cv, x+2*s, y+3*s, w, 2*ih+6*s, 4*s, uni(col(0, 0, 0, .35)))
	fillRRect(cv, x, y, w, 2*ih+6*s, 4*s, uni(hex("#f7ecd2")))
	strokeRRect(cv, x, y, w, 2*ih+6*s, 4*s, 1*s, uni(hex("#6b4a22")))
	for i, opt := range []struct {
		id, label string
		v         int
	}{{"dd1", "Draw One", 1}, {"dd3", "Draw Three", 3}} {
		iy := y + 3*s + float64(i)*ih
		a.addHit(opt.id, x, iy, w, ih)
		if a.hover == opt.id || (a.hover == "" && a.sv.Draw == opt.v) {
			fillRect(cv, image.Rect(int(x+2*s), int(iy), int(x+w-2*s), int(iy+ih)), hex("#c9a464"))
		}
		mark := ""
		if a.sv.Draw == opt.v {
			mark = "✓"
		}
		drawText(cv, mark, fontSpec{Size: 14 * s}, hex("#2b1a0c"), x+9*s, iy+ih/2+14*s*.33, alignLeft)
		drawText(cv, opt.label, fontSpec{Size: 14 * s}, hex("#2b1a0c"), x+26*s, iy+ih/2+14*s*.33, alignLeft)
	}
}

/* ------------------------------------------------------------------ panels */

func (a *App) drawVeil(cv *image.RGBA) { blit(cv, a.chromeC().veil, 0, 0) }

// panelBox draws the parchment panel (the original's .panel CSS).
func (a *App) panelBox(cv *image.RGBA, x, y, w, h float64) {
	s := a.s
	key := [2]int{int(w), int(h)}
	sh := a.chromeC().shadow[key]
	if sh == nil {
		sh = shadowRRect(w, h, 10*s, int(30*s), col(0, 0, 0, .7))
		a.chrome.shadow[key] = sh
	}
	m := float64(int(30*s) * 2)
	blit(cv, sh, int(x-m), int(y+20*s-m))
	fillRRect(cv, x, y, w, h, 10*s, newLin(0, y, 0, y+h, stop{0, hex("#f7ecd2")}, stop{1, hex("#e6d2a6")}))
	strokeRRect(cv, x, y, w, h, 10*s, 3*s, uni(hex("#6b4a22")))
	strokeRRect(cv, x+7*s, y+7*s, w-14*s, h-14*s, 3*s, 2*s, uni(hex("#e8c66a")))
}

// heading draws an h1/h2 line centered at cx; returns its height.
func (a *App) h1(cv *image.RGBA, t string, size, cx, y float64) float64 {
	f := fontSpec{Size: size * a.s, Bold: true, Spacing: 6 * a.s}
	lh := size * a.s * 1.15
	w := measure(t, f) - 6*a.s // letter-spacing after the last letter doesn't count for centering in CSS? it does; keep visual centre
	_ = w
	drawTextShadow(cv, t, f, hex("#7a2b16"), col(255, 255, 255, .53), cx+3*a.s, y+lh*.8, 0, 1*a.s, alignCenter)
	return lh
}

func (a *App) h2(cv *image.RGBA, t string, cx, y float64) float64 {
	f := fontSpec{Size: 16 * a.s, Spacing: 3 * a.s}
	lh := 16 * a.s * 1.15
	drawText(cv, t, f, hex("#6b4a22"), cx+1.5*a.s, y+lh*.8, alignCenter)
	return lh
}

func (a *App) bigButton(cv *image.RGBA, id, label string, cx, y float64) float64 {
	st := btnStyle{size: 18 * a.s, padX: 28 * a.s, padY: 10 * a.s}
	w := a.buttonW(label, st)
	a.button(cv, id, label, cx-w/2, y, st, false)
	return a.buttonH(st)
}

func (a *App) drawSplash(cv *image.RGBA) {
	s := a.s
	pf := 17 * s
	body := hex("#3a2412")
	lines := []string{"Stake your claim on Bonanza Creek.", "Build each suit from Ace to King to strike it rich."}
	hint := []string{"Drag cards, or click a card to send it where it fits.", "F2 new game · Ctrl+Z undo · F11 full screen"}
	cw := 0.0
	for _, l := range lines {
		cw = math.Max(cw, measure(l, fontSpec{Size: pf}))
	}
	for _, l := range hint {
		cw = math.Max(cw, measure(l, fontSpec{Size: 13 * s}))
	}
	cw = math.Min(cw, 520*s)
	pw := cw + 96*s + 6*s
	lh := pf * 1.55
	ph := 3*s + 34*s + 40*s*1.15 + 6*s + 16*s*1.15 + 18*s + 8*s + 2*lh + 22*s + a.buttonH(btnStyle{size: 18 * s, padY: 10 * s}) + 16*s + 2*13*s*1.25 + 18*s + 32*s + 34*s + 3*s
	px, py := math.Round((float64(a.W)-pw)/2), math.Round((float64(a.H)-ph)/2)
	a.panelBox(cv, px, py, pw, ph)
	a.addHit("panel", px, py, pw, ph)
	cx := px + pw/2
	y := py + 3*s + 34*s
	y += a.h1(cv, "KLONDIKE", 40, cx, y) + 6*s
	y += a.h2(cv, "A GOLD RUSH SOLITAIRE", cx, y) + 18*s + 8*s
	for _, l := range lines {
		drawText(cv, l, fontSpec{Size: pf}, body, cx, y+lh/2+pf*.35, alignCenter)
		y += lh
	}
	y += 22 * s
	y += a.bigButton(cv, "start", "Stake Your Claim", cx, y) + 16*s
	for _, l := range hint {
		drawText(cv, l, fontSpec{Size: 13 * s}, hex("#7a5a30"), cx, y+13*s*.95, alignCenter)
		y += 13 * s * 1.25
	}
	y += 18 * s
	a.byLine(cv, cx, y)
}

// byLine draws "AN IRONITE GAME [logo]".
func (a *App) byLine(cv *image.RGBA, cx, y float64) {
	s := a.s
	f := fontSpec{Size: 13 * s, Spacing: 1 * s}
	t := "AN IRONITE GAME "
	tw := measure(t, f)
	lh := 22 * s
	lw := lh * float64(A.logo.Rect.Dx()) / float64(A.logo.Rect.Dy())
	bw := lw + 18*s + 4*s
	x := cx - (tw+bw)/2
	mid := y + 16*s
	drawText(cv, t, f, hex("#7a5a30"), x, mid+13*s*.35, alignLeft)
	bx := x + tw + 4*s
	fillRRect(cv, bx, mid-lh/2-5*s, lw+18*s, lh+10*s, 5*s, uni(hex("#003b2f")))
	logo := a.logoAt(int(lh))
	blit(cv, logo, int(bx+9*s), int(mid-lh/2))
}

var logoCache = map[int]*image.RGBA{}

func (a *App) logoAt(h int) *image.RGBA {
	if l := logoCache[h]; l != nil {
		return l
	}
	w := int(math.Round(float64(h) * float64(A.logo.Rect.Dx()) / float64(A.logo.Rect.Dy())))
	l := scaleImage(A.logo, w, h, true)
	logoCache[h] = l
	return l
}

func (a *App) drawWin(cv *image.RGBA) {
	s := a.s
	pf := 17 * s
	lh := pf * 1.55
	lines := layoutSpans(a.winSpans, pf, 520*s)
	cw := 0.0
	for _, l := range lines {
		cw = math.Max(cw, l.w)
	}
	cw = math.Max(cw, measure("EUREKA!", fontSpec{Size: 40 * s, Bold: true, Spacing: 6 * s}))
	pw := cw + 96*s + 6*s
	ph := 3*s + 34*s + 40*s*1.15 + 6*s + 16*s*1.15 + 18*s + 8*s + float64(len(lines))*lh + 22*s + a.buttonH(btnStyle{size: 18 * s, padY: 10 * s}) + 34*s + 3*s
	px, py := math.Round((float64(a.W)-pw)/2), math.Round((float64(a.H)-ph)/2)
	a.panelBox(cv, px, py, pw, ph)
	a.addHit("panel", px, py, pw, ph)
	cx := px + pw/2
	y := py + 3*s + 34*s
	y += a.h1(cv, "EUREKA!", 40, cx, y) + 6*s
	y += a.h2(cv, "YOU STRUCK GOLD", cx, y) + 18*s + 8*s
	y += drawParagraph(cv, a.winSpans, pf, lh, px+51*s, y, cw, alignCenter)
	y += 22 * s
	a.bigButton(cv, "again", "Deal Again", cx, y)
}

/* ------------------------------------------------------------ the about box */

type block struct {
	kind  string // dt, dd, p, h3, li, foot
	spans []span
}

func (a *App) aboutBlocks() []block {
	ink, dt := hex("#3a2412"), hex("#6b4a22")
	if a.aboutTab == 0 {
		it := func(t string) span { return span{Text: t, Color: ink} }
		return []block{
			{"dt", []span{{Text: "DEVELOPED & PUBLISHED BY", Bold: true, Color: dt}}}, {"dd", []span{{Text: "Ironite", Bold: true, Color: ink}}},
			{"dt", []span{{Text: "GAME DESIGN", Bold: true, Color: dt}}}, {"dd", []span{it("Ironite")}},
			{"dt", []span{{Text: "CARD ART & ILLUSTRATION", Bold: true, Color: dt}}}, {"dd", []span{it("Ironite. Every card, the prospector kings, dance hall queens, Mountie jacks and the card back are original artwork.")}},
			{"dt", []span{{Text: "MUSIC", Bold: true, Color: dt}}}, {"dd", []span{it("Ironite. Four original frontier tunes: Chilkoot Pass Reel, Dawson City Waltz, Bonanza Creek and Sourdough Hoedown, played on fiddle, harmonica, banjo, guitar and upright bass.")}},
			{"dt", []span{{Text: "SOUND EFFECTS", Bold: true, Color: dt}}}, {"dd", []span{it("Ironite")}},
			{"dt", []span{{Text: "PROGRAMMING", Bold: true, Color: dt}}}, {"dd", []span{it("Ironite")}},
			{"dt", []span{{Text: "SPECIAL THANKS", Bold: true, Color: dt}}}, {"dd", []span{it("The stampeders of 1896–1899, and everyone who ever played solitaire on a slow afternoon.")}},
			{"foot", []span{{Text: "© 2026 Ironite. All rights reserved.", Color: hex("#7a5a30")}}},
		}
	}
	var out []block
	for _, ln := range strings.Split(A.changes, "\n") {
		ln = strings.TrimRight(ln, "\r ")
		switch {
		case strings.HasPrefix(ln, "## "):
			v, date := ln[3:], ""
			if i := strings.Index(v, " - "); i >= 0 {
				v, date = v[:i], prettyDate(strings.TrimSpace(v[i+3:]))
			}
			v = strings.Trim(v, "[]")
			out = append(out, block{"h3", []span{{Text: "Version " + v, Bold: true, Color: hex("#7a2b16"), Size: 17 * a.s}, {Text: "  " + date, Color: hex("#7a5a30"), Size: 13 * a.s}}})
		case strings.HasPrefix(ln, "- "):
			out = append(out, block{"li", []span{{Text: ln[2:], Color: ink}}})
		}
	}
	return out
}

func prettyDate(d string) string {
	months := []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	if len(d) == 10 && d[4] == '-' && d[7] == '-' {
		m := int(d[5]-'0')*10 + int(d[6]-'0')
		day := int(d[8]-'0')*10 + int(d[9]-'0')
		if m >= 1 && m <= 12 {
			return months[m-1] + " " + itoa(day) + ", " + d[:4]
		}
	}
	return d
}

func (a *App) drawAbout(cv *image.RGBA) {
	s := a.s
	cw := 640 * s
	pw := cw + 80*s + 6*s
	viewH := math.Round(float64(a.H) * .42)
	brandH := 62*s + 44*s
	ph := 3*s + 30*s + 4*s + brandH + 18*s + 32*s*1.15 + 6*s + 16*s*1.15 + 18*s + a.buttonH(btnStyle{size: 14 * s, padY: 6 * s}) + 14*s + viewH + 18*s + a.buttonH(btnStyle{size: 14 * s, padY: 6 * s}) + 30*s + 3*s
	if ph > float64(a.H)-20*s { // small windows: shrink the scroll area
		viewH -= ph - (float64(a.H) - 20*s)
		ph = float64(a.H) - 20*s
	}
	px, py := math.Round((float64(a.W)-pw)/2), math.Round((float64(a.H)-ph)/2)
	a.panelBox(cv, px, py, pw, ph)
	a.addHit("panel", px, py, pw, ph)
	x0 := px + 3*s + 40*s
	cx := px + pw/2
	y := py + 3*s + 30*s + 4*s
	// brand box with the Ironite logo
	fillRRect(cv, x0, y, cw, brandH, 8*s, uni(hex("#003b2f")))
	fillRect(cv, image.Rect(int(x0+8*s), int(y), int(x0+cw-8*s), int(y+3*s)), col(0, 0, 0, .25))
	logo := a.logoAt(int(62 * s))
	blit(cv, logo, int(cx-float64(logo.Rect.Dx())/2), int(y+22*s))
	y += brandH + 18*s
	y += a.h1(cv, "KLONDIKE", 32, cx, y) + 6*s
	y += a.h2(cv, "GOLD RUSH SOLITAIRE · VERSION "+version, cx, y) + 18*s
	// tabs
	st := btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
	w0, w1 := a.buttonW("Credits", st), a.buttonW("Changelog", st)
	tx := cx - (w0+w1+8*s)/2
	s0, s1 := st, st
	s0.on, s1.on = a.aboutTab == 0, a.aboutTab == 1
	a.button(cv, "tab0", "Credits", tx, y, s0, false)
	a.button(cv, "tab1", "Changelog", tx+w0+8*s, y, s1, false)
	y += a.buttonH(st) + 14*s
	// scroll area
	view := image.Rect(int(x0), int(y), int(x0+cw), int(y+viewH))
	content := a.layoutAbout(cw - 16*s)
	maxScroll := math.Max(0, content.h-viewH)
	a.scrollY = math.Min(a.scrollY, maxScroll)
	sub := cv.SubImage(view).(*image.RGBA)
	content.draw(sub, x0, y-a.scrollY)
	if maxScroll > 0 { // scrollbar
		th := viewH * viewH / content.h
		ty := y + (viewH-th)*a.scrollY/maxScroll
		fillRRect(cv, x0+cw-6*s, ty, 5*s, th, 2.5*s, uni(col(107, 74, 34, .45)))
	}
	y += viewH + 18*s
	a.bigButtonSmall(cv, "close", "Back to the Table", cx, y)
}

func (a *App) bigButtonSmall(cv *image.RGBA, id, label string, cx, y float64) {
	st := btnStyle{size: 14 * a.s, padX: 12 * a.s, padY: 6 * a.s}
	w := a.buttonW(label, st)
	a.button(cv, id, label, cx-w/2, y, st, false)
}

type aboutLayout struct {
	h     float64
	items []func(dst *image.RGBA, x, y float64)
}

func (l aboutLayout) draw(dst *image.RGBA, x, y float64) {
	for _, f := range l.items {
		f(dst, x, y)
	}
}

func (a *App) layoutAbout(w float64) aboutLayout {
	s := a.s
	var L aboutLayout
	y := 0.0
	size := 15 * s
	lh := size * 1.5
	for _, b := range a.aboutBlocks() {
		b := b
		switch b.kind {
		case "dt":
			y += 10 * s
			yy := y
			L.items = append(L.items, func(d *image.RGBA, x, oy float64) {
				drawText(d, b.spans[0].Text, fontSpec{Size: 13 * s, Bold: true, Spacing: 1 * s}, b.spans[0].Color, x, oy+yy+13*s, alignLeft)
			})
			y += 13 * s * 1.5
		case "dd", "li":
			y += 2 * s
			yy, ind := y, 0.0
			if b.kind == "li" {
				ind = 20 * s
			}
			n := len(layoutSpans(b.spans, size, w-ind))
			L.items = append(L.items, func(d *image.RGBA, x, oy float64) {
				if b.kind == "li" {
					fillCircle(d, x+ind-10*s, oy+yy+lh/2, 2.6*s, uni(hex("#3a2412")))
				}
				drawParagraph(d, b.spans, size, lh, x+ind, oy+yy, w-ind, alignLeft)
			})
			y += float64(n) * lh
		case "h3":
			y += 14 * s
			yy := y
			L.items = append(L.items, func(d *image.RGBA, x, oy float64) {
				drawParagraph(d, b.spans, 17*s, 17*s*1.5, x, oy+yy, w, alignLeft)
			})
			y += 17*s*1.5 + 4*s
		case "foot":
			y += 18 * s
			yy := y
			L.items = append(L.items, func(d *image.RGBA, x, oy float64) {
				drawText(d, b.spans[0].Text, fontSpec{Size: 13 * s}, b.spans[0].Color, x+w/2, oy+yy+13*s, alignCenter)
			})
			y += 13*s*1.5 + 8*s
		}
	}
	L.h = y + 8*s
	return L
}
