package main

import (
	"strings"

	"asciiweb/engine"
)

// renderLine writes one line as ANSI text, clipped to cols. Cells of the
// target hl are drawn reversed.
func renderLine(l engine.Line, cols, hl, depth int) string {
	if len(l) > cols {
		l = l[:cols]
		if last := l[cols-1]; last.R != 0 && engine.RuneWidth(last.R) == 2 {
			l = append(l[:cols-1:cols-1], engine.Cell{R: ' '})
		}
	}
	var sb strings.Builder
	var cur engine.Style
	plain := true
	for _, c := range l {
		if c.R == 0 {
			continue
		}
		s := c.S
		if hl >= 0 && int(s.T) == hl+1 {
			s.Rev = true
		}
		if !engine.SameLook(s, cur) || (plain && s != (engine.Style{})) {
			if s.FG == 0 && !s.Bold && !s.Dim && !s.Italic && !s.Under && !s.Rev {
				sb.WriteString("\x1b[0m")
			} else {
				sb.WriteString(engine.SGR(s, depth))
			}
			cur = s
			cur.T = 0
			plain = false
		}
		sb.WriteRune(c.R)
	}
	if !plain {
		sb.WriteString("\x1b[0m")
	}
	return sb.String()
}
