// The game: layout, animation and input. Rendering is in draw.go.
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"image"
	"image/draw"
	"math"
	"math/rand"
	"time"
)

var version = "0.0.0-dev" // set with -ldflags "-X main.version=..."

type dispCard struct {
	x, y float64
	up   bool
	f0   float64 // when the current flip started (seconds), or -1
	wait float64 // don't move before this time (dealing)
}

type target struct {
	c    card
	x, y float64
	pile pileRef
	idx  int
}

type dragState struct {
	src    pileRef
	idx    int
	ids    map[int]bool
	cards  []card
	ox, oy float64 // pointer offset inside the first card
	sx, sy float64 // where the press started
	moved  bool
	offs   []float64
}

// The win celebration: the deck flies off the foundations into a turning
// wheel at the middle of the table while bursts of gold dust go off above it.
type spark struct {
	x, y, vx, vy float64
	born, life   float64
	size         int // index into the sparkle sprites
	phase        float64
}

type celebration struct {
	t0     float64
	from   [52][2]float64 // where each card started (its foundation)
	order  []int          // cards in wheel order
	delay  [52]float64
	sparks []spark
	next   float64 // next burst
	glow   [3]*image.RGBA
}

type panelKind int

const (
	panelNone panelKind = iota
	panelSplash
	panelWin
	panelAbout
	panelScores // records: claim register, daily claim, statistics, stamps
	panelOptions
)

type later struct {
	at float64
	fn func()
}

// App is the whole game. Coordinates are physical pixels; s is the UI scale
// (1 at 96 DPI). Table coordinates start below the toolbar (bar).
type App struct {
	W, H   int
	s      float64
	canvas *image.RGBA

	sv  saveData
	g   *game
	rnd *rand.Rand
	now float64

	bar, tw, th                          float64
	cw, ch, gap, left, top0, tabY, fanUp float64
	fanDown                              float64
	spr                                  *spriteSet
	sprBuiltAt, resizedAt                float64
	table                                *image.RGBA
	tableHQ                              bool

	disp        [52]dispCard
	drag        *dragState
	cel         *celebration
	nextAuto    float64
	started     bool
	panel       panelKind
	aboutTab    int
	scrollY     float64
	winSpans    []span
	prevPanel   panelKind // where Back to the Table returns from Credits/Scores
	entry       winEntry  // nickname box on the win screen
	entrySelAll bool      // the suggested name is selected (typing replaces it)
	caretAt     float64
	caretPhase  int
	highlight   int // board place to highlight (just saved), or 0
	timers      []later

	searching  bool // looking for a winnable deal
	dealCh     chan dealReady
	toastText  string
	toastAt    float64
	toastQueue []string
	hintOn     hintShow
	hintIdx    int
	kb         kbState
	resumable  *game // last session's unfinished game
	recTab     int   // records panel tab
	optScroll  float64
	longAt     float64 // touch: when a press that may become a long press began
	longX      float64
	longY      float64
	lastMoves  int // moves at the last resume save
	scrollDrag struct {
		on     bool
		y0, s0 float64
	}

	hover, press   string
	menu           string   // open dropdown: "draw" or "game"
	tb             tbLayout // how the toolbar fits this window
	ddX, ddY, ddW  float64
	gmX, gmY       float64 // where the game menu opens
	lastW          float64
	noHits         bool
	slider         bool // dragging the volume slider
	mouseX, mouseY float64

	dirty  bool
	hudSec int
	hits   []hit // clickable areas from the last frame
	chrome *chromeCache

	// platform hooks
	SetCursor        func(hand bool)
	ToggleFullscreen func()
	ShowKeyboard     func() // touch keyboard for the nickname box
	TouchScreen      bool   // the computer has a touch screen
	touchInput       bool   // the current pointer event came from a finger or pen
}

type hit struct {
	id string
	r  rectF
}

type rectF struct{ x, y, w, h float64 }

func (r rectF) in(x, y float64) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

