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
	case panelScores:
		a.drawVeil(cv)
		a.drawScores(cv)
	case panelOptions:
		a.drawVeil(cv)
		a.drawOptions(cv)
	}
	if a.panel == panelNone || a.panel == panelWin {
		a.drawToast(cv)
	}
	if a.menu != "" {
		a.drawMenu(cv)
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
	if a.g != nil && len(a.g.Waste) > 0 && a.g.canRecycle() {
		slot(a.stockX(), a.top0, sprSlotR)
	} else {
		slot(a.stockX(), a.top0, sprSlot)
	}
	slot(a.wasteX(), a.top0, sprSlot)
	for f := 0; f < 4; f++ {
		slot(a.foundX(f), a.top0, sprSlotA)
	}
	for t := 0; t < 7; t++ {
		slot(a.colX(t), a.tabY, sprSlot)
	}
	if a.g == nil {
		return
	}
	T := a.targets()
	var moving, dragged, held []target
	k := a.kb
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
		if k.held && t.pile == k.src && t.idx >= k.sidx {
			held = append(held, t)
			continue
		}
		if math.Abs(t.x-d.x)+math.Abs(t.y-d.y) > 2 {
			moving = append(moving, t)
		} else {
			a.paint(cv, t, false, 0)
		}
	}
	for _, t := range moving {
		a.paint(cv, t, false, 0)
	}
	if a.hintOn.on {
		a.drawHint(cv, T)
	}
	lift := -10 * a.s
	for _, t := range held {
		a.paint(cv, t, true, lift)
	}
	if k.on && a.panel == panelNone {
		a.drawCursor(cv, T, lift)
	}
	for _, t := range dragged {
		a.paint(cv, t, true, 0)
	}
	if a.cel != nil {
		a.drawCelebration(cv)
	}
}

// ring outlines a card-sized area (table coordinates), with a soft glow.
func (a *App) ring(cv *image.RGBA, x, y, w, h float64, c color.RGBA, width float64) {
	r := a.cw * .075
	glow := c
	glow.A /= 3
	glow.R, glow.G, glow.B = uint8(int(c.R)/3), uint8(int(c.G)/3), uint8(int(c.B)/3) // premultiplied
	gw := width * 2.6
	strokeRRect(cv, x-gw/2, y+a.bar-gw/2, w+gw, h+gw, r+gw/2, gw, uni(glow))
	strokeRRect(cv, x-width/2, y+a.bar-width/2, w+width, h+width, r+width/2, width, uni(c))
}

// pileRect is the area of pile p from card idx down (a slot if it's empty).
func (a *App) pileRect(T []target, p pileRef, idx int) (x, y, w, h float64) {
	x, y = a.pileX(p), a.top0
	if p.Kind == pTab {
		y = a.tabY
	}
	w, h = a.cw, a.ch
	first := true
	for _, t := range T {
		if t.pile != p || t.idx < idx {
			continue
		}
		if first {
			x, y, first = t.x, t.y, false
		}
		h = t.y + a.ch - y
		x = math.Max(x, t.x) // a fanned waste: the top card
	}
	if p.Kind == pWaste || p.Kind == pFound || p.Kind == pStock {
		for _, t := range T {
			if t.pile == p {
				x, y = t.x, t.y
			}
		}
		h = a.ch
	}
	return
}

func (a *App) drawHint(cv *image.RGBA, T []target) {
	m := a.hintOn.m
	t := a.now - a.hintOn.t0
	al := (.75 + .25*math.Sin(t*7)) * clamp((hintLife-t)/.4, 0, 1) * clamp(t/.15, 0, 1)
	w := 4.5 * a.s
	x, y, cw, ch := a.pileRect(T, m.src, m.idx)
	a.ring(cv, x, y, cw, ch, col(255, 214, 102, al), w)
	if m.src.Kind == pStock {
		return
	}
	dx, dy, dw, dh := a.pileRect(T, m.dst, 1<<20)
	if m.dst.Kind == pTab && len(a.g.Tab[m.dst.I]) > 0 {
		n := len(a.g.Tab[m.dst.I]) - 1
		dx, dy, dw, dh = a.pileRect(T, m.dst, n)
	}
	a.ring(cv, dx, dy, dw, dh, col(120, 214, 150, al), w)
}

