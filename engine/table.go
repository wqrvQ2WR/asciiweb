package engine

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type tcell struct {
	n    *html.Node
	span int
	th   bool
}

func (c *ctx) table(n *html.Node, st Style, w int) []Line {
	var rows [][]tcell
	var caption *html.Node
	var collect func(p *html.Node)
	collect = func(p *html.Node) {
		for ch := p.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != html.ElementNode || hidden(ch) {
				continue
			}
			switch ch.Data {
			case "tr":
				var row []tcell
				for td := ch.FirstChild; td != nil; td = td.NextSibling {
					if td.Type == html.ElementNode && (td.Data == "td" || td.Data == "th") && !hidden(td) {
						sp := attrInt(td, "colspan")
						if sp < 1 {
							sp = 1
						}
						if sp > 20 {
							sp = 20
						}
						row = append(row, tcell{td, sp, td.Data == "th"})
					}
				}
				if len(row) > 0 {
					rows = append(rows, row)
				}
			case "thead", "tbody", "tfoot":
				collect(ch)
			case "caption":
				caption = ch
			}
		}
	}
	collect(n)
	if len(rows) == 0 {
		return nil
	}
	ncols := 0
	for _, r := range rows {
		s := 0
		for _, c := range r {
			s += c.span
		}
		if s > ncols {
			ncols = s
		}
	}
	bordered := false
	if b := attr(n, "border"); b != "" {
		if v, err := strconv.Atoi(b); err != nil || v > 0 {
			bordered = true
		}
	}
	var out []Line
	if caption != nil {
		out = append(out, trimBlank(c.layout(caption, st, w, alignCenter))...)
	}
	sep, edge := 2, 0
	if bordered {
		sep, edge = 3, 4
	}
	avail := w - edge - sep*(ncols-1)
	if ncols == 1 || avail < ncols*2 || c.depth > 14 {
		// single column (or no room): stack the cells
		for _, r := range rows {
			for _, cell := range r {
				cs := st
				if cell.th {
					cs.Bold = true
				}
				al := alignLeft
				if a := alignOf(cell.n); a >= 0 {
					al = a
				}
				out = append(out, trimBlank(c.layout(cell.n, cs, w, al))...)
			}
		}
		return out
	}

	// measure natural and minimum widths of single-span cells
	nat := make([]int, ncols)
	minw := make([]int, ncols)
	for _, r := range rows {
		col := 0
		for _, cell := range r {
			if cell.span == 1 {
				t0 := len(c.targets)
				ls := c.layout(cell.n, st, 300, alignLeft)
				c.targets = c.targets[:t0]
				if m := maxWidth(ls); m > nat[col] {
					nat[col] = m
				}
				if m := longestWord(cell.n); m > minw[col] {
					minw[col] = m
				}
			}
			col += cell.span
		}
	}
	total, summin := 0, 0
	for i := range nat {
		if nat[i] < 1 {
			nat[i] = 1
		}
		if minw[i] > nat[i] {
			minw[i] = nat[i]
		}
		if minw[i] < 1 {
			minw[i] = 1
		}
		total += nat[i]
		summin += minw[i]
	}
	widths := make([]int, ncols)
	switch {
	case total <= avail:
		copy(widths, nat)
	case summin >= avail:
		for i := range widths {
			widths[i] = avail * minw[i] / summin
			if widths[i] < 1 {
				widths[i] = 1
			}
		}
	default:
		extra, need := avail-summin, total-summin
		for i := range widths {
			widths[i] = minw[i] + extra*(nat[i]-minw[i])/need
		}
	}

	bs := Style{FG: colRule}
	rule := func(l, m, r string) Line {
		var sb strings.Builder
		sb.WriteString(l)
		for i, cw := range widths {
			sb.WriteString(strings.Repeat("─", cw+2))
			if i < len(widths)-1 {
				sb.WriteString(m)
			}
		}
		sb.WriteString(r)
		return cells(sb.String(), bs)
	}
	if bordered {
		out = append(out, rule("┌", "┬", "┐"))
	}
	first := true
	for _, r := range rows {
		var parts [][]Line
		var pw []int
		col, height := 0, 0
		for _, cell := range r {
			cw := (cell.span - 1) * sep
			for k := col; k < col+cell.span && k < ncols; k++ {
				cw += widths[k]
			}
			cs := st
			if cell.th {
				cs.Bold = true
			}
			al := alignLeft
			if a := alignOf(cell.n); a >= 0 {
				al = a
			}
			ls := trimBlank(c.layout(cell.n, cs, cw, al))
			parts = append(parts, ls)
			pw = append(pw, cw)
			if len(ls) > height {
				height = len(ls)
			}
			col += cell.span
		}
		if height == 0 {
			continue
		}
		if bordered && !first {
			out = append(out, rule("├", "┼", "┤"))
		}
		first = false
		for y := 0; y < height; y++ {
			var l Line
			if bordered {
				l = append(l, cells("│ ", bs)...)
			}
			for i, ls := range parts {
				var cl Line
				if y < len(ls) {
					cl = ls[y]
				}
				l = append(l, cl...)
				if pad := pw[i] - len(cl); pad > 0 {
					l = append(l, spaces(pad, Style{})...)
				}
				if i < len(parts)-1 {
					if bordered {
						l = append(l, cells(" │ ", bs)...)
					} else {
						l = append(l, spaces(sep, Style{})...)
					}
				}
			}
			if bordered {
				l = append(l, cells(" │", bs)...)
			}
			out = append(out, trimRight(l))
		}
	}
	if bordered {
		out = append(out, rule("└", "┴", "┘"))
	}
	return out
}

func longestWord(n *html.Node) int {
	m := 1
	for _, w := range strings.Fields(textContent(n)) {
		cw := 0
		for _, r := range w {
			cw += RuneWidth(r)
		}
		if cw > m {
			m = cw
		}
	}
	return m
}