func newApp(w, h int, scale float64) *App {
	a := &App{s: scale, rnd: rand.New(rand.NewSource(time.Now().UnixNano())), sv: loadSave(), panel: panelSplash, hudSec: -1,
		dealCh: make(chan dealReady, 1), toastAt: -10}
	a.SetCursor = func(bool) {}
	a.ToggleFullscreen = func() {}
	a.ShowKeyboard = func() {}
	a.resumable = loadResume()
	mix.mu.Lock()
	mix.musicOn, mix.vol, mix.sfxOn = a.sv.Music, float64(a.sv.Vol)/100, a.sv.Sfx
	mix.mu.Unlock()
	a.resize(w, h, scale)
	a.preview()
	return a
}

/* ---------------------------------------------------------------- layout */

func (a *App) colX(i int) float64 { return a.left + float64(i)*(a.cw+a.gap) }

// Where the stock, waste and foundations go: mirrored for left-handed play.
func (a *App) stockX() float64 {
	if a.sv.LeftHanded {
		return a.colX(6)
	}
	return a.colX(0)
}

func (a *App) wasteX() float64 {
	if a.sv.LeftHanded {
		return a.colX(5)
	}
	return a.colX(1)
}

func (a *App) foundX(f int) float64 {
	if a.sv.LeftHanded {
		return a.colX(f)
	}
	return a.colX(3 + f)
}

// touchUI reports whether controls are sized for fingers.
func (a *App) touchUI() bool { return a.sv.Controls == 1 || (a.sv.Controls == 0 && a.TouchScreen) }

// refreshLook rebuilds sprites and the table after an appearance option changes.
func (a *App) refreshLook() {
	a.spr = nil
	a.table = nil
	a.chrome = nil
	a.resize(a.W, a.H, a.s)
}

func (a *App) resize(w, h int, scale float64) {
	w, h = max(w, 320), max(h, 240)
	sizeChanged := w != a.W || h != a.H || scale != a.s
	a.W, a.H, a.s = w, h, scale
	if a.canvas == nil || sizeChanged {
		a.canvas = image.NewRGBA(image.Rect(0, 0, w, h))
		a.chrome = nil
	}
	s := a.s
	a.tb = a.fitToolbar()
	a.bar = a.tb.bar
	a.tw, a.th = float64(w), float64(h)-a.bar
	gr, rows := .14, 4.75
	if a.sv.LargeCards { // tighter gaps, and columns fan closer when long
		gr, rows = .07, 4.3
	}
	a.cw = math.Floor(math.Min((a.tw-40*s)/(7+6*gr), (a.th-30*s)/rows))
	a.cw = math.Max(a.cw, 30)
	a.ch = math.Round(a.cw * 1.4)
	a.gap = a.cw * gr
	a.left = (a.tw - (7*a.cw + 6*a.gap)) / 2
	a.top0 = math.Max(14*s, a.gap)
	if a.sv.LargeCards {
		a.top0 = math.Max(10*s, a.gap)
	}
	a.tabY = a.top0 + a.ch + a.gap*1.5
	a.fanUp, a.fanDown = a.ch*.27, a.ch*.12
	if a.spr == nil || math.Abs(a.spr.cw-a.cw) > .5 || a.spr.back != a.sv.Back || a.spr.four != a.sv.FourColor {
		a.spr = buildSprites(a.cw, a.spr == nil, a.sv.Back, a.sv.FourColor) // fast now, sharp once resizing settles
		a.sprBuiltAt = a.now
	}
	if a.table == nil || a.table.Rect.Dx() != w || a.table.Rect.Dy() != int(a.th) {
		a.table = coverTable(tableSource(a.sv.Table), w, int(a.th), false)
		a.tableHQ = false
	}
	a.resizedAt = a.now
	if a.g != nil {
		for _, t := range a.targets() {
			d := &a.disp[t.c.ID]
			d.x, d.y, d.wait = t.x, t.y, 0
		}
	}
	a.dirty = true
}

