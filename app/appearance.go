// Card backs and tables you unlock by winning, and four-colour suits.
// The variants are made from the original art at load time.
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"sync"
)

type look struct {
	name string
	wins int // wins needed to unlock
}

var cardBacks = []look{{"Klondike Red", 0}, {"Yukon Night", 3}, {"Spruce Green", 10}, {"Midnight Sable", 25}}
var tables = []look{{"Plank Table", 0}, {"Dark Walnut", 2}, {"Dawson Saloon", 5}, {"Riverboat Blue", 15}}

func unlocked(l look, wins int) bool { return wins >= l.wins }

var (
	variantMu  sync.Mutex
	backMaster = map[int]*image.RGBA{}
	tableArt   = map[int]*image.RGBA{}
	fourMaster = map[int]*image.RGBA{}
)

// hsv conversions on 0..1 floats.
func toHSV(r, g, b float64) (h, s, v float64) {
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	v = mx
	d := mx - mn
	if mx > 0 {
		s = d / mx
	}
	if d == 0 {
		return 0, s, v
	}
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return
}

func fromHSV(h, s, v float64) (r, g, b float64) {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return r + m, g + m, b + m
}

// backArt is the card back in colourway i (made once, at master size).
func backArt(i int) *image.RGBA {
	if i <= 0 || i >= len(cardBacks) {
		return A.master[sprBack]
	}
	variantMu.Lock()
	defer variantMu.Unlock()
	if m := backMaster[i]; m != nil {
		return m
	}
	src := A.master[sprBack]
	m := image.NewRGBA(src.Rect)
	copy(m.Pix, src.Pix)
	pad := float64(A.meta.Pad)
	cw, ch := float64(A.meta.CW), float64(A.meta.CW)*1.4
	cx, cy := pad+cw/2, pad+ch/2
	ox, oy := cw*.29+cw*.04, cw*.36+cw*.04 // leave the mountain vignette alone
	for y := 0; y < m.Rect.Dy(); y++ {
		for x := 0; x < m.Rect.Dx(); x++ {
			dx, dy := (float64(x)-cx)/ox, (float64(y)-cy)/oy
			if dx*dx+dy*dy < 1 {
				continue
			}
			o := y*m.Stride + x*4
			al := float64(m.Pix[o+3]) / 255
			if al == 0 {
				continue
			}
			// stored BGRA, premultiplied
			b, g, r := float64(m.Pix[o])/255/al, float64(m.Pix[o+1])/255/al, float64(m.Pix[o+2])/255/al
			h, s, v := toHSV(r, g, b)
			if s < .28 || (h > 24 && h < 330) {
				continue // gold and cream stay
			}
			switch i {
			case 1: // navy
				h, s = 220, s*.8
				v = math.Min(1, v*1.05)
			case 2: // spruce
				h, s, v = 152, s*.7, v*.92
			case 3: // sable
				h, s, v = 30, s*.12, v*.7
			}
			r, g, b = fromHSV(h, s, v)
			m.Pix[o], m.Pix[o+1], m.Pix[o+2] = uint8(b*al*255+.5), uint8(g*al*255+.5), uint8(r*al*255+.5)
		}
	}
	backMaster[i] = m
	return m
}

var (
	inkBlack = [3]float64{0x1e, 0x19, 0x14}
	inkRed   = [3]float64{0xa5, 0x22, 0x1b}
	inkGreen = [3]float64{0x1d, 0x6a, 0x2e}
	inkBlue  = [3]float64{0x1c, 0x4c, 0xa6}
)

// fourColour reports the replacement ink for a card in four-colour mode.
func fourColour(id int) (from, to [3]float64, ok bool) {
	switch suitOf(id) {
	case 3:
		return inkBlack, inkGreen, true
	case 2:
		return inkRed, inkBlue, true
	}
	return
}

func cssOf(c [3]float64) string {
	h := "0123456789abcdef"
	s := "#"
	for _, v := range c {
		n := int(v)
		s += string(h[n>>4]) + string(h[n&15])
	}
	return s
}

