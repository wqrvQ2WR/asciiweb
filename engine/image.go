package engine

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"sort"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// Thumb is a small RGB copy of an image; the original is dropped right away,
// which keeps memory use low.
type Thumb struct {
	OW, OH int // original size
	W, H   int
	Pix    []uint8 // RGB
}

const (
	thumbMaxW = 216
	thumbMaxH = 400
	maxPixels = 16_000_000
)

func decodeThumb(r io.Reader) (*Thumb, error) {
	data, err := io.ReadAll(io.LimitReader(r, 6<<20))
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, errors.New("image too large")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	data = nil
	return downscale(img), nil
}

func downscale(img image.Image) *Thumb {
	b := img.Bounds()
	ow, oh := b.Dx(), b.Dy()
	tw := ow
	if tw > thumbMaxW {
		tw = thumbMaxW
	}
	th := oh * tw / ow
	if th > thumbMaxH {
		th = thumbMaxH
		tw = ow * th / oh
	}
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	pix := make([]uint8, tw*th*3)
	for y := 0; y < th; y++ {
		y0 := b.Min.Y + y*oh/th
		y1 := b.Min.Y + (y+1)*oh/th
		if y1 <= y0 {
			y1 = y0 + 1
		}
		sy := (y1 - y0) / 3
		if sy < 1 {
			sy = 1
		}
		for x := 0; x < tw; x++ {
			x0 := b.Min.X + x*ow/tw
			x1 := b.Min.X + (x+1)*ow/tw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			sx := (x1 - x0) / 3
			if sx < 1 {
				sx = 1
			}
			var sr, sg, sb, n uint32
			for yy := y0; yy < y1; yy += sy {
				for xx := x0; xx < x1; xx += sx {
					r, g, bl, a := img.At(xx, yy).RGBA() // premultiplied; composite on white paper
					sr += (r + 0xffff - a) >> 8
					sg += (g + 0xffff - a) >> 8
					sb += (bl + 0xffff - a) >> 8
					n++
				}
			}
			i := (y*tw + x) * 3
			pix[i], pix[i+1], pix[i+2] = uint8(sr/n), uint8(sg/n), uint8(sb/n)
		}
	}
	return &Thumb{OW: ow, OH: oh, W: tw, H: th, Pix: pix}
}

const ramp = " .:-=+*#%@"

// art renders the thumbnail as cols columns of ASCII characters. t is the
// link target id carried by every cell (0 for none).
func (t *Thumb) art(cols int, color bool, target int32) []Line {
	if cols < 1 {
		cols = 1
	}
	rows := int(math.Round(float64(cols) * float64(t.H) / float64(t.W) * 0.5))
	if rows < 1 {
		rows = 1
	}
	if rows > 60 {
		rows = 60
	}
	type px struct{ r, g, b, l float64 }
	grid := make([]px, rows*cols)
	lums := make([]float64, 0, rows*cols)
	for ry := 0; ry < rows; ry++ {
		y0, y1 := ry*t.H/rows, (ry+1)*t.H/rows
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for cx := 0; cx < cols; cx++ {
			x0, x1 := cx*t.W/cols, (cx+1)*t.W/cols
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sr, sg, sb, n float64
			for y := y0; y < y1 && y < t.H; y++ {
				for x := x0; x < x1 && x < t.W; x++ {
					i := (y*t.W + x) * 3
					sr += float64(t.Pix[i])
					sg += float64(t.Pix[i+1])
					sb += float64(t.Pix[i+2])
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			p := px{r: sr / n, g: sg / n, b: sb / n}
			p.l = (0.299*p.r + 0.587*p.g + 0.114*p.b) / 255
			grid[ry*cols+cx] = p
			lums = append(lums, p.l)
		}
	}
	// A bright border means "paper": draw the dark ink instead of the light.
	invert := false
	{
		var sum, n float64
		for cx := 0; cx < cols; cx++ {
			sum += grid[cx].l + grid[(rows-1)*cols+cx].l
			n += 2
		}
		for ry := 1; ry < rows-1; ry++ {
			sum += grid[ry*cols].l + grid[ry*cols+cols-1].l
			n += 2
		}
		invert = sum/n > 0.8
	}
	if invert {
		lums = lums[:0]
		for i := range grid {
			grid[i].l = 1 - grid[i].l
			lums = append(lums, grid[i].l)
		}
	}
	// auto-level on the 2nd and 98th percentile
	sort.Float64s(lums)
	lo := lums[len(lums)*2/100]
	hi := lums[(len(lums)-1)*98/100]
	if hi-lo < 0.05 {
		lo, hi = 0, 1
	}
	out := make([]Line, rows)
	for ry := 0; ry < rows; ry++ {
		l := make(Line, cols)
		for cx := 0; cx < cols; cx++ {
			p := grid[ry*cols+cx]
			v := (p.l - lo) / (hi - lo)
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			ch := rune(ramp[int(v*float64(len(ramp)-1)+0.5)])
			st := Style{T: target}
			if color && ch != ' ' {
				if invert {
					st.FG = normColor(p.r, p.g, p.b)
				} else {
					st.FG = Rgb(boost(p.r), boost(p.g), boost(p.b))
				}
			}
			l[cx] = Cell{ch, st}
		}
		out[ry] = trimRight(l)
	}
	return out
}

func boost(v float64) uint8 {
	v = v*1.2 + 24
	if v > 255 {
		v = 255
	}
	return uint8(v)
}

// normColor keeps the hue but lifts the color to full brightness (for ink
// that was dark on paper).
func normColor(r, g, b float64) uint32 {
	m := math.Max(r, math.Max(g, b))
	if m < 40 {
		return Rgb(235, 235, 235)
	}
	k := 255 / m
	return Rgb(uint8(r*k), uint8(g*k), uint8(b*k))
}