// targets is where every card belongs right now (same as the original).
func (a *App) targets() []target {
	g := a.g
	L := make([]target, 0, 52)
	for i, c := range g.Stock {
		o := float64(i/6) * .8 * a.s
		L = append(L, target{c, a.stockX() - o, a.top0 - o, pileRef{pStock, 0}, i})
	}
	n := len(g.Waste)
	base := max(0, n-3)
	for i, c := range g.Waste {
		x := a.wasteX()
		if g.Draw == 3 && i > base {
			dx := float64(i-base) * a.cw * .24
			if a.sv.LeftHanded {
				dx = -dx // fan toward the middle of the table
			}
			x += dx
		}
		L = append(L, target{c, x, a.top0, pileRef{pWaste, 0}, i})
	}
	for f, p := range g.Found {
		for i, c := range p {
			L = append(L, target{c, a.foundX(f), a.top0, pileRef{pFound, f}, i})
		}
	}
	avail := a.th - a.tabY - a.ch - 12*a.s
	for t, p := range g.Tab {
		need := 0.0
		for i := 0; i < len(p)-1; i++ {
			if p[i].Up {
				need += a.fanUp
			} else {
				need += a.fanDown
			}
		}
		k := 1.0
		if need > avail {
			k = avail / need
		}
		y := a.tabY
		for i, c := range p {
			L = append(L, target{c, a.colX(t), y, pileRef{pTab, t}, i})
			if c.Up {
				y += a.fanUp * k
			} else {
				y += a.fanDown * k
			}
		}
	}
	return L
}

/* ------------------------------------------------------------- the game */

func (a *App) after(delay float64, fn func()) { a.timers = append(a.timers, later{a.now + delay, fn}) }

func (a *App) bindSound(g *game) {
	g.sound = func(name string, delay float64) { mix.play(name, delay) }
}

// preview shows a dealt table behind the title screen.
func (a *App) preview() {
	a.g = newGame(a.sv.Draw, a.rnd)
	if a.resumable != nil {
		a.g = a.resumable
	}
	a.bindSound(a.g)
	for _, t := range a.targets() {
		a.disp[t.c.ID] = dispCard{x: t.x, y: t.y, up: t.c.Up, f0: -1}
	}
}

func (a *App) newGame() { a.deal(dealNew) }

func (a *App) afterMove() {
	g := a.g
	a.countGame()
	a.clearHint()
	if g.allFound() {
		a.win()
		return
	}
	if !g.Auto && g.readyToAutoFinish() {
		g.Auto = true
		a.nextAuto = a.now + .3
	}
	if g.Moves-a.lastMoves >= 10 { // keep the resume file fresh
		a.lastMoves = g.Moves
		a.saveResume()
	}
	a.dirty = true
}

func (a *App) win() {
	g := a.g
	g.Won, g.Auto, g.Collect = true, false, false
	a.kb.held = false
	secs := max(1, int(math.Round(g.Elapsed)))
	bonus := 0
	if !g.Vegas {
		bonus = winBonus(secs)
		g.Score += bonus
	}
	a.recordWin(secs)
	a.prepareEntry(g.Score, secs, g.Moves)
	mix.play("fanfare", 0)
	a.after(.5, a.startCelebration)
	gold := hex("#3a2412")
	note := hex("#7a5a30")
	sp := []span{
		{Text: "Your claim paid out in ", Color: gold}, {Text: clock(secs), Bold: true, Color: gold},
		{Text: " with ", Color: gold}, {Text: itoa(g.Moves), Bold: true, Color: gold}, {Text: " moves.\n", Color: gold},
	}
	if g.Vegas {
		sp = append(sp, span{Text: "This deal: ", Color: gold}, span{Text: dollars(g.Score), Bold: true, Color: gold},
			span{Text: "  ·  Your poke: ", Color: gold}, span{Text: dollars(a.sv.Poke), Bold: true, Color: gold})
	} else {
		sp = append(sp, span{Text: "Final score: ", Color: gold}, span{Text: commas(g.Score), Bold: true, Color: gold})
		if bonus > 0 {
			sp = append(sp, span{Text: " (time bonus " + commas(bonus) + ")", Color: note})
		}
	}
	line := "\nGames won: " + itoa(a.sv.Won) + " of " + itoa(a.sv.Played)
	if a.sv.Streak > 1 {
		line += "  ·  Streak: " + itoa(a.sv.Streak)
	}
	if g.Daily != "" {
		line = "\nDaily Claim for " + prettyShortDate(g.Daily) + " — struck!" + line
	}
	sp = append(sp, span{Text: line, Color: gold})
	a.winSpans = sp
	a.after(3.5, func() {
		if a.g.Won {
			a.panel = panelWin
			a.dirty = true
		}
	})
}

