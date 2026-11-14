package engine

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Only inline style="" and a few legacy attributes are understood; there is
// no stylesheet engine, like w3m.

func styleProps(n *html.Node) map[string]string {
	s := attr(n, "style")
	if s == "" {
		return nil
	}
	m := map[string]string{}
	for _, d := range strings.Split(s, ";") {
		if i := strings.IndexByte(d, ':'); i > 0 {
			m[strings.ToLower(strings.TrimSpace(d[:i]))] = strings.ToLower(strings.TrimSpace(d[i+1:]))
		}
	}
	return m
}

func hidden(n *html.Node) bool {
	if hasAttr(n, "hidden") || attr(n, "aria-hidden") == "true" {
		return true
	}
	if n.Data == "input" && strings.EqualFold(attr(n, "type"), "hidden") {
		return true
	}
	if m := styleProps(n); m != nil {
		v := strings.ReplaceAll(m["display"], " ", "")
		if strings.HasPrefix(v, "none") || m["visibility"] == "hidden" {
			return true
		}
	}
	return false
}

func alignOf(n *html.Node) int {
	if n.Data == "center" {
		return alignCenter
	}
	v := strings.ToLower(attr(n, "align"))
	if m := styleProps(n); m != nil && m["text-align"] != "" {
		v = m["text-align"]
	}
	switch v {
	case "center", "middle":
		return alignCenter
	case "right", "end":
		return alignRight
	case "left", "start":
		return alignLeft
	}
	return -1
}

func applyCSS(n *html.Node, st Style) Style {
	if n.Data == "font" {
		if c, ok := parseColor(attr(n, "color")); ok {
			st.FG = c
		}
	}
	if m := styleProps(n); m != nil {
		if c, ok := parseColor(m["color"]); ok {
			st.FG = c
		}
		switch w := m["font-weight"]; {
		case w == "bold" || w == "bolder":
			st.Bold = true
		case len(w) == 3 && w[0] >= '6' && w[0] <= '9':
			st.Bold = true
		}
		if m["font-style"] == "italic" {
			st.Italic = true
		}
		if strings.Contains(m["text-decoration"], "underline") {
			st.Under = true
		}
	}
	return st
}

var namedColors = map[string]uint32{
	"red": 0xFF5555, "green": 0x55DD77, "blue": 0x5599FF, "yellow": 0xFFDD55, "orange": 0xFFA040,
	"purple": 0xB070FF, "pink": 0xFF77BB, "cyan": 0x55DDEE, "magenta": 0xFF55EE, "lime": 0x77FF55,
	"white": 0xFFFFFF, "gray": 0xAAAAAA, "grey": 0xAAAAAA, "silver": 0xCCCCCC, "gold": 0xFFD24D,
	"teal": 0x44CCBB, "navy": 0x6677FF, "maroon": 0xFF6677, "olive": 0xCCCC55, "aqua": 0x55DDEE,
}

// parseColor accepts #rgb, #rrggbb, rgb(...) and a few names. Colors that
// would be unreadable on a dark terminal are rejected.
func parseColor(s string) (uint32, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	var r, g, b int
	switch {
	case s == "":
		return 0, false
	case strings.HasPrefix(s, "#") && len(s) == 4:
		v, err := strconv.ParseUint(s[1:], 16, 16)
		if err != nil {
			return 0, false
		}
		r, g, b = int(v>>8&15)*17, int(v>>4&15)*17, int(v&15)*17
	case strings.HasPrefix(s, "#") && len(s) == 7:
		v, err := strconv.ParseUint(s[1:], 16, 32)
		if err != nil {
			return 0, false
		}
		r, g, b = int(v>>16&255), int(v>>8&255), int(v&255)
	case strings.HasPrefix(s, "rgb"):
		i, j := strings.IndexByte(s, '('), strings.IndexByte(s, ')')
		if i < 0 || j < i {
			return 0, false
		}
		parts := strings.FieldsFunc(s[i+1:j], func(r rune) bool { return r == ',' || r == ' ' || r == '/' })
		if len(parts) < 3 {
			return 0, false
		}
		vals := [3]int{}
		for k := 0; k < 3; k++ {
			v, err := strconv.Atoi(strings.TrimSuffix(parts[k], "%"))
			if err != nil {
				return 0, false
			}
			vals[k] = v
		}
		r, g, b = vals[0], vals[1], vals[2]
	default:
		v, ok := namedColors[s]
		if !ok {
			return 0, false
		}
		r, g, b = int(v>>16), int(v>>8&255), int(v&255)
	}
	if (299*r+587*g+114*b)/1000 < 110 {
		return 0, false
	}
	return Rgb(uint8(r), uint8(g), uint8(b)), true
}
