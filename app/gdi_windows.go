//go:build windows

// Text drawn by Windows itself (GDI) in Georgia, so the game's lettering is
// exactly what the original showed. Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"image"
	"math"
	"sync"
	"syscall"
	"unsafe"
)

var (
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	pCreateCompatibleDC    = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection      = gdi32.NewProc("CreateDIBSection")
	pCreateFontW           = gdi32.NewProc("CreateFontW")
	pSelectObject          = gdi32.NewProc("SelectObject")
	pDeleteObject          = gdi32.NewProc("DeleteObject")
	pSetTextColor          = gdi32.NewProc("SetTextColor")
	pSetBkMode             = gdi32.NewProc("SetBkMode")
	pSetTextCharacterExtra = gdi32.NewProc("SetTextCharacterExtra")
	pTextOutW              = gdi32.NewProc("TextOutW")
	pGetTextExtentPoint32W = gdi32.NewProc("GetTextExtentPoint32W")
	pGetTextMetricsW       = gdi32.NewProc("GetTextMetricsW")
	pGetGlyphIndicesW      = gdi32.NewProc("GetGlyphIndicesW")
	pGdiFlush              = gdi32.NewProc("GdiFlush")
)

type bitmapInfoHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}

type textMetric struct {
	Height, Ascent, Descent, InternalLeading, ExternalLeading int32
	AveCharWidth, MaxCharWidth, Weight, Overhang              int32
	DigitizedAspectX, DigitizedAspectY                        int32
	FirstChar, LastChar, DefaultChar, BreakChar               uint16
	Italic, Underlined, StruckOut, PitchAndFamily, CharSet    uint8
}

type gdiFontKey struct {
	face   string
	px     int
	bold   bool
	italic bool
}

type gdiRasterizer struct {
	mu    sync.Mutex
	dc    uintptr
	fonts map[gdiFontKey]uintptr
}

func newGDIRasterizer() *gdiRasterizer {
	dc, _, _ := pCreateCompatibleDC.Call(0)
	if dc == 0 {
		return nil
	}
	pSetBkMode.Call(dc, 1)             // TRANSPARENT
	pSetTextColor.Call(dc, 0x00FFFFFF) // white on black: coverage
	r := &gdiRasterizer{dc: dc, fonts: map[gdiFontKey]uintptr{}}
	// Georgia must be there; if not, use the built-in typeface instead.
	if m := r.render("Ag", fontSpec{Size: 20}); m == nil || blank(m.Mask) {
		return nil
	}
	return r
}

func (r *gdiRasterizer) font(k gdiFontKey) uintptr {
	if f, ok := r.fonts[k]; ok {
		return f
	}
	w := 400
	if k.bold {
		w = 700
	}
	it := 0
	if k.italic {
		it = 1
	}
	face, _ := syscall.UTF16PtrFromString(k.face)
	f, _, _ := pCreateFontW.Call(uintptr(-k.px), 0, 0, 0, uintptr(w), uintptr(it), 0, 0,
		1 /*DEFAULT_CHARSET*/, 4 /*OUT_TT_PRECIS*/, 0, 4 /*ANTIALIASED_QUALITY*/, 0, uintptr(unsafe.Pointer(face)))
	if len(r.fonts) > 200 {
		for _, old := range r.fonts {
			pDeleteObject.Call(old)
		}
		r.fonts = map[gdiFontKey]uintptr{}
	}
	r.fonts[k] = f
	return f
}

type gdiRun struct {
	text []uint16
	font uintptr
	w    int
}

// render draws s in Georgia; characters Georgia lacks (★ ♪ ▾ ✓) come from
// Segoe UI Symbol, as the browser did.
func (r *gdiRasterizer) render(s string, f fontSpec) *textMask {
	r.mu.Lock()
	defer r.mu.Unlock()
	px := max(1, int(math.Round(f.Size)))
	main := r.font(gdiFontKey{"Georgia", px, f.Bold, f.Italic})
	sym := r.font(gdiFontKey{"Segoe UI Symbol", px, f.Bold, false})
	extra := int(math.Round(f.Spacing))
	pSetTextCharacterExtra.Call(r.dc, uintptr(extra))

	u, _ := syscall.UTF16FromString(s)
	u = u[:len(u)-1]
	var runs []gdiRun
	if len(u) > 0 {
		pSelectObject.Call(r.dc, main)
		idx := make([]uint16, len(u))
		pGetGlyphIndicesW.Call(r.dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&idx[0])), 1 /*GGI_MARK_NONEXISTING_GLYPHS*/)
		start := 0
		missing := func(i int) bool { return idx[i] == 0xFFFF && u[i] > 0x7F }
		for i := 1; i <= len(u); i++ {
			if i == len(u) || missing(i) != missing(start) {
				fnt := main
				if missing(start) {
					fnt = sym
				}
				runs = append(runs, gdiRun{text: u[start:i], font: fnt})
				start = i
			}
		}
	}
	var tm textMetric
	pSelectObject.Call(r.dc, main)
	pGetTextMetricsW.Call(r.dc, uintptr(unsafe.Pointer(&tm)))
	total := 0
	for i := range runs {
		var sz [2]int32
		pSelectObject.Call(r.dc, runs[i].font)
		pGetTextExtentPoint32W.Call(r.dc, uintptr(unsafe.Pointer(&runs[i].text[0])), uintptr(len(runs[i].text)), uintptr(unsafe.Pointer(&sz)))
		runs[i].w = int(sz[0])
		total += runs[i].w
	}
	padX := int(math.Ceil(f.Size*.3)) + 2
	padY := int(math.Ceil(f.Size * .15))
	w := max(1, total+2*padX)
	h := int(tm.Height) + 2*padY
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	if total > 0 {
		bi := bitmapInfoHeader{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
		var bits unsafe.Pointer
		bmp, _, _ := pCreateDIBSection.Call(r.dc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
		if bmp == 0 || bits == nil {
			return nil
		}
		old, _, _ := pSelectObject.Call(r.dc, bmp)
		x := padX
		for _, run := range runs {
			pSelectObject.Call(r.dc, run.font)
			// align every run on Georgia's baseline
			var rtm textMetric
			pGetTextMetricsW.Call(r.dc, uintptr(unsafe.Pointer(&rtm)))
			y := padY + int(tm.Ascent) - int(rtm.Ascent)
			pTextOutW.Call(r.dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&run.text[0])), uintptr(len(run.text)))
			x += run.w
		}
		pGdiFlush.Call()
		px := unsafe.Slice((*byte)(bits), w*h*4)
		for i := 0; i < w*h; i++ {
			b, g, rr := px[i*4], px[i*4+1], px[i*4+2]
			mask.Pix[i] = max(b, g, rr)
		}
		pSelectObject.Call(r.dc, old)
		pDeleteObject.Call(bmp)
	}
	return &textMask{Mask: mask, OX: padX, Base: padY + int(tm.Ascent), Advance: float64(total),
		Ascent: float64(tm.Ascent), Descent: float64(tm.Descent)}
}

func init() {
	if r := newGDIRasterizer(); r != nil {
		primary = r
	}
}