func (a *App) undo() {
	if a.g != nil && a.started && a.g.doUndo() {
		a.clearHint()
		a.kb.held = false
		a.dirty = true
	}
}

func (a *App) redo() {
	if a.g != nil && a.started && a.g.doRedo() {
		a.clearHint()
		a.kb.held = false
		a.dirty = true
	}
}

func (a *App) setDraw(n int) {
	a.sv.Draw = n
	a.sv.write()
	if a.g != nil && a.g.Moves == 0 && a.g.Daily == "" {
		a.g.Draw = n // takes effect now if no move was made yet
	}
	a.dirty = true
}

func (a *App) toggleMusic() {
	a.sv.Music = !a.sv.Music
	a.sv.write()
	mix.setMusic(a.sv.Music)
	a.dirty = true
}

func (a *App) toggleSfx() {
	a.sv.Sfx = !a.sv.Sfx
	a.sv.write()
	mix.setSfx(a.sv.Sfx)
	a.dirty = true
}

/* --------------------------------------------------------- celebration */

func (a *App) startCelebration() {
	c := &celebration{t0: a.now, next: a.now + .15}
	// kings first round the wheel, so each suit runs King..Ace around it
	for r := 12; r >= 0; r-- {
		for f := 0; f < 4; f++ {
			if r < len(a.g.Found[f]) {
				id := a.g.Found[f][r].ID
				c.order = append(c.order, id)
				c.from[id] = [2]float64{a.foundX(f), a.top0}
				c.delay[id] = float64(len(c.order)-1) * .035
			}
		}
	}
	for i, r := range []float64{4, 7, 12} {
		r *= a.s
		n := int(math.Ceil(r * 2))
		g := image.NewRGBA(image.Rect(0, 0, n, n))
		fillGrad(g, g.Rect, newRad(float64(n)/2, float64(n)/2, 0, r,
			stop{0, hex("#fff6d8")}, stop{.35, hex("#e8c66a")}, stop{1, col(201, 164, 100, 0)}), draw.Src)
		c.glow[i] = g
	}
	a.cel = c
}

// wheelPos is where card number i of n sits on the wheel at time t.
func (a *App) wheelPos(i, n int, t float64) (float64, float64) {
	cx, cy := a.tw/2, a.th/2
	ry := math.Max(a.ch*.3, a.th/2-a.ch/2-16*a.s)
	rx := math.Max(a.cw, math.Min(a.tw/2-a.cw*.75, ry*1.3))
	ang := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n) + t*.45
	bob := math.Sin(t*2.2+float64(i)*.5) * 3 * a.s
	return cx + math.Cos(ang)*rx - a.cw/2, cy + math.Sin(ang)*ry - a.ch/2 + bob
}

// celebrationCards gives each card's position this frame, back to front.
func (a *App) celebrationCards() []target {
	c := a.cel
	t := a.now - c.t0
	n := len(c.order)
	out := make([]target, 0, n)
	for i, id := range c.order {
		wx, wy := a.wheelPos(i, n, t)
		p := clamp((t-c.delay[id])/.85, 0, 1)
		e := 1 - math.Pow(1-p, 3)
		f := c.from[id]
		// a little hop on the way, so cards arc rather than slide
		hop := math.Sin(p*math.Pi) * a.ch * .35
		out = append(out, target{c: card{ID: id, Up: true}, x: f[0] + (wx-f[0])*e, y: math.Max(6*a.s, f[1]+(wy-f[1])*e-hop)})
	}
	// cards nearer the bottom of the wheel are in front
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].y < out[j-1].y; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func (a *App) celebrationStep() {
	c := a.cel
	s := a.s
	if a.now >= c.next {
		x := a.tw * (.15 + .7*a.rnd.Float64())
		y := a.th * (.12 + .3*a.rnd.Float64())
		n := 44 + a.rnd.Intn(20)
		for k := 0; k < n; k++ {
			ang := 2 * math.Pi * (float64(k) + a.rnd.Float64()) / float64(n)
			sp := (110 + a.rnd.Float64()*240) * s
			size := 0
			if r := a.rnd.Float64(); r > .9 {
				size = 2
			} else if r > .55 {
				size = 1
			}
			c.sparks = append(c.sparks, spark{x: x, y: y, vx: math.Cos(ang) * sp, vy: math.Sin(ang)*sp - 60*s,
				born: a.now, life: 1.3 + a.rnd.Float64()*.9, size: size, phase: a.rnd.Float64() * 6})
		}
		c.next = a.now + .3 + a.rnd.Float64()*.3
	}
}

