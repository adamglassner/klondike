// Text fallback: the embedded Lora typeface (SIL Open Font License).
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"image"
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type otRasterizer struct {
	mu        sync.Mutex
	reg, bold *opentype.Font
	faces     map[fontSpec]font.Face
}

func newOpenTypeRasterizer() rasterizer {
	r := &otRasterizer{faces: map[fontSpec]font.Face{}}
	if b, err := assetFS.ReadFile("assets/fonts/Lora-Regular.ttf"); err == nil {
		r.reg, _ = opentype.Parse(b)
	}
	if b, err := assetFS.ReadFile("assets/fonts/Lora-Bold.ttf"); err == nil {
		r.bold, _ = opentype.Parse(b)
	}
	return r
}

func (r *otRasterizer) face(f fontSpec) font.Face {
	k := fontSpec{Size: f.Size, Bold: f.Bold}
	if fc := r.faces[k]; fc != nil {
		return fc
	}
	ft := r.reg
	if f.Bold && r.bold != nil {
		ft = r.bold
	}
	fc, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: f.Size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil
	}
	r.faces[k] = fc
	return fc
}

func (r *otRasterizer) render(s string, f fontSpec) *textMask {
	r.mu.Lock()
	defer r.mu.Unlock()
	fc := r.face(f)
	if fc == nil {
		return &textMask{Mask: image.NewAlpha(image.Rect(0, 0, 1, 1))}
	}
	m := fc.Metrics()
	asc, desc := float64(m.Ascent)/64, float64(m.Descent)/64
	// measure with letter-spacing
	adv := 0.0
	for _, ch := range s {
		a, _ := fc.GlyphAdvance(ch)
		adv += float64(a)/64 + f.Spacing
	}
	padX := int(math.Ceil(f.Size * .3))
	w, h := int(math.Ceil(adv))+2*padX, int(math.Ceil(asc+desc))+2
	mask := image.NewAlpha(image.Rect(0, 0, max(1, w), max(1, h)))
	d := &font.Drawer{Dst: mask, Src: image.Opaque, Face: fc}
	x := float64(padX)
	base := int(math.Ceil(asc)) + 1
	for _, ch := range s {
		d.Dot = fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.I(base)}
		d.DrawString(string(ch))
		a, _ := fc.GlyphAdvance(ch)
		x += float64(a)/64 + f.Spacing
	}
	return &textMask{Mask: mask, OX: padX, Base: base, Advance: adv, Ascent: asc, Descent: desc}
}
