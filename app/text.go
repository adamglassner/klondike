// Text. Klondike: Gold Rush Solitaire (c) 2026 Ironite.
//
// The original game was drawn in Georgia. On Windows, text is rasterized by
// Windows itself (GDI, see gdi_windows.go) in Georgia, so it looks exactly as
// before. Elsewhere (and if GDI ever fails) an embedded open-licensed serif
// (Lora) is used instead.
package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"sync"
)

type fontSpec struct {
	Size    float64 // pixels (CSS font-size)
	Bold    bool
	Italic  bool
	Spacing float64 // extra pixels after each character (CSS letter-spacing)
}

// textMask is rendered text as coverage. The pen starts at (OX, Base) inside
// Mask; Advance is the distance the pen moved.
type textMask struct {
	Mask            *image.Alpha
	OX, Base        int
	Advance         float64
	Ascent, Descent float64
}

type rasterizer interface {
	render(s string, f fontSpec) *textMask
}

var (
	textMu       sync.Mutex
	textCache    = map[textKey]*textMask{}
	primary      rasterizer // GDI on Windows, nil elsewhere
	fallback     rasterizer // embedded Lora
	fallbackOnce sync.Once
)

type textKey struct {
	s string
	f fontSpec
}

func renderText(s string, f fontSpec) *textMask {
	f.Size = math.Round(f.Size*4) / 4
	k := textKey{s, f}
	textMu.Lock()
	m := textCache[k]
	textMu.Unlock()
	if m != nil {
		return m
	}
	if primary != nil {
		m = primary.render(s, f)
		if m != nil && strings.TrimSpace(s) != "" && blank(m.Mask) {
			m = nil // GDI produced nothing: use the fallback
		}
	}
	if m == nil {
		fallbackOnce.Do(func() { fallback = newOpenTypeRasterizer() })
		m = fallback.render(s, f)
	}
	textMu.Lock()
	if len(textCache) > 4000 {
		textCache = map[textKey]*textMask{}
	}
	textCache[k] = m
	textMu.Unlock()
	return m
}

func blank(a *image.Alpha) bool {
	for _, v := range a.Pix {
		if v != 0 {
			return false
		}
	}
	return true
}

func measure(s string, f fontSpec) float64 { return renderText(s, f).Advance }

type align int

const (
	alignLeft align = iota
	alignCenter
	alignRight
)

// drawText draws s with its baseline at y. Returns the advance.
func drawText(dst *image.RGBA, s string, f fontSpec, c color.RGBA, x, y float64, al align) float64 {
	if s == "" {
		return 0
	}
	m := renderText(s, f)
	switch al {
	case alignCenter:
		x -= m.Advance / 2
	case alignRight:
		x -= m.Advance
	}
	px, py := int(math.Round(x))-m.OX, int(math.Round(y))-m.Base
	r := m.Mask.Bounds().Add(image.Pt(px, py))
	draw.DrawMask(dst, r, uni(c), image.Point{}, m.Mask, image.Point{}, draw.Over)
	return m.Advance
}

// drawTextShadow draws text with a 1-pixel-style drop shadow (CSS text-shadow).
func drawTextShadow(dst *image.RGBA, s string, f fontSpec, c, sh color.RGBA, x, y, dx, dy float64, al align) float64 {
	drawText(dst, s, f, sh, x+dx, y+dy, al)
	return drawText(dst, s, f, c, x, y, al)
}

/* ------------------------------------------------------- rich paragraphs */

// span is a run of text in one style.
type span struct {
	Text  string
	Bold  bool
	Color color.RGBA
	Size  float64 // 0 = paragraph size
}

type laidWord struct {
	x    float64
	sp   span
	text string
}
type laidLine struct {
	words []laidWord
	w     float64
}

// layoutSpans word-wraps spans to maxW. "\n" in a span forces a new line.
func layoutSpans(spans []span, size, maxW float64) []laidLine {
	var lines []laidLine
	cur := laidLine{}
	space := measure(" ", fontSpec{Size: size})
	newline := func() { lines = append(lines, cur); cur = laidLine{} }
	pending := false // a space is owed before the next word
	for _, sp := range spans {
		sz := sp.Size
		if sz == 0 {
			sz = size
		}
		f := fontSpec{Size: sz, Bold: sp.Bold}
		for li, part := range strings.Split(sp.Text, "\n") {
			if li > 0 {
				newline()
			}
			if li > 0 {
				pending = false
			}
			for _, w := range strings.Split(part, " ") {
				if w == "" {
					pending = true
					continue
				}
				lead := pending
				pending = true
				ww := measure(w, f)
				x := cur.w
				if lead && len(cur.words) > 0 {
					x += space
				}
				if x+ww > maxW && len(cur.words) > 0 {
					newline()
					x = 0
				}
				cur.words = append(cur.words, laidWord{x: x, sp: sp, text: w})
				cur.w = x + ww
			}
			pending = len(part) > 0 && part[len(part)-1] == ' '
		}
	}
	if len(cur.words) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// drawParagraph draws wrapped spans; returns the height used.
func drawParagraph(dst *image.RGBA, spans []span, size, lineH, x, y, maxW float64, al align) float64 {
	lines := layoutSpans(spans, size, maxW)
	for i, ln := range lines {
		ox := x
		switch al {
		case alignCenter:
			ox = x + (maxW-ln.w)/2
		case alignRight:
			ox = x + maxW - ln.w
		}
		base := y + float64(i)*lineH + lineH/2 + size*.35
		for _, w := range ln.words {
			sz := w.sp.Size
			if sz == 0 {
				sz = size
			}
			drawText(dst, w.text, fontSpec{Size: sz, Bold: w.sp.Bold}, w.sp.Color, ox+w.x, base, alignLeft)
		}
	}
	return float64(len(lines)) * lineH
}

// transformMask scales a mask horizontally by sx and optionally turns it 180°.
func transformMask(m *textMask, sx float64, flip bool) *image.Alpha {
	src := m.Mask
	if math.Abs(sx-1) > .01 {
		w := max(1, int(math.Round(float64(src.Rect.Dx())*sx)))
		dst := image.NewAlpha(image.Rect(0, 0, w, src.Rect.Dy()))
		for y := 0; y < src.Rect.Dy(); y++ {
			for x := 0; x < w; x++ {
				fx := (float64(x)+.5)/sx - .5
				x0 := int(math.Floor(fx))
				t := fx - float64(x0)
				a := float64(src.AlphaAt(x0, y).A)
				b := float64(src.AlphaAt(x0+1, y).A)
				dst.Pix[y*dst.Stride+x] = uint8(a + (b-a)*t + .5)
			}
		}
		src = dst
	}
	if flip {
		w, h := src.Rect.Dx(), src.Rect.Dy()
		dst := image.NewAlpha(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Pix[(h-1-y)*dst.Stride+(w-1-x)] = src.Pix[y*src.Stride+x]
			}
		}
		src = dst
	}
	return src
}
