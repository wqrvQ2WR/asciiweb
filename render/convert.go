package render

import (
	"image"
	"math"
	"sort"

	"asciiweb/engine"
)

const (
	rampSimple = " .:-=+*#%@"
	rampDetail = " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$"
)

type rgb struct{ r, g, b float64 }

func (p rgb) lum() float64 { return (0.299*p.r + 0.587*p.g + 0.114*p.b) / 255 }

// sample box-averages the image into a gw x gh grid.
func sample(img image.Image, gw, gh int) []rgb {
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	out := make([]rgb, gw*gh)
	for y := 0; y < gh; y++ {
		y0 := b.Min.Y + y*ih/gh
		y1 := b.Min.Y + (y+1)*ih/gh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		sy := (y1 - y0 + 2) / 3
		for x := 0; x < gw; x++ {
			x0 := b.Min.X + x*iw/gw
			x1 := b.Min.X + (x+1)*iw/gw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			sx := (x1 - x0 + 2) / 3
			var sr, sg, sb, n float64
			for yy := y0; yy < y1; yy += sy {
				for xx := x0; xx < x1; xx += sx {
					r, g, bl, _ := img.At(xx, yy).RGBA()
					sr += float64(r >> 8)
					sg += float64(g >> 8)
					sb += float64(bl >> 8)
					n++
				}
			}
			out[y*gw+x] = rgb{sr / n, sg / n, sb / n}
		}
	}
	return out
}

func contrast(l float64, c int) float64 {
	l = (l-0.5)*(1+0.3*float64(c)) + 0.5
	return math.Max(0, math.Min(1, l))
}

func fg(p rgb, color bool, ink bool) uint32 {
	if !color {
		return 0
	}
	if ink { // dark ink on paper: keep hue, full brightness
		m := math.Max(p.r, math.Max(p.g, p.b))
		if m < 40 {
			return engine.Rgb(235, 235, 235)
		}
		k := 255 / m
		return engine.Rgb(uint8(p.r*k), uint8(p.g*k), uint8(p.b*k))
	}
	f := func(v float64) uint8 { return uint8(math.Min(255, v*1.15+20)) }
	return engine.Rgb(f(p.r), f(p.g), f(p.b))
}

func pix(p rgb) uint32 { return engine.Rgb(uint8(p.r), uint8(p.g), uint8(p.b)) }

// Convert draws img on a cols x rows character grid.
func Convert(img image.Image, cols, rows int, s Settings, depth int) []engine.Line {
	color := depth != engine.NoColor && s.Color != "none"
	subX, subY := 1, 1
	switch s.Style {
	case "blocks":
		subY = 2
	case "braille":
		subX, subY = 2, 4
	}
	gw, gh := cols*subX, rows*subY
	g := sample(img, gw, gh)

	var mean float64
	for _, p := range g {
		mean += p.lum()
	}
	mean /= float64(len(g))
	invert := mean > 0.55
	switch s.Invert {
	case "on":
		invert = true
	case "off":
		invert = false
	}
	// "ink" brightness: dense characters where the page has content
	ink := func(p rgb) float64 {
		l := contrast(p.lum(), s.Contrast)
		if invert {
			l = 1 - l
		}
		return l
	}
	out := make([]engine.Line, rows)
	thr := 0.5
	if s.Style == "braille" {
		all := make([]float64, len(g))
		for i, p := range g {
			all[i] = ink(p)
		}
		thr = math.Min(0.9, bgLevel(all)+0.22)
	}
	switch s.Style {
	case "blocks":
		for y := 0; y < rows; y++ {
			l := make(engine.Line, cols)
			for x := 0; x < cols; x++ {
				top, bot := g[(2*y)*gw+x], g[(2*y+1)*gw+x]
				if !color {
					v := (ink(top) + ink(bot)) / 2
					l[x] = engine.Cell{R: []rune(" ░▒▓█")[int(v*4+0.5)]}
				} else {
					l[x] = engine.Cell{R: '▀', S: engine.Style{FG: pix(top), BG: pix(bot)}}
				}
			}
			out[y] = l
		}
	case "braille":
		bit := [4][2]uint8{{1, 8}, {2, 16}, {4, 32}, {64, 128}}
		for y := 0; y < rows; y++ {
			l := make(engine.Line, cols)
			for x := 0; x < cols; x++ {
				var bits uint8
				var sr, sg, sb, n float64
				for dy := 0; dy < 4; dy++ {
					for dx := 0; dx < 2; dx++ {
						p := g[(y*4+dy)*gw+x*2+dx]
						if ink(p) > thr {
							bits |= bit[dy][dx]
							sr, sg, sb, n = sr+p.r, sg+p.g, sb+p.b, n+1
						}
					}
				}
				if bits == 0 {
					l[x] = engine.Cell{R: ' '}
					continue
				}
				c := rgb{sr / n, sg / n, sb / n}
				l[x] = engine.Cell{R: rune(0x2800) + rune(bits), S: engine.Style{FG: fg(c, color, invert)}}
			}
			out[y] = l
		}
	default:
		ramp := rampSimple
		if s.Style == "detail" {
			ramp = rampDetail
		}
		lums := make([]float64, len(g))
		for i, p := range g {
			lums[i] = ink(p)
		}
		srt := append([]float64(nil), lums...)
		sort.Float64s(srt)
		lo := bgLevel(lums) // the page background becomes blank
		hi := srt[(len(srt)-1)*98/100]
		if hi-lo < 0.15 {
			hi = lo + 0.5
		}
		for y := 0; y < rows; y++ {
			l := make(engine.Line, cols)
			for x := 0; x < cols; x++ {
				v := (lums[y*gw+x] - lo) / (hi - lo)
				v = math.Max(0, math.Min(1, (v-0.1)/0.9)) // dead zone: near-background stays blank
				ch := rune(ramp[int(v*float64(len(ramp)-1)+0.5)])
				c := engine.Cell{R: ch}
				if ch != ' ' {
					c.S.FG = fg(g[y*gw+x], color, invert)
				}
				l[x] = c
			}
			out[y] = l
		}
	}
	if s.Bold {
		for _, l := range out {
			for i := range l {
				l[i].S.Bold = true
			}
		}
	}
	return out
}

// bgLevel returns the most common ink level, i.e. the page background.
func bgLevel(v []float64) float64 {
	var h [64]int
	for _, x := range v {
		i := int(x * 63.999)
		if i < 0 {
			i = 0
		}
		if i > 63 {
			i = 63
		}
		h[i]++
	}
	best := 0
	for i := range h {
		if h[i] > h[best] {
			best = i
		}
	}
	return (float64(best) + 0.5) / 64
}
