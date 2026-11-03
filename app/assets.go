// Embedded art and audio, exported from the original game by tools/export.
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"math"
	"regexp"
	"strconv"
	"sync"

	"golang.org/x/image/webp"
)

//go:embed assets
var assetFS embed.FS

// Sprite indexes: 0-51 card faces, then these.
const (
	sprBack = 52 + iota
	sprSlot
	sprSlotA
	sprSlotR
	sprLift
	sprCount
)

var spriteNames = func() []string {
	n := make([]string, sprCount)
	for i := 0; i < 52; i++ {
		n[i] = "c" + strconv.Itoa(i)
	}
	n[sprBack], n[sprSlot], n[sprSlotA], n[sprSlotR], n[sprLift] = "back", "slot_", "slotA", "slotR", "lift"
	return n
}()

type textOp struct {
	Text     string     `json:"text"`
	X        float64    `json:"x"`
	Y        float64    `json:"y"`
	Font     string     `json:"font"`
	Color    string     `json:"color"`
	Align    string     `json:"align"`
	Baseline string     `json:"baseline"`
	M        [6]float64 `json:"m"`
}

type artMeta struct {
	CW, CH, Pad, LiftMargin int
	Text                    map[string][]textOp
}

type art struct {
	meta    artMeta
	master  [sprCount]*image.RGBA // card art without text, at meta.CW
	table   *image.RGBA           // 2560 x 1440 plank table
	logo    *image.RGBA           // Ironite wordmark
	icon    *image.RGBA
	changes string
}

var A art

func loadArt() error {
	b, err := assetFS.ReadFile("assets/cards/cards.json")
	if err != nil {
		return err
	}
	var raw struct {
		CW, CH, Pad, LiftMargin int
		Text                    map[string][]textOp `json:"text"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	A.meta = artMeta{raw.CW, raw.CH, raw.Pad, raw.LiftMargin, raw.Text}
	var wg sync.WaitGroup
	errs := make([]error, sprCount)
	for i := 0; i < sprCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			data, err := assetFS.ReadFile("assets/cards/" + spriteNames[i] + ".webp")
			if err != nil {
				errs[i] = err
				return
			}
			im, err := webp.Decode(bytes.NewReader(data))
			if err != nil {
				errs[i] = err
				return
			}
			A.master[i] = toBGRA(im)
		}(i)
	}
	tb, err := assetFS.ReadFile("assets/table.jpg")
	if err == nil {
		if im, e := jpeg.Decode(bytes.NewReader(tb)); e == nil {
			A.table = toBGRA(im)
		}
	}
	if lb, e := assetFS.ReadFile("assets/ironite.png"); e == nil {
		if im, _, e := image.Decode(bytes.NewReader(lb)); e == nil {
			A.logo = toBGRA(im)
		}
	}
	if cb, e := assetFS.ReadFile("assets/CHANGELOG.md"); e == nil {
		A.changes = string(cb)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	if A.table == nil {
		A.table = image.NewRGBA(image.Rect(0, 0, 16, 16))
	}
	return nil
}

/* ------------------------------------------------------------ sprites */

// spriteSet holds every card sprite scaled for the current card width.
type spriteSet struct {
	cw   float64 // card width the set was built for
	k    float64 // scale from the master art
	pad  float64 // sprite margin (shadow) at this scale
	lift float64 // margin of the drag-shadow sprite
	img  [sprCount]*image.RGBA
	hq   bool
}

var fontRE = regexp.MustCompile(`(bold\s+)?(italic\s+)?([\d.]+)px`)

// buildSprites scales the card art to card width cw and draws each card's
// text on top (in Georgia on Windows).
func buildSprites(cw float64, hq bool) *spriteSet {
	k := cw / float64(A.meta.CW)
	s := &spriteSet{cw: cw, k: k, pad: float64(A.meta.Pad) * k, lift: float64(A.meta.LiftMargin) * k, hq: hq}
	var wg sync.WaitGroup
	for i := 0; i < sprCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := A.master[i]
			w := int(math.Round(float64(m.Rect.Dx()) * k))
			h := int(math.Round(float64(m.Rect.Dy()) * k))
			img := scaleImage(m, w, h, hq)
			for _, op := range A.meta.Text[spriteNames[i]] {
				drawTextOp(img, op, k)
			}
			s.img[i] = img
		}(i)
	}
	wg.Wait()
	return s
}

// drawTextOp replays one canvas fillText call recorded from the original
// game, at scale k: same font, color, alignment and transform.
func drawTextOp(dst *image.RGBA, op textOp, k float64) {
	fm := fontRE.FindStringSubmatch(op.Font)
	if fm == nil {
		return
	}
	size, _ := strconv.ParseFloat(fm[3], 64)
	a, b, c, d, e, f := op.M[0], op.M[1], op.M[2], op.M[3], op.M[4], op.M[5]
	sx, sy := math.Hypot(a, b), math.Hypot(c, d)
	px := k * sy // device pixels per local unit, vertically
	spec := fontSpec{Size: size * px, Bold: fm[1] != "", Italic: fm[2] != ""}
	tm := renderText(op.Text, spec)
	// local (pre-transform) rectangle of the mask
	wLocal := tm.Advance / px
	left := op.X
	switch op.Align {
	case "center":
		left -= wLocal / 2
	case "right", "end":
		left -= wLocal
	}
	baseLocal := op.Y
	if op.Baseline == "middle" {
		baseLocal += (tm.Ascent - tm.Descent) / 2 / px
	} else if op.Baseline == "top" {
		baseLocal += tm.Ascent / px
	}
	x0 := left - float64(tm.OX)/px
	y0 := baseLocal - float64(tm.Base)/px
	x1 := x0 + float64(tm.Mask.Rect.Dx())/px
	y1 := y0 + float64(tm.Mask.Rect.Dy())/px
	tr := func(x, y float64) (float64, float64) { return (a*x + c*y + e) * k, (b*x + d*y + f) * k }
	ax, ay := tr(x0, y0)
	bx, by := tr(x1, y1)
	flip := a < 0
	mask := transformMask(tm, sx/sy, flip)
	ox, oy := math.Min(ax, bx), math.Min(ay, by)
	r := mask.Bounds().Add(image.Pt(int(math.Round(ox)), int(math.Round(oy))))
	draw.DrawMask(dst, r, uni(css(op.Color)), image.Point{}, mask, image.Point{}, draw.Over)
}

// coverTable scales the plank table to cover w x h (like the original,
// which drew it to fill the window).
func coverTable(w, h int, hq bool) *image.RGBA {
	t := A.table
	s := math.Max(float64(w)/float64(t.Rect.Dx()), float64(h)/float64(t.Rect.Dy()))
	sw, sh := int(math.Ceil(float64(t.Rect.Dx())*s)), int(math.Ceil(float64(t.Rect.Dy())*s))
	big := scaleImage(t, sw, sh, hq)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), big, image.Pt((sw-w)/2, (sh-h)/2), draw.Src)
	return out
}
