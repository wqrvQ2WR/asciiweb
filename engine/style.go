// Package engine turns HTML into a grid of styled terminal cells.
package engine

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

var rw = func() *runewidth.Condition {
	c := runewidth.NewCondition()
	c.EastAsianWidth = false
	return c
}()

// RuneWidth is the display width (0, 1 or 2) of r.
func RuneWidth(r rune) int { return rw.RuneWidth(r) }

// Style of one cell. FG==0 means default; otherwise RGB|0xRRGGBB.
// T is the target id (index+1) when the cell belongs to a link or control.
type Style struct {
	FG, BG                        uint32
	Bold, Italic, Under, Dim, Rev bool
	T                             int32
}

const RGB uint32 = 1 << 24

func Rgb(r, g, b uint8) uint32 { return RGB | uint32(r)<<16 | uint32(g)<<8 | uint32(b) }

// Palette (bright, saturated).
var (
	colLink  = Rgb(0x3D, 0xA9, 0xFF)
	colH1    = Rgb(0xFF, 0x4D, 0x9D)
	colH2    = Rgb(0x9B, 0x5C, 0xFF)
	colH3    = Rgb(0x3D, 0xA9, 0xFF)
	colCode  = Rgb(0x7C, 0xE3, 0xA1)
	colCtrl  = Rgb(0xFF, 0xD2, 0x4D)
	colQuote = Rgb(0x9A, 0xA0, 0xB5)
	colRule  = Rgb(0x6B, 0x72, 0x90)
)

// Cell is one terminal column. A double-width rune occupies two cells; the
// second has R==0.
type Cell struct {
	R rune
	S Style
}

type Line []Cell

func (l Line) String() string {
	var sb strings.Builder
	for _, c := range l {
		if c.R != 0 {
			sb.WriteRune(c.R)
		}
	}
	return sb.String()
}

// cells converts text to cells, dropping control and zero-width runes.
func cells(s string, st Style) Line {
	l := make(Line, 0, len(s))
	for _, r := range s {
		if r == 0xa0 || r == '\t' {
			r = ' '
		}
		if r < 32 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			continue
		}
		w := RuneWidth(r)
		if w == 0 {
			continue
		}
		l = append(l, Cell{r, st})
		if w == 2 {
			l = append(l, Cell{0, st})
		}
	}
	return l
}

func spaces(n int, st Style) Line {
	l := make(Line, n)
	for i := range l {
		l[i] = Cell{' ', st}
	}
	return l
}

func isBlank(l Line) bool {
	for _, c := range l {
		if c.R != ' ' && c.R != 0 {
			return false
		}
	}
	return true
}

func trimRight(l Line) Line {
	for len(l) > 0 {
		c := l[len(l)-1]
		if c.R == ' ' && c.S.T == 0 && !c.S.Rev {
			l = l[:len(l)-1]
		} else {
			break
		}
	}
	return l
}

func trimBlank(ls []Line) []Line {
	for len(ls) > 0 && isBlank(ls[0]) {
		ls = ls[1:]
	}
	for len(ls) > 0 && isBlank(ls[len(ls)-1]) {
		ls = ls[:len(ls)-1]
	}
	return ls
}

func maxWidth(ls []Line) int {
	m := 0
	for _, l := range ls {
		if len(trimRight(l)) > m {
			m = len(trimRight(l))
		}
	}
	return m
}

// Text converts a string to cells in style st.
func Text(s string, st Style) Line { return cells(s, st) }
