package engine

import (
	"fmt"
	"strings"
)

// Color depth for terminal output.
const (
	NoColor = iota
	Color256
	TrueColor
)

// SGR returns the escape sequence that switches to style s from a reset state.
func SGR(s Style, depth int) string {
	var sb strings.Builder
	sb.WriteString("\x1b[0")
	if s.Bold {
		sb.WriteString(";1")
	}
	if s.Dim {
		sb.WriteString(";2")
	}
	if s.Italic {
		sb.WriteString(";3")
	}
	if s.Under {
		sb.WriteString(";4")
	}
	if s.Rev {
		sb.WriteString(";7")
	}
	if s.FG != 0 && depth != NoColor {
		r, g, b := int(s.FG>>16&255), int(s.FG>>8&255), int(s.FG&255)
		if depth == TrueColor {
			fmt.Fprintf(&sb, ";38;2;%d;%d;%d", r, g, b)
		} else {
			fmt.Fprintf(&sb, ";38;5;%d", to256(r, g, b))
		}
	}
	if s.BG != 0 && depth != NoColor {
		r, g, b := int(s.BG>>16&255), int(s.BG>>8&255), int(s.BG&255)
		if depth == TrueColor {
			fmt.Fprintf(&sb, ";48;2;%d;%d;%d", r, g, b)
		} else {
			fmt.Fprintf(&sb, ";48;5;%d", to256(r, g, b))
		}
	}
	sb.WriteByte('m')
	return sb.String()
}

func to256(r, g, b int) int {
	if r == g && g == b {
		if r < 8 {
			return 16
		}
		if r > 248 {
			return 231
		}
		return 232 + (r-8)*24/247
	}
	q := func(v int) int { return (v*5 + 127) / 255 }
	return 16 + 36*q(r) + 6*q(g) + q(b)
}

// SameLook reports whether two styles print identically.
func SameLook(a, b Style) bool {
	a.T, b.T = 0, 0
	return a == b
}