func (a *App) moveSparks(dt float64) {
	c := a.cel
	live := c.sparks[:0]
	drag := math.Exp(-dt * 1.6)
	for _, p := range c.sparks {
		if a.now-p.born > p.life {
			continue
		}
		p.vx *= drag
		p.vy = p.vy*drag + 300*a.s*dt
		p.x += p.vx * dt
		p.y += p.vy * dt
		live = append(live, p)
	}
	c.sparks = live
}

/* --------------------------------------------------------------- update */

// update advances the game by dt seconds. It reports whether a new frame
// should be drawn.
func (a *App) update(dt float64) bool {
	real := math.Min(dt, 1) // the game clock runs on real time
	dt = math.Min(dt, .05)  // animations never jump
	a.now += dt
	select {
	case d := <-a.dealCh:
		a.applyDeal(d)
	default:
	}
	g := a.g
	if len(a.timers) > 0 {
		due := a.timers[:0:0]
		keep := a.timers[:0]
		for _, t := range a.timers {
			if a.now >= t.at {
				due = append(due, t)
			} else {
				keep = append(keep, t)
			}
		}
		a.timers = keep
		for _, t := range due {
			t.fn()
		}
	}
	// sharpen sprites and table once resizing has settled
	if a.spr != nil && !a.spr.hq && a.now-a.resizedAt > .15 {
		a.spr = buildSprites(a.cw, true, a.sv.Back, a.sv.FourColor)
		a.dirty = true
	}
	if !a.tableHQ && a.now-a.resizedAt > .15 {
		a.table = coverTable(tableSource(a.sv.Table), a.W, int(a.th), true)
		a.tableHQ = true
		a.dirty = true
	}
	if g.Started && !g.Won && a.started {
		g.Elapsed += real
	}
	if g.Auto && !g.Won && a.now >= a.nextAuto {
		if g.autoStep() {
			a.nextAuto = a.now + .095
			a.afterMove()
		} else {
			g.Auto, g.Collect = false, false
		}
	}
	moving := false
	k := 1 - math.Exp(-dt*15)
	for _, t := range a.targets() {
		d := &a.disp[t.c.ID]
		if a.drag != nil && a.drag.ids[t.c.ID] {
			if d.up != t.c.Up {
				d.up, d.f0 = t.c.Up, a.now
			}
			continue
		}
		if a.now >= d.wait {
			if d.x != t.x || d.y != t.y {
				d.x += (t.x - d.x) * k
				d.y += (t.y - d.y) * k
				if math.Abs(t.x-d.x) < .25 && math.Abs(t.y-d.y) < .25 {
					d.x, d.y = t.x, t.y
				}
				moving = true
			}
			if d.up != t.c.Up {
				d.up, d.f0 = t.c.Up, a.now
			}
		} else {
			moving = true
		}
		if d.f0 >= 0 && a.now-d.f0 < .25 {
			moving = true
		}
	}
	if a.cel != nil {
		a.celebrationStep()
		a.moveSparks(dt)
		moving = true
	}
	if g.Auto || len(a.timers) > 0 || a.searching {
		moving = true
	}
	if a.toastStep() {
		moving = true
	}
	if a.hintOn.on {
		if a.now-a.hintOn.t0 > hintLife {
			a.hintOn.on = false
			a.dirty = true
		} else {
			moving = true
		}
	}
	// touch: holding a finger still on the table collects cards
	if a.longAt > 0 && a.now-a.longAt > .6 {
		a.longAt = 0
		if a.drag == nil || !a.drag.moved {
			a.drag = nil
			a.collect()
		}
	}
	if sec := int(g.Elapsed); sec != a.hudSec {
		a.hudSec = sec
		a.dirty = true
	}
	if a.typing() { // blink the caret
		if ph := int((a.now - a.caretAt) / .53); ph != a.caretPhase {
			a.caretPhase = ph
			a.dirty = true
		}
	}
	r := a.dirty || moving
	a.dirty = false
	return r
}

