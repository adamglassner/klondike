// Drawing primitives. Klondike: Gold Rush Solitaire (c) 2026 Ironite.
//
// Every image the game draws into is an *image.RGBA whose bytes are stored in
// Windows' native B,G,R,A order (premultiplied), so finished frames go to the
// screen without any conversion. Colors are therefore always made with col(),
// and images loaded from files go through toBGRA().
package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

// col returns a premultiplied color in the canvas' BGRA byte order.
func col(r, g, b uint8, a float64) color.RGBA {
	a = clamp(a, 0, 1)
	return color.RGBA{R: uint8(float64(b)*a + .5), G: uint8(float64(g)*a + .5), B: uint8(float64(r)*a + .5), A: uint8(255*a + .5)}
}

func hex(s string) color.RGBA { return css(s) }

// css parses "#rgb", "#rrggbb" and "rgba(r, g, b, a)" / "rgb(r, g, b)".
func css(s string) color.RGBA {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		v, _ := strconv.ParseUint(h, 16, 32)
		return col(uint8(v>>16), uint8(v>>8), uint8(v), 1)
	}
	if i := strings.Index(s, "("); i >= 0 {
		parts := strings.Split(strings.TrimSuffix(s[i+1:], ")"), ",")
		f := func(k int, def float64) float64 {
			if k >= len(parts) {
				return def
			}
			v, _ := strconv.ParseFloat(strings.TrimSpace(parts[k]), 64)
			return v
		}
		return col(uint8(f(0, 0)), uint8(f(1, 0)), uint8(f(2, 0)), f(3, 1))
	}
	return col(0, 0, 0, 1)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// toBGRA converts any image to a premultiplied BGRA canvas image.
func toBGRA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	swapRB(dst)
	return dst
}

func swapRB(m *image.RGBA) {
	p := m.Pix
	for i := 0; i+3 < len(p); i += 4 {
		p[i], p[i+2] = p[i+2], p[i]
	}
}

// scaleImage resizes src to w x h. hq selects Catmull-Rom over bilinear.
func scaleImage(src *image.RGBA, w, h int, hq bool) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, max(1, w), max(1, h)))
	var k xdraw.Interpolator = xdraw.ApproxBiLinear
	if hq {
		k = xdraw.CatmullRom
	}
	k.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst
}

// blit composites src at (x, y) with source-over.
func blit(dst *image.RGBA, src *image.RGBA, x, y int) {
	r := src.Bounds().Sub(src.Bounds().Min).Add(image.Pt(x, y))
	draw.Draw(dst, r, src, src.Bounds().Min, draw.Over)
}

// blitScaled draws src into the rectangle (x, y, w, h) with source-over.
func blitScaled(dst *image.RGBA, src *image.RGBA, x, y, w, h float64) {
	r := image.Rect(int(math.Round(x)), int(math.Round(y)), int(math.Round(x+w)), int(math.Round(y+h)))
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return
	}
	xdraw.ApproxBiLinear.Scale(dst, r, src, src.Bounds(), xdraw.Over, nil)
}

// blitAlpha composites src at (x, y) with an extra opacity factor.
func blitAlpha(dst *image.RGBA, src *image.RGBA, x, y int, a float64) {
	if a >= .999 {
		blit(dst, src, x, y)
		return
	}
	r := src.Bounds().Sub(src.Bounds().Min).Add(image.Pt(x, y))
	m := image.NewUniform(color.Alpha{A: uint8(clamp(a, 0, 1) * 255)})
	draw.DrawMask(dst, r, src, src.Bounds().Min, m, image.Point{}, draw.Over)
}

func fillRect(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Over)
}

func fillRectSrc(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

/* ---------------------------------------------------------------- paths */

type path struct {
	r    *vector.Rasterizer
	x, y float64 // origin of the rasterizer in canvas space
}

func newPath(b image.Rectangle) *path {
	return &path{r: vector.NewRasterizer(max(1, b.Dx()), max(1, b.Dy())), x: float64(b.Min.X), y: float64(b.Min.Y)}
}
func (p *path) move(x, y float64) { p.r.MoveTo(float32(x-p.x), float32(y-p.y)) }
func (p *path) line(x, y float64) { p.r.LineTo(float32(x-p.x), float32(y-p.y)) }
func (p *path) close()            { p.r.ClosePath() }
func (p *path) quad(cx, cy, x, y float64) {
	p.r.QuadTo(float32(cx-p.x), float32(cy-p.y), float32(x-p.x), float32(y-p.y))
}

// arc adds an arc (angles in radians, clockwise in screen space when a1 > a0).
func (p *path) arc(cx, cy, rad, a0, a1 float64, first bool) {
	n := int(math.Max(8, math.Abs(a1-a0)*rad/3))
	for i := 0; i <= n; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(n)
		x, y := cx+math.Cos(a)*rad, cy+math.Sin(a)*rad
		if i == 0 && first {
			p.move(x, y)
		} else {
			p.line(x, y)
		}
	}
}