// faceArt is card id's face, recoloured for four-colour suits if asked.
// Picture cards keep their artwork; only their corner indexes change.
func faceArt(id int, four bool) *image.RGBA {
	from, to, ok := fourColour(id)
	if !four || !ok {
		return A.master[id]
	}
	variantMu.Lock()
	defer variantMu.Unlock()
	if m := fourMaster[id]; m != nil {
		return m
	}
	src := A.master[id]
	m := image.NewRGBA(src.Rect)
	copy(m.Pix, src.Pix)
	pad := float64(A.meta.Pad)
	cw, ch := float64(A.meta.CW), float64(A.meta.CW)*1.4
	court := rankOf(id) > 10
	cream := [3]float64{0xf1, 0xe6, 0xcc}
	lum := func(c [3]float64) float64 { return .3*c[0] + .59*c[1] + .11*c[2] }
	lf, lc := lum(from), lum(cream)
	for y := 0; y < m.Rect.Dy(); y++ {
		for x := 0; x < m.Rect.Dx(); x++ {
			if court {
				lx, ly := float64(x)-pad, float64(y)-pad
				inCorner := (lx < cw*.2 && ly < ch*.26) || (lx > cw*.8 && ly > ch*.74)
				if !inCorner {
					continue
				}
			}
			o := y*m.Stride + x*4
			al := float64(m.Pix[o+3])
			if al < 250 {
				continue
			}
			c := [3]float64{float64(m.Pix[o+2]), float64(m.Pix[o+1]), float64(m.Pix[o])}
			// how far from ink toward the card's cream this pixel is
			t := clamp((lum(c)-lf)/(lc-lf), 0, 1)
			if t > .72 { // the card's own tint and glow stay as they are
				continue
			}
			d := 0.0
			for k := 0; k < 3; k++ {
				e := c[k] - (from[k] + (cream[k]-from[k])*t)
				d += e * e
			}
			if d > 32*32 {
				continue
			}
			for k := 0; k < 3; k++ {
				c[k] = clamp(c[k]+(to[k]-from[k])*(1-t), 0, 255)
			}
			m.Pix[o+2], m.Pix[o+1], m.Pix[o] = uint8(c[0]+.5), uint8(c[1]+.5), uint8(c[2]+.5)
		}
	}
	fourMaster[id] = m
	return m
}

// tableSource is table i at full size (2560 x 1440).
func tableSource(i int) *image.RGBA {
	if i <= 0 || i >= len(tables) {
		return A.table
	}
	variantMu.Lock()
	defer variantMu.Unlock()
	if t := tableArt[i]; t != nil {
		return t
	}
	src := A.table
	w, h := src.Rect.Dx(), src.Rect.Dy()
	t := image.NewRGBA(src.Rect)
	switch i {
	case 1: // dark walnut: the same planks, deeper and cooler
		for p := 0; p < len(src.Pix); p += 4 {
			b, g, r := float64(src.Pix[p]), float64(src.Pix[p+1]), float64(src.Pix[p+2])
			l := .3*r + .59*g + .11*b
			r, g, b = l+(r-l)*.8, l+(g-l)*.8, l+(b-l)*.8
			t.Pix[p], t.Pix[p+1], t.Pix[p+2], t.Pix[p+3] = uint8(b*.52), uint8(g*.5), uint8(r*.56), 255
		}
	default: // baize
		base := color.RGBA{0x24, 0x5c, 0x37, 255} // saloon green
		if i == 3 {
			base = color.RGBA{0x1e, 0x3f, 0x66, 255} // riverboat blue
		}
		felt(t, w, h, base)
	}
	tableArt[i] = t
	return t
}

// felt paints a baize cloth: a lamp-lit middle, a fine woven grain and darker edges.
func felt(t *image.RGBA, w, h int, base color.RGBA) {
	rnd := rand.New(rand.NewSource(1898))
	cx, cy := float64(w)/2, float64(h)*.45
	rmax := math.Hypot(cx, float64(h)-cy)
	var wg sync.WaitGroup
	rows := make([]int64, h)
	for y := range rows {
		rows[y] = rnd.Int63()
	}
	for band := 0; band < 8; band++ {
		wg.Add(1)
		go func(band int) {
			defer wg.Done()
			for y := band; y < h; y += 8 {
				r := rand.New(rand.NewSource(rows[y]))
				for x := 0; x < w; x++ {
					d := math.Hypot(float64(x)-cx, float64(y)-cy) / rmax
					light := 1.18 - .55*d*d
					n := (r.Float64() - .5) * .09
					weave := .025 * math.Sin(float64(x+y)*1.9) * math.Sin(float64(x-y)*1.7)
					k := light * (1 + n + weave)
					o := y*t.Stride + x*4
					t.Pix[o] = uint8(clamp(float64(base.B)*k, 0, 255))
					t.Pix[o+1] = uint8(clamp(float64(base.G)*k, 0, 255))
					t.Pix[o+2] = uint8(clamp(float64(base.R)*k, 0, 255))
					t.Pix[o+3] = 255
				}
			}
		}(band)
	}
	wg.Wait()
}