/* ---------------------------------------------------------------- input */

func (a *App) hitAt(x, y float64) string {
	for i := len(a.hits) - 1; i >= 0; i-- {
		if a.hits[i].r.in(x, y) {
			return a.hits[i].id
		}
	}
	return ""
}

func (a *App) pointerMove(x, y float64) {
	a.mouseX, a.mouseY = x, y
	if a.longAt > 0 && math.Hypot(x-a.longX, y-a.longY) > 10*a.s {
		a.longAt = 0
	}
	if a.slider {
		a.setSliderFrom(x)
		return
	}
	if a.scrollDrag.on {
		a.scrollY = math.Max(0, a.scrollDrag.s0-(y-a.scrollDrag.y0))
		a.dirty = true
		return
	}
	if d := a.drag; d != nil {
		ty := y - a.bar
		if !d.moved && math.Hypot(x-d.sx, ty-d.sy) > 5*a.s {
			d.moved = true
		}
		if d.moved {
			for j, c := range d.cards {
				dc := &a.disp[c.ID]
				dc.x, dc.y = x-d.ox, ty-d.oy+d.offs[j]
			}
			a.dirty = true
		}
		return
	}
	h := a.hitAt(x, y)
	if a.touchInput { // fingers don't hover
		h = ""
	}
	if h != a.hover {
		a.hover = h
		a.dirty = true
	}
	a.SetCursor(h != "" && h != "panel" && h != "scroll")
}

func (a *App) pointerDown(x, y float64) {
	a.mouseX, a.mouseY = x, y
	a.kb.on, a.kb.held = false, false
	h := a.hitAt(x, y)
	a.press = h
	if a.touchInput {
		a.hover = h // show the press under the finger
	}
	if a.menu != "" {
		switch h {
		case "dd1":
			a.setDraw(1)
		case "dd3":
			a.setDraw(3)
		case "newgame", "restart", "daily":
			a.menu = ""
			a.click(h)
		}
		if h != "draw" && h != "gamemenu" {
			a.menu = ""
			a.press = ""
		}
		a.dirty = true
		if h != "draw" && h != "gamemenu" {
			return
		}
	}
	if h == "vol" {
		a.slider = true
		a.setSliderFrom(x)
		return
	}
	if h == "scroll" {
		a.scrollDrag.on, a.scrollDrag.y0, a.scrollDrag.s0 = true, y, a.scrollY
		return
	}
	if h != "" {
		a.dirty = true
		return
	}
	if a.panel != panelNone || y < a.bar {
		return
	}
	g := a.g
	if g.Won || g.Auto || a.searching {
		return
	}
	if a.touchInput {
		a.longAt, a.longX, a.longY = a.now, x, y
	}
	e := 0.0
	if a.touchInput {
		e = 6 * a.s // fingers are less exact
	}
	p := struct{ x, y float64 }{x, y - a.bar}
	in := func(px, py, w, h float64) bool {
		return p.x >= px-e && p.x <= px+w+e && p.y >= py-e && p.y <= py+h+e
	}
	if in(a.stockX()-4*a.s, a.top0-10*a.s, a.cw+8*a.s, a.ch+12*a.s) {
		a.longAt = 0
		if g.clickStock() {
			a.afterMove()
		}
		return
	}
	T := a.targets()
	for i := len(T) - 1; i >= 0; i-- {
		t := T[i]
		if t.pile.Kind == pStock || !in(t.x, t.y, a.cw, a.ch) {
			continue
		}
		if !t.c.Up {
			return
		}
		pl := *g.pile(t.pile)
		if (t.pile.Kind == pWaste || t.pile.Kind == pFound) && t.idx != len(pl)-1 {
			return
		}
		cards := append([]card(nil), pl[t.idx:]...)
		d := &dragState{src: t.pile, idx: t.idx, ids: map[int]bool{}, cards: cards, ox: p.x - t.x, oy: p.y - t.y, sx: p.x, sy: p.y}
		for j, c := range cards {
			d.ids[c.ID] = true
			if i+j < len(T) {
				d.offs = append(d.offs, T[i+j].y-t.y)
			} else {
				d.offs = append(d.offs, float64(j)*a.fanUp)
			}
		}
		a.drag = d
		return
	}
}