// rrect adds a rounded rectangle; reverse winds it the other way, so a
// reversed inner rectangle punches a hole (used for borders).
func (p *path) rrect(x, y, w, h, r float64, reverse bool) {
	r = math.Min(r, math.Min(w, h)/2)
	if !reverse {
		p.arc(x+w-r, y+r, r, -math.Pi/2, 0, true)
		p.arc(x+w-r, y+h-r, r, 0, math.Pi/2, false)
		p.arc(x+r, y+h-r, r, math.Pi/2, math.Pi, false)
		p.arc(x+r, y+r, r, math.Pi, 1.5*math.Pi, false)
	} else {
		p.arc(x+r, y+r, r, 1.5*math.Pi, math.Pi, true)
		p.arc(x+r, y+h-r, r, math.Pi, math.Pi/2, false)
		p.arc(x+w-r, y+h-r, r, math.Pi/2, 0, false)
		p.arc(x+w-r, y+r, r, 0, -math.Pi/2, false)
	}
	p.close()
}

// fill draws the path with src (a color via image.Uniform, or a gradient).
func (p *path) fill(dst *image.RGBA, src image.Image) {
	b := image.Rect(int(p.x), int(p.y), int(p.x)+p.r.Size().X, int(p.y)+p.r.Size().Y)
	if b.In(dst.Bounds()) {
		p.r.DrawOp = draw.Over
		p.r.Draw(dst, b, src, b.Min)
		return
	}
	if !b.Overlaps(dst.Bounds()) {
		return
	}
	// partly outside dst (a clipped sub-image): go through a mask, which clips
	m := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	p.r.DrawOp = draw.Src
	p.r.Draw(m, m.Bounds(), image.Opaque, image.Point{})
	draw.DrawMask(dst, b, src, b.Min, m, image.Point{}, draw.Over)
}

func uni(c color.RGBA) image.Image { return image.NewUniform(c) }

func rrBounds(x, y, w, h, grow float64) image.Rectangle {
	return image.Rect(int(math.Floor(x-grow)), int(math.Floor(y-grow)), int(math.Ceil(x+w+grow)), int(math.Ceil(y+h+grow)))
}

// fillRRect fills a rounded rectangle.
func fillRRect(dst *image.RGBA, x, y, w, h, r float64, src image.Image) {
	if w <= 0 || h <= 0 {
		return
	}
	p := newPath(rrBounds(x, y, w, h, 1))
	p.rrect(x, y, w, h, r, false)
	p.fill(dst, src)
}

// strokeRRect draws a rounded-rectangle border of width lw, inside the box.
func strokeRRect(dst *image.RGBA, x, y, w, h, r, lw float64, src image.Image) {
	if w <= 0 || h <= 0 {
		return
	}
	p := newPath(rrBounds(x, y, w, h, 1))
	p.rrect(x, y, w, h, r, false)
	p.rrect(x+lw, y+lw, w-2*lw, h-2*lw, math.Max(0, r-lw), true)
	p.fill(dst, src)
}

func fillCircle(dst *image.RGBA, cx, cy, r float64, src image.Image) {
	p := newPath(rrBounds(cx-r, cy-r, 2*r, 2*r, 1))
	p.arc(cx, cy, r, 0, 2*math.Pi, true)
	p.close()
	p.fill(dst, src)
}

func fillPoly(dst *image.RGBA, pts [][2]float64, src image.Image) {
	x0, y0, x1, y1 := pts[0][0], pts[0][1], pts[0][0], pts[0][1]
	for _, q := range pts {
		x0, y0, x1, y1 = math.Min(x0, q[0]), math.Min(y0, q[1]), math.Max(x1, q[0]), math.Max(y1, q[1])
	}
	p := newPath(rrBounds(x0, y0, x1-x0, y1-y0, 1))
	for i, q := range pts {
		if i == 0 {
			p.move(q[0], q[1])
		} else {
			p.line(q[0], q[1])
		}
	}
	p.close()
	p.fill(dst, src)
}

/* ------------------------------------------------------------ gradients */

type stop struct {
	at float64
	c  color.RGBA
}

func lerpC(a, b color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + .5) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), l(a.A, b.A)}
}

