package engine

type tok struct {
	cells Line
	space bool
	br    bool
}

const (
	alignLeft = iota
	alignCenter
	alignRight
)

func aligned(l Line, w, align int) Line {
	l = trimRight(l)
	if align == alignLeft || len(l) >= w {
		return l
	}
	pad := w - len(l)
	if align == alignCenter {
		pad /= 2
	}
	out := make(Line, 0, pad+len(l))
	out = append(out, spaces(pad, Style{})...)
	return append(out, l...)
}

// wrap breaks inline tokens into lines of at most w cells.
func wrap(toks []tok, w, align int) []Line {
	if w < 1 {
		w = 1
	}
	// adjacent pieces without a space between them are one word
	merged := make([]tok, 0, len(toks))
	for _, t := range toks {
		if n := len(merged); n > 0 && !t.space && !t.br && !merged[n-1].space && !merged[n-1].br {
			merged[n-1].cells = append(merged[n-1].cells[:len(merged[n-1].cells):len(merged[n-1].cells)], t.cells...)
		} else {
			merged = append(merged, t)
		}
	}
	toks = merged
	var out []Line
	var cur, pend Line
	emit := func() {
		out = append(out, aligned(cur, w, align))
		cur, pend = nil, nil
	}
	for _, t := range toks {
		switch {
		case t.br:
			emit()
		case t.space:
			if len(cur) > 0 {
				pend = t.cells
			}
		default:
			c := t.cells
			if len(cur) > 0 && len(cur)+len(pend)+len(c) > w {
				emit()
			}
			if len(cur) > 0 {
				cur = append(cur, pend...)
			}
			pend = nil
			for len(c) > w {
				cut := w
				if c[cut].R == 0 { // do not split a wide rune
					cut--
				}
				if cut <= 0 {
					cut = len(c)
					if cut > 2 {
						cut = 2
					}
				}
				if len(cur) > 0 {
					emit()
				}
				cur = append(cur, c[:cut]...)
				emit()
				c = c[cut:]
			}
			cur = append(cur, c...)
		}
	}
	if len(cur) > 0 {
		emit()
	}
	return out
}