func (a *App) pointerUp(x, y float64) {
	a.longAt = 0
	if a.touchInput && a.hover != "" {
		defer func() { a.hover = ""; a.dirty = true }()
	}
	if a.slider {
		a.slider = false
		a.sv.write()
		return
	}
	if a.scrollDrag.on {
		a.scrollDrag.on = false
		return
	}
	if d := a.drag; d != nil {
		a.drag = nil
		a.dirty = true
		g := a.g
		if !d.moved {
			if g.smartMove(d.src, d.idx) {
				a.afterMove()
			}
			return
		}
		px, py := x, y-a.bar
		rx, ry := px-d.ox, py-d.oy
		id := d.cards[0].ID
		ov := func(tx, ty float64) float64 {
			return math.Max(0, math.Min(rx+a.cw, tx+a.cw)-math.Max(rx, tx)) * math.Max(0, math.Min(ry+a.ch, ty+a.ch)-math.Max(ry, ty))
		}
		var best *pileRef
		ba := 0.0
		if len(d.cards) == 1 {
			for f := 0; f < 4; f++ {
				if d.src.Kind == pFound && d.src.I == f {
					continue
				}
				if ar := ov(a.foundX(f), a.top0); ar > ba && g.canFound(id, f) {
					ba, best = ar, &pileRef{pFound, f}
				}
			}
		}
		T := a.targets()
		for t := 0; t < 7; t++ {
			if d.src.Kind == pTab && d.src.I == t {
				continue
			}
			ty := a.tabY
			if n := len(g.Tab[t]); n > 0 {
				for _, q := range T {
					if q.pile.Kind == pTab && q.pile.I == t && q.idx == n-1 {
						ty = q.y
					}
				}
			}
			ar := math.Max(ov(a.colX(t), ty), ov(a.colX(t), ty+a.fanUp*.5))
			if ar > ba && g.canTab(id, t) {
				ba, best = ar, &pileRef{pTab, t}
			}
		}
		if best != nil {
			g.move(d.src, d.idx, *best, true)
			a.afterMove()
		}
		return
	}
	h := a.hitAt(x, y)
	pressed := a.press
	a.press = ""
	a.dirty = true
	if h == "" || h != pressed {
		return
	}
	a.click(h)
}

// pointerRight is a right click: collect everything that can go up.
func (a *App) pointerRight(x, y float64) {
	if a.panel == panelNone && a.menu == "" && y >= a.bar {
		a.collect()
	}
}

func (a *App) click(id string) {
	if len(id) > 4 && id[:4] == "opt:" {
		a.option(id[4:])
		a.dirty = true
		return
	}
	switch id {
	case "new", "newgame":
		if a.started {
			a.newGame()
		} else {
			a.start()
		}
	case "gamemenu":
		a.toggleMenu("game")
	case "restart":
		if a.started && a.g != nil {
			a.deal(dealRestart)
		}
	case "daily", "playdaily":
		a.panel = panelNone
		a.started = true
		a.resumable = nil
		a.deal(dealDaily)
	case "undo":
		a.undo()
	case "redo":
		a.redo()
	case "hint":
		a.hint()
	case "draw":
		a.toggleMenu("draw")
	case "music":
		a.toggleMusic()
	case "sfx":
		a.toggleSfx()
	case "about":
		a.prevPanel = a.panel
		a.panel, a.aboutTab, a.scrollY = panelAbout, 0, 0
	case "options":
		a.prevPanel = a.panel
		a.panel, a.scrollY = panelOptions, 0
	case "scores":
		a.prevPanel, a.highlight = a.panel, 0
		a.panel, a.recTab, a.scrollY = panelScores, 0, 0
	case "board":
		a.prevPanel = a.panel
		a.panel, a.recTab, a.scrollY = panelScores, 0, 0
	case "rtab0", "rtab1", "rtab2", "rtab3":
		a.recTab, a.scrollY = int(id[4]-'0'), 0
	case "savescore":
		a.saveEntry()
	case "name":
		a.entrySelAll = false
		a.caretAt = a.now
		if a.touchInput {
			a.ShowKeyboard()
		}
	case "continue":
		a.resumeGame()
	case "start":
		a.start()
	case "again":
		a.newGame()
	case "close":
		a.panel = panelNone
		if a.prevPanel == panelWin && a.g.Won {
			a.panel = panelWin
		}
		if !a.started {
			a.panel = panelSplash
		}
	case "tab0", "tab1":
		a.aboutTab = int(id[3] - '0')
		a.scrollY = 0
	}
	a.dirty = true
}