func stopsAt(s []stop, t float64) color.RGBA {
	if t <= s[0].at {
		return s[0].c
	}
	for i := 1; i < len(s); i++ {
		if t <= s[i].at {
			return lerpC(s[i-1].c, s[i].c, (t-s[i-1].at)/(s[i].at-s[i-1].at))
		}
	}
	return s[len(s)-1].c
}

// linGrad is a linear gradient usable as a draw source.
type linGrad struct {
	x0, y0, dx, dy, l2 float64
	s                  []stop
}

func newLin(x0, y0, x1, y1 float64, s ...stop) *linGrad {
	dx, dy := x1-x0, y1-y0
	return &linGrad{x0, y0, dx, dy, math.Max(1e-9, dx*dx+dy*dy), s}
}
func (g *linGrad) ColorModel() color.Model { return color.RGBAModel }
func (g *linGrad) Bounds() image.Rectangle {
	return image.Rect(-1e9, -1e9, 1e9, 1e9)
}
func (g *linGrad) At(x, y int) color.Color {
	t := ((float64(x)+.5-g.x0)*g.dx + (float64(y)+.5-g.y0)*g.dy) / g.l2
	return stopsAt(g.s, t)
}

// radGrad is a radial gradient between two concentric circles.
type radGrad struct {
	cx, cy, r0, r1 float64
	s              []stop
}

func newRad(cx, cy, r0, r1 float64, s ...stop) *radGrad { return &radGrad{cx, cy, r0, r1, s} }
func (g *radGrad) ColorModel() color.Model              { return color.RGBAModel }
func (g *radGrad) Bounds() image.Rectangle              { return image.Rect(-1e9, -1e9, 1e9, 1e9) }
func (g *radGrad) At(x, y int) color.Color {
	d := math.Hypot(float64(x)+.5-g.cx, float64(y)+.5-g.cy)
	return stopsAt(g.s, (d-g.r0)/math.Max(1e-9, g.r1-g.r0))
}

// fillGrad fills a rectangle from a gradient into a new image (fast path
// for big static layers).
func fillGrad(dst *image.RGBA, r image.Rectangle, g image.Image, op draw.Op) {
	draw.Draw(dst, r, g, r.Min, op)
}

/* --------------------------------------------------------------- shadow */

// blurAlpha box-blurs an alpha mask three times (close to a Gaussian).
func blurAlpha(m *image.Alpha, rad int) {
	if rad < 1 {
		return
	}
	w, h := m.Rect.Dx(), m.Rect.Dy()
	tmp := make([]uint8, len(m.Pix))
	for pass := 0; pass < 3; pass++ {
		for y := 0; y < h; y++ { // horizontal
			row := m.Pix[y*m.Stride : y*m.Stride+w]
			sum, n := 0, 2*rad+1
			for i := -rad; i <= rad; i++ {
				sum += int(row[min(max(i, 0), w-1)])
			}
			for x := 0; x < w; x++ {
				tmp[y*m.Stride+x] = uint8(sum / n)
				sum += int(row[min(x+rad+1, w-1)]) - int(row[max(x-rad, 0)])
			}
		}
		for x := 0; x < w; x++ { // vertical
			sum, n := 0, 2*rad+1
			at := func(y int) int { return int(tmp[min(max(y, 0), h-1)*m.Stride+x]) }
			for i := -rad; i <= rad; i++ {
				sum += at(i)
			}
			for y := 0; y < h; y++ {
				m.Pix[y*m.Stride+x] = uint8(sum / n)
				sum += at(y+rad+1) - at(y-rad)
			}
		}
	}
}

// shadowRRect returns a soft shadow image for a rounded rectangle of w x h,
// with the given blur radius, as a canvas image offset by -blur*2.
func shadowRRect(w, h, r float64, blur int, c color.RGBA) *image.RGBA {
	m := blur * 2
	W, H := int(math.Ceil(w))+2*m, int(math.Ceil(h))+2*m
	tmp := image.NewRGBA(image.Rect(0, 0, W, H))
	fillRRect(tmp, float64(m), float64(m), w, h, r, uni(col(255, 255, 255, 1)))
	a := image.NewAlpha(tmp.Bounds())
	for i := 0; i < W*H; i++ {
		a.Pix[i] = tmp.Pix[i*4+3]
	}
	blurAlpha(a, max(1, blur/2))
	out := image.NewRGBA(tmp.Bounds())
	draw.DrawMask(out, out.Bounds(), uni(c), image.Point{}, a, image.Point{}, draw.Src)
	return out
}