func (a *App) drawCursor(cv *image.RGBA, T []target, lift float64) {
	k := a.kb
	idx := 0
	if k.at.Kind == pTab {
		idx = k.idx
	}
	x, y, w, h := a.pileRect(T, k.at, idx)
	if k.held && k.at == k.src {
		y += lift
	}
	a.ring(cv, x, y, w, h, hex("#ffd451"), 4*a.s)
	if k.held {
		hx, hy, hw, hh := a.pileRect(T, k.src, k.sidx)
		a.ring(cv, hx, hy+lift, hw, hh, col(120, 214, 150, 1), 2.5*a.s)
	}
}

// drawToast shows a short message at the bottom of the table.
func (a *App) drawToast(cv *image.RGBA) {
	t := a.now - a.toastAt
	if t < 0 || t > toastLife || a.toastText == "" {
		return
	}
	s := a.s
	al := clamp(t/.2, 0, 1) * clamp((toastLife-t)/.4, 0, 1)
	f := fontSpec{Size: 15 * s}
	w := measure(a.toastText, f) + 36*s
	h := 38 * s
	x := (float64(a.W) - w) / 2
	y := float64(a.H) - h - 34*s
	fillRRect(cv, x, y+2*s, w, h, h/2, uni(col(0, 0, 0, .3*al)))
	fillRRect(cv, x, y, w, h, h/2, uni(col(30, 16, 8, .9*al)))
	strokeRRect(cv, x, y, w, h, h/2, 1.5*s, uni(col(232, 198, 106, .8*al)))
	drawText(cv, a.toastText, f, col(241, 223, 184, al), x+w/2, y+h/2+15*s*.34, alignCenter)
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
func (a *App) paint(cv *image.RGBA, t target, shadow bool, dy float64) {
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
	x, y := d.x, d.y+a.bar+dy
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
	ak := st.size / (14 * s) // arrow scales with the button
	if arrow {
		w += 16 * s * ak
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
		tx -= 8 * s * ak
	}
	drawText(cv, label, fontSpec{Size: st.size}, ink, tx, y+h/2+st.size*.33, alignCenter)
	if arrow {
		ax, ay, as := x+w-st.padX-4*s*ak, y+h/2, s*ak
		fillPoly(cv, [][2]float64{{ax - 4.5*as, ay - 2.5*as}, {ax + 4.5*as, ay - 2.5*as}, {ax, ay + 3*as}}, uni(ink))
	}
	return w
}

func bright(c color.RGBA, f float64) color.RGBA {
	m := func(v uint8) uint8 { return uint8(math.Min(float64(c.A), float64(v)*f)) }
	return color.RGBA{m(c.R), m(c.G), m(c.B), c.A}
}

// tbLayout is how the toolbar fits the window. Narrow windows first lose the
// subtitle, then the controls shrink, and finally the stats move to a second
// row (and, narrowest of all, the title steps aside). On a touch screen the
// controls start larger.
type tbLayout struct {
	k       float64 // control size factor
	sub     bool    // show "GOLD RUSH SOLITAIRE"
	title   bool
	twoRows bool
	bar     float64
	row1    float64 // height of the first row
}

func (a *App) tbStyle(k float64) btnStyle {
	s := a.s * k
	return btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
}

func (a *App) tbTitleW(k float64, sub bool) float64 {
	s := a.s * k
	w := measure("KLONDIKE", fontSpec{Size: 22 * s, Bold: true, Spacing: 5 * s})
	if sub {
		w += 6*s + measure("GOLD RUSH SOLITAIRE", fontSpec{Size: 11 * s, Spacing: 2 * s})
	}
	return w + 12*s
}

const (
	tbButton = iota
	tbMenu   // a button with a drop-down arrow
	tbSplit  // the narrow arrow half of a split button
	tbSlider
)

type tbItem struct {
	id, label string
	kind      int
	widest    string // the widest label this control can show
	off       bool
}

func (a *App) tbItems() []tbItem {
	drawLabel := "Draw One"
	if a.sv.Draw == 3 {
		drawLabel = "Draw Three"
	}
	ml := "♪ Music"
	if !a.sv.Music {
		ml = "♪ Muted"
	}
	return []tbItem{
		{id: "new", label: "New Game"}, {id: "gamemenu", kind: tbSplit},
		{id: "undo", label: "Undo"}, {id: "redo", label: "Redo"}, {id: "hint", label: "Hint"},
		{id: "draw", label: drawLabel, kind: tbMenu, widest: "Draw Three"},
		{id: "music", label: ml, widest: "♪ Muted", off: !a.sv.Music},
		{id: "vol", kind: tbSlider},
		{id: "sfx", label: "Sounds", off: !a.sv.Sfx},
		{id: "options", label: "Options"}, {id: "scores", label: "Scores"}, {id: "about", label: "Credits"},
	}
}

// tbItemW is a control's width, and the gap before it.
func (a *App) tbItemW(it tbItem, k float64) (w, gap float64) {
	s := a.s * k
	st := a.tbStyle(k)
	gap = 8 * s
	switch it.kind {
	case tbSlider:
		return 80 * s, gap
	case tbSplit:
		return a.buttonW("", st) + 6*s, 2 * a.s
	case tbMenu:
		return math.Max(a.buttonW(it.label, st), a.buttonW(it.widest, st)) + 16*s, gap
	}
	w = a.buttonW(it.label, st)
	if it.widest != "" {
		w = math.Max(w, a.buttonW(it.widest, st))
	}
	return w, gap
}

func (a *App) tbControlsW(k float64) float64 {
	w := 0.0
	for _, it := range a.tbItems() {
		iw, g := a.tbItemW(it, k)
		w += iw + g
	}
	return w + 8*a.s*k
}

func (a *App) tbStats() [][2]string {
	moves, secs := 0, 0
	g := a.g
	if g != nil {
		moves, secs = g.Moves, int(g.Elapsed)
	}
	first := [2]string{"Score", "0"}
	if g != nil {
		first[1] = commas(g.Score)
	}
	if (g != nil && g.Vegas) || (g == nil && a.sv.Vegas) {
		first = [2]string{"Poke", dollars(a.pokeNow())}
	}
	return [][2]string{first, {"Moves", itoa(moves)}, {"Time", clock(secs)}, {"Won", itoa(a.sv.Won) + "/" + itoa(a.sv.Played)}}
}

// tbStatsW is the stats' width with room for numbers to grow.
func (a *App) tbStatsW(k float64) float64 {
	s := a.s * k
	f, fb := fontSpec{Size: 15 * s}, fontSpec{Size: 15 * s, Bold: true}
	w := 0.0
	for _, it := range [][2]string{{"Score", "88,888"}, {"Moves", "8888"}, {"Time", "88:88"}, {"Won", "888/888"}} {
		w += measure(it[0]+" ", f) + measure(it[1], fb) + 20*s
	}
	return w - 20*s
}

func (a *App) fitToolbar() tbLayout {
	s := a.s
	W := float64(a.W)
	edge := 14 * s
	big := 1.0
	ks := []float64{1, .92, .85, .78, .72, .66, .6}
	if a.touchUI() { // finger-sized: don't shrink as far
		big = 1.2
		ks = []float64{1.2, 1.1, 1, .92, .85}
	}
	for _, o := range []struct {
		k   float64
		sub bool
	}{{big, true}, {big, false}, {big * .92, false}, {big * .85, false}} {
		need := edge + a.tbTitleW(o.k, o.sub) + a.tbControlsW(o.k) + 20*s + a.tbStatsW(math.Min(o.k, 1)) + edge
		if need <= W {
			b := math.Round(50 * s * math.Max(1, o.k))
			return tbLayout{k: o.k, sub: o.sub, title: true, bar: b, row1: b}
		}
	}
	// two rows: title and controls on top, the stats underneath
	row2 := math.Round(30 * s)
	if a.touchUI() {
		row2 = math.Round(34 * s)
	}
	last := ks[len(ks)-1]
	for _, title := range []bool{true, false} {
		for _, k := range ks {
			need := 2*edge + a.tbControlsW(k)
			if title {
				need += a.tbTitleW(k, false)
			}
			if need <= W || (!title && k == last) {
				r1 := math.Round(50 * s * math.Max(k, .8))
				return tbLayout{k: k, title: title, twoRows: true, bar: r1 + row2, row1: r1}
			}
		}
	}
	return tbLayout{}
}

func (a *App) drawToolbar(cv *image.RGBA) {
	s := a.s
	L := a.tb
	ks := s * L.k
	ch := a.chromeC()
	blit(cv, ch.barBG, 0, 0)
	toolbar := a.panel == panelNone
	if !toolbar { // under a panel the toolbar can't be clicked
		a.noHits = true
		defer func() { a.noHits = false }()
	}
	mid := (L.row1 - 2*s) / 2
	if L.twoRows {
		mid = L.row1 / 2
	}
	x := 14 * s
	if L.title {
		// title: 22px bold, letter-spacing 5px, gold, text-shadow 0 1px 0 #000
		tf := fontSpec{Size: 22 * ks, Bold: true, Spacing: 5 * ks}
		base := mid + 22*ks*.35
		drawText(cv, "KLONDIKE", tf, col(232, 198, 106, .22), x, base, alignLeft) // soft glow
		x += drawTextShadow(cv, "KLONDIKE", tf, hex("#e8c66a"), hex("#000000"), x, base, 0, 1*s, alignLeft)
		if L.sub {
			x += 6 * ks
			x += drawText(cv, "GOLD RUSH SOLITAIRE", fontSpec{Size: 11 * ks, Spacing: 2 * ks}, hex("#c9a96a"), x, base, alignLeft)
		}
		x += 12 * ks
	} else {
		x -= 8 * ks // no title: start the buttons at the edge
	}
	st := a.tbStyle(L.k)
	bh := a.buttonH(st)
	by := math.Round(mid - bh/2)
	extra := 0.0
	if L.twoRows { // spread the controls across the row
		free := float64(a.W) - 14*s - x - a.tbControlsW(L.k)
		extra = math.Max(0, math.Min(free/11, 10*s))
	}
	for _, it := range a.tbItems() {
		w, gap := a.tbItemW(it, L.k)
		if it.kind != tbSplit {
			gap += extra
		}
		x += gap
		switch it.kind {
		case tbSlider:
			a.addHit("vol", x, by, w, bh)
			v := float64(a.sv.Vol) / 100
			ty := mid
			kr := 8 * ks
			fillRRect(cv, x, ty-2.5*ks, w, 5*ks, 2.5*ks, uni(col(255, 255, 255, .82)))
			fillRRect(cv, x, ty-2.5*ks, math.Max(5*ks, w*v), 5*ks, 2.5*ks, uni(hex("#c9a464")))
			kx := x + kr + (w-2*kr)*v
			fillCircle(cv, kx, ty, kr, uni(hex("#c9a464")))
			fillCircle(cv, kx, ty, kr-1.2*ks, uni(bright(hex("#c9a464"), map[bool]float64{true: 1.1, false: 1}[a.hover == "vol"])))
		case tbSplit:
			bs := st
			bs.minW = w
			a.button(cv, it.id, "", x, by, bs, false)
			ax, ay := x+w/2, by+bh/2
			ink := hex("#2b1a0c")
			fillPoly(cv, [][2]float64{{ax - 4.5*ks, ay - 2.5*ks}, {ax + 4.5*ks, ay - 2.5*ks}, {ax, ay + 3*ks}}, uni(ink))
			a.gmX, a.gmY = x-a.lastW-2*s, by+bh
		case tbMenu:
			bs := st
			bs.minW = w - 16*ks
			a.ddX, a.ddY, a.ddW = x, by+bh, w
			a.button(cv, it.id, it.label, x, by, bs, true)
		default:
			bs := st
			bs.off = it.off
			bs.minW = w
			a.button(cv, it.id, it.label, x, by, bs, false)
		}
		a.lastW = w
		x += w
	}
	// stats: Score (or Vegas poke), Moves, Time, Won
	items := a.tbStats()
	sk := math.Min(ks, s*1.0)
	f, fb := fontSpec{Size: 15 * sk}, fontSpec{Size: 15 * sk, Bold: true}
	if L.twoRows {
		f, fb = fontSpec{Size: 15 * s}, fontSpec{Size: 15 * s, Bold: true}
		// a faint rule between the rows
		fillRect(cv, image.Rect(int(14*s), int(L.row1), a.W-int(14*s), int(L.row1+math.Max(1, s))), col(0, 0, 0, .35))
		fillRect(cv, image.Rect(int(14*s), int(L.row1+math.Max(1, s)), a.W-int(14*s), int(L.row1+2*math.Max(1, s))), col(255, 230, 180, .08))
		base := L.row1 + (L.bar-L.row1-2*s)/2 + 15*s*.35
		total := 0.0
		for _, it := range items {
			total += measure(it[0]+" ", f) + measure(it[1], fb) + 28*s
		}
		x := (float64(a.W) - (total - 28*s)) / 2
		for _, it := range items {
			x += drawText(cv, it[0]+" ", f, hex("#f1dfb8"), x, base, alignLeft)
			x += drawText(cv, it[1], fb, hex("#e8c66a"), x, base, alignLeft) + 28*s
		}
		return
	}
	base := mid + 22*s*.35
	rx := float64(a.W) - 14*s
	for i := len(items) - 1; i >= 0; i-- {
		rx -= drawText(cv, items[i][1], fb, hex("#e8c66a"), rx, base, alignRight)
		rx -= drawText(cv, items[i][0]+" ", f, hex("#f1dfb8"), rx, base, alignRight)
		rx -= 20 * s
	}
}

type menuItem struct {
	id, label, key string
	check          bool
}

// drawMenu draws the open drop-down (Draw One/Three, or the game menu).
func (a *App) drawMenu(cv *image.RGBA) {
	s := a.s
	var items []menuItem
	x, y, minW := a.ddX, a.ddY, a.ddW
	if a.menu == "game" {
		x, y, minW = a.gmX, a.gmY, 0
		items = []menuItem{{"newgame", "New Game", "F2", false}, {"restart", "Restart This Deal", "", false}, {"daily", "Daily Claim", "", false}}
	} else {
		items = []menuItem{{"dd1", "Draw One", "", a.sv.Draw == 1}, {"dd3", "Draw Three", "", a.sv.Draw == 3}}
	}
	ih := 28 * s
	fs := 14 * s
	if a.touchUI() {
		ih, fs = 42*s, 16*s
	}
	f := fontSpec{Size: fs}
	w := minW
	for _, it := range items {
		lw := 30*s + measure(it.label, f) + 18*s
		if it.key != "" {
			lw += measure(it.key, f) + 24*s
		}
		w = math.Max(w, lw)
	}
	y += 2 * s
	h := float64(len(items))*ih + 6*s
	fillRRect(cv, x+2*s, y+3*s, w, h, 4*s, uni(col(0, 0, 0, .35)))
	fillRRect(cv, x, y, w, h, 4*s, uni(hex("#f7ecd2")))
	strokeRRect(cv, x, y, w, h, 4*s, 1*s, uni(hex("#6b4a22")))
	for i, it := range items {
		iy := y + 3*s + float64(i)*ih
		a.addHit(it.id, x, iy, w, ih)
		if a.hover == it.id {
			fillRect(cv, image.Rect(int(x+2*s), int(iy), int(x+w-2*s), int(iy+ih)), hex("#c9a464"))
		}
		base := iy + ih/2 + fs*.34
		if it.check {
			drawText(cv, "✓", f, hex("#2b1a0c"), x+10*s, base, alignLeft)
		}
		drawText(cv, it.label, f, hex("#2b1a0c"), x+30*s, base, alignLeft)
		if it.key != "" {
			drawText(cv, it.key, f, hex("#7a5a30"), x+w-12*s, base, alignRight)
		}
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
	hint := []string{"Drag cards, or click a card to send it where it fits.", "F2 new game · Ctrl+Z undo · H hint · F11 full screen"}
	if a.touchUI() {
		hint[0] = "Drag cards, or tap one to send it where it fits."
	}
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
	small := btnStyle{size: 14 * s, padX: 12 * s, padY: 6 * s}
	ph := 3*s + 34*s + 40*s*1.15 + 6*s + 16*s*1.15 + 18*s + 8*s + 2*lh + 22*s + a.buttonH(btnStyle{size: 18 * s, padY: 10 * s}) + 16*s + 2*13*s*1.25 + 18*s + 32*s + 34*s + 3*s
	if a.resumable != nil {
		ph += a.buttonH(small) + 12*s
	}
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
	if a.resumable != nil {
		y += a.bigButton(cv, "continue", "Continue Your Claim", cx, y) + 12*s
		bw := a.buttonW("Stake a New Claim", small)
		a.button(cv, "start", "Stake a New Claim", cx-bw/2, y, small, false)
		y += a.buttonH(small) + 16*s
	} else {
		y += a.bigButton(cv, "start", "Stake Your Claim", cx, y) + 16*s
	}
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
	cw = math.Max(cw, 560*s) // room for the nickname row
	pw := cw + 96*s + 6*s
	entryH := 22 * s
	if a.entry.open && !a.entry.saved {
		entryH = a.buttonH(btnStyle{size: 14 * s, padY: 6 * s}) + 4*s
	} else if a.entry.saved {
		entryH = 24 * s
	}
	ph := 3*s + 34*s + 40*s*1.15 + 6*s + 16*s*1.15 + 18*s + 8*s + float64(len(lines))*lh + 18*s + entryH + 22*s + a.buttonH(btnStyle{size: 18 * s, padY: 10 * s}) + 34*s + 3*s
	px, py := math.Round((float64(a.W)-pw)/2), math.Round((float64(a.H)-ph)/2)
	a.panelBox(cv, px, py, pw, ph)
	a.addHit("panel", px, py, pw, ph)
	cx := px + pw/2
	y := py + 3*s + 34*s
	y += a.h1(cv, "EUREKA!", 40, cx, y) + 6*s
	y += a.h2(cv, "YOU STRUCK GOLD", cx, y) + 18*s + 8*s
	y += drawParagraph(cv, a.winSpans, pf, lh, px+51*s, y, cw, alignCenter)
	y += 18 * s
	y += a.drawEntry(cv, cx, y)
	y += 22 * s
	st := btnStyle{size: 18 * s, padX: 28 * s, padY: 10 * s}
	w1, w2 := a.buttonW("Deal Again", st), a.buttonW("Leaderboard", st)
	bx := cx - (w1+12*s+w2)/2
	a.button(cv, "again", "Deal Again", bx, y, st, false)
	a.button(cv, "board", "Leaderboard", bx+w1+12*s, y, st, false)
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
