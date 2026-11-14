package main

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"asciiweb/engine"
)

type htmlNode = html.Node

func attrOf(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}

// gradientBar draws the status line: pink to purple to sky blue behind white text.
func gradientBar(left, right string, cols, depth int) string {
	rs := []rune(right)
	rw := 0
	for _, r := range rs {
		rw += engine.RuneWidth(r)
	}
	avail := cols - rw - 3
	if avail < 4 {
		avail = cols - 1
		rs, rw = nil, 0
	}
	// clip left to avail cells
	var l []rune
	w := 0
	for _, r := range left {
		cw := engine.RuneWidth(r)
		if cw == 0 {
			continue
		}
		if w+cw > avail {
			l = append(l, '…')
			w++
			break
		}
		l = append(l, r)
		w += cw
	}
	cells := []rune(" ")
	cells = append(cells, l...)
	used := 1 + w
	gap := cols - used - rw - 1
	if gap < 0 {
		gap = 0
	}
	cells = append(cells, []rune(strings.Repeat(" ", gap))...)
	cells = append(cells, rs...)
	cells = append(cells, ' ')
	// color each rune by its column
	var sb strings.Builder
	col := 0
	for _, r := range cells {
		if depth == engine.NoColor {
			if col == 0 {
				sb.WriteString("\x1b[7m")
			}
		} else if depth == engine.TrueColor {
			cr, cg, cb := grad(float64(col) / float64(max(cols-1, 1)))
			fmt.Fprintf(&sb, "\x1b[1;38;2;255;255;255;48;2;%d;%d;%dm", cr, cg, cb)
		} else if col == 0 {
			sb.WriteString("\x1b[1;97;48;5;99m")
		}
		sb.WriteRune(r)
		col += engine.RuneWidth(r)
	}
	return sb.String()
}

func grad(t float64) (int, int, int) {
	stops := [][3]float64{{0xE0, 0x2F, 0x82}, {0x7F, 0x45, 0xE0}, {0x1F, 0x86, 0xE0}}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	seg := t * 2
	i := int(seg)
	if i > 1 {
		i = 1
	}
	f := seg - float64(i)
	a, b := stops[i], stops[i+1]
	return int(a[0] + (b[0]-a[0])*f), int(a[1] + (b[1]-a[1])*f), int(a[2] + (b[2]-a[2])*f)
}