func (a *App) toggleMenu(m string) {
	if a.menu == m {
		a.menu = ""
	} else {
		a.menu = m
	}
}

func (a *App) start() {
	a.panel = panelNone
	a.started = true
	a.resumable = nil
	a.newGame()
}

func (a *App) setSliderFrom(x float64) {
	for _, h := range a.hits {
		if h.id == "vol" {
			v := clamp((x-h.r.x)/h.r.w, 0, 1)
			a.sv.Vol = int(math.Round(v * 100))
			mix.setVol(float64(a.sv.Vol) / 100)
			a.dirty = true
		}
	}
}

func (a *App) wheel(delta float64) {
	if a.panel == panelAbout || a.panel == panelScores || a.panel == panelOptions {
		a.scrollY = math.Max(0, a.scrollY-delta*40*a.s)
		a.dirty = true
	}
}

// Keys the game understands.
const (
	keyF2 = iota + 1
	keyF11
	keyEscape
	keyUndo // Ctrl+Z
	keyRedo // Ctrl+Y
	keyLeft
	keyRight
	keyUp
	keyDown
	keyEnter
	keySpace
	keyHint    // H
	keyCollect // C
)

func (a *App) key(k int) {
	defer func() { a.dirty = true }()
	if a.typing() && k != keyF2 && k != keyF11 {
		return // the nickname box has the keyboard
	}
	switch k {
	case keyF2:
		if a.started {
			a.newGame()
		}
	case keyUndo:
		a.undo()
	case keyRedo:
		a.redo()
	case keyF11:
		a.ToggleFullscreen()
	case keyEscape:
		switch {
		case a.menu != "":
			a.menu = ""
		case a.kb.held:
			a.kb.held = false
		case a.panel == panelAbout || a.panel == panelScores || a.panel == panelOptions:
			a.click("close")
		default:
			a.kb.on = false
		}
	case keyEnter:
		switch a.panel {
		case panelSplash:
			if a.resumable != nil {
				a.resumeGame()
			} else {
				a.start()
			}
		case panelWin:
			a.newGame()
		case panelNone:
			a.kbSelect()
		}
	case keySpace:
		if a.panel == panelNone {
			a.kbDraw()
		}
	case keyHint:
		if a.panel == panelNone {
			a.hint()
		}
	case keyCollect:
		if a.panel == panelNone {
			a.collect()
		}
	case keyLeft, keyRight, keyUp, keyDown:
		if a.panel != panelNone {
			if k == keyUp || k == keyDown {
				d := 1.0
				if k == keyDown {
					d = -1
				}
				a.wheel(d)
			}
			return
		}
		if !a.started || a.g.Won {
			return
		}
		switch k {
		case keyLeft:
			a.kbMove(-1, 0)
		case keyRight:
			a.kbMove(1, 0)
		case keyUp:
			a.kbMove(0, -1)
		case keyDown:
			a.kbMove(0, 1)
		}
	}
}

func (a *App) saveWindow(w, h int, maximized bool) {
	a.sv.WinW, a.sv.WinH, a.sv.Maximized = w, h, maximized
	a.sv.write()
}

/* -------------------------------------------------------------- helpers */

func clock(secs int) string {
	s := secs % 60
	ss := itoa(s)
	if s < 10 {
		ss = "0" + ss
	}
	return itoa(secs/60) + ":" + ss
}

func commas(n int) string {
	s := itoa(n)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg, s = true, s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}
