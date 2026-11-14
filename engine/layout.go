package engine

import (
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type ctx struct {
	p       *Page
	targets []Target
	depth   int
}

func (c *ctx) addTarget(t Target) int32 {
	c.targets = append(c.targets, t)
	return int32(len(c.targets))
}

// layout lays out the children of n as block content w cells wide.
func (c *ctx) layout(n *html.Node, st Style, w, align int) []Line {
	if c.depth > 40 {
		return nil
	}
	c.depth++
	defer func() { c.depth-- }()
	f := &flow{c: c, w: w, align: align}
	f.walk(n, st)
	f.flush()
	return f.out
}

type flow struct {
	c     *ctx
	w     int
	align int
	toks  []tok
	out   []Line
}

func (f *flow) flush() {
	if len(f.toks) > 0 {
		f.out = append(f.out, wrap(f.toks, f.w, f.align)...)
		f.toks = f.toks[:0]
	}
}

// gap inserts one blank line between blocks.
func (f *flow) gap() {
	if len(f.out) > 0 && !isBlank(f.out[len(f.out)-1]) {
		f.out = append(f.out, nil)
	}
}

func (f *flow) space(st Style) {
	if n := len(f.toks); n == 0 || f.toks[n-1].space || f.toks[n-1].br {
		return
	}
	f.toks = append(f.toks, tok{space: true, cells: Line{{' ', st}}})
}

func (f *flow) text(s string, st Style) {
	if s == "" {
		return
	}
	isSp := func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '\f' }
	words := strings.FieldsFunc(s, isSp)
	if len(words) == 0 {
		f.space(st)
		return
	}
	if isSp(rune(s[0])) {
		f.space(st)
	}
	for i, w := range words {
		if i > 0 {
			f.space(st)
		}
		if cs := cells(w, st); len(cs) > 0 {
			f.toks = append(f.toks, tok{cells: cs})
		}
	}
	if isSp(rune(s[len(s)-1])) {
		f.space(st)
	}
}

func (f *flow) walk(n *html.Node, st Style) {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		switch ch.Type {
		case html.TextNode:
			f.text(ch.Data, st)
		case html.ElementNode:
			f.elem(ch, st)
		}
	}
}

var skipTags = map[string]bool{
	"head": true, "script": true, "style": true, "noscript": true, "template": true,
	"svg": true, "iframe": true, "frame": true, "canvas": true, "title": true,
	"link": true, "meta": true, "object": true, "embed": true, "map": true, "dialog": true,
}

var blockTags = map[string]bool{
	"html": true, "body": true, "div": true, "section": true, "article": true, "header": true,
	"footer": true, "main": true, "nav": true, "aside": true, "form": true, "address": true,
	"figure": true, "figcaption": true, "fieldset": true, "legend": true, "details": true,
	"summary": true, "center": true, "dl": true, "dt": true, "menu": true, "dir": true,
	"hgroup": true, "search": true, "tr": true, "td": true, "th": true, "tbody": true,
	"thead": true, "tfoot": true, "caption": true, "optgroup": true,
}

func (f *flow) block(n *html.Node, st Style) {
	f.flush()
	old := f.align
	if a := alignOf(n); a >= 0 {
		f.align = a
	}
	f.walk(n, st)
	f.flush()
	f.align = old
}

// indent lays n's children out in a narrower column with a prefix.
func (f *flow) indent(n *html.Node, st Style, first, rest Line) {
	f.flush()
	w := f.w - len(first)
	if w < 4 {
		w = 4
	}
	lines := trimBlank(f.c.layout(n, st, w, f.align))
	for i, l := range lines {
		pre := rest
		if i == 0 {
			pre = first
		}
		nl := make(Line, 0, len(pre)+len(l))
		nl = append(nl, pre...)
		f.out = append(f.out, append(nl, l...))
	}
}

func (f *flow) elem(n *html.Node, st Style) {
	tag := n.Data
	if skipTags[tag] || hidden(n) {
		return
	}
	st = applyCSS(n, st)
	switch tag {
	case "br":
		f.toks = append(f.toks, tok{br: true})
	case "hr":
		f.flush()
		f.gap()
		w := f.w
		if w > 72 {
			w = 72
		}
		f.out = append(f.out, cells(strings.Repeat("─", w), Style{FG: colRule}))
		f.gap()
	case "img":
		f.img(n, st)
	case "a":
		if href := attr(n, "href"); href != "" && !strings.HasPrefix(strings.ToLower(href), "javascript:") {
			st.T = f.c.addTarget(Target{Kind: TLink, URL: f.c.resolve(href)})
			st.FG = colLink
			st.Under = true
		}
		f.walk(n, st)
	case "input", "button", "select", "textarea":
		f.control(n, st)
	case "table":
		f.flush()
		f.gap()
		f.out = append(f.out, f.c.table(n, st, f.w)...)
		f.gap()
	case "ul", "ol":
		f.flush()
		if f.c.listDepth(n) == 0 {
			f.gap()
		}
		f.out = append(f.out, f.c.list(n, st, f.w)...)
		if f.c.listDepth(n) == 0 {
			f.gap()
		}
	case "li":
		f.indent(n, st, cells("• ", Style{FG: colH2}), spaces(2, Style{}))
	case "p":
		f.flush()
		f.gap()
		f.block(n, st)
		f.gap()
	case "h1", "h2", "h3", "h4", "h5", "h6":
		f.heading(n, st)
	case "pre":
		f.pre(n, st)
	case "blockquote":
		f.gap()
		qs := st
		qs.FG = colQuote
		f.indent(n, qs, cells("│ ", Style{FG: colQuote}), cells("│ ", Style{FG: colQuote}))
		f.gap()
	case "dd":
		f.indent(n, st, spaces(4, Style{}), spaces(4, Style{}))
	case "dt":
		st.Bold = true
		f.block(n, st)
	case "label", "span", "font", "small", "abbr", "sub", "sup", "time", "q", "bdi", "bdo", "wbr", "data", "ins", "output":
		if tag == "small" {
			st.Dim = true
		}
		f.walk(n, st)
	case "b", "strong":
		st.Bold = true
		f.walk(n, st)
	case "i", "em", "cite", "var", "dfn":
		st.Italic = true
		f.walk(n, st)
	case "u":
		st.Under = true
		f.walk(n, st)
	case "mark":
		st.Rev = true
		f.walk(n, st)
	case "code", "kbd", "samp", "tt":
		st.FG = colCode
		f.walk(n, st)
	case "del", "s", "strike":
		st.Dim = true
		f.walk(n, st)
	case "video", "audio":
		f.toks = append(f.toks, tok{cells: cells("["+tag+"]", Style{Dim: true, T: st.T})})
	default:
		if blockTags[tag] {
			f.block(n, st)
		} else {
			f.walk(n, st)
		}
	}
}

func (f *flow) heading(n *html.Node, st Style) {
	f.flush()
	f.gap()
	st.Bold = true
	switch n.Data {
	case "h1":
		st.FG = colH1
	case "h2":
		st.FG = colH2
	case "h3":
		st.FG = colH3
	}
	start := len(f.out)
	f.block(n, st)
	if (n.Data == "h1" || n.Data == "h2") && len(f.out) > start {
		w := maxWidth(f.out[start:])
		if w > f.w {
			w = f.w
		}
		ch := "─"
		if n.Data == "h1" {
			ch = "━"
		}
		if w > 0 {
			f.out = append(f.out, cells(strings.Repeat(ch, w), Style{FG: st.FG}))
		}
	}
	f.gap()
}

func (f *flow) pre(n *html.Node, st Style) {
	f.flush()
	f.gap()
	s := strings.ReplaceAll(textContent(n), "\r", "")
	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimRight(s, "\n ")
	st.FG = colCode
	for _, ln := range strings.Split(strings.ReplaceAll(s, "\t", "    "), "\n") {
		cs := cells(ln, st)
		for len(cs) > f.w && f.w > 2 {
			cut := f.w
			if cs[cut].R == 0 {
				cut--
			}
			f.out = append(f.out, cs[:cut])
			cs = cs[cut:]
		}
		f.out = append(f.out, cs)
	}
	f.gap()
}

func (f *flow) img(n *html.Node, st Style) {
	alt := strings.Join(strings.Fields(attr(n, "alt")), " ")
	src := f.c.p.imgSrc(n)
	var th *Thumb
	if f.c.p.Opts.Images && src != "" {
		th = f.c.p.thumb(src)
	}
	if th == nil || th.OW < 32 || th.OH < 32 {
		ph := Style{Dim: true, T: st.T}
		switch {
		case alt != "":
			f.toks = append(f.toks, tok{cells: cells("["+alt+"]", ph)})
		case th == nil && src != "" && f.c.p.Opts.Images && attrInt(n, "width") != 1:
			f.toks = append(f.toks, tok{cells: cells("[img]", ph)})
		}
		return
	}
	wpx := attrInt(n, "width")
	if wpx <= 0 {
		wpx = th.OW
	}
	cols := wpx / 6
	max := f.w
	if max > 76 {
		max = 76
	}
	if cols > max {
		cols = max
	}
	if cols < 6 {
		cols = 6
	}
	f.flush()
	lines := th.art(cols, f.c.p.Opts.Color, st.T)
	if f.align != alignLeft {
		for i := range lines {
			lines[i] = aligned(lines[i], f.w, f.align)
		}
	}
	f.out = append(f.out, lines...)
}

func (f *flow) control(n *html.Node, st Style) {
	p := f.c.p
	typ := strings.ToLower(attr(n, "type"))
	form := ancestor(n, "form")
	cs := Style{FG: colCtrl}
	if form == nil {
		// buttons and checkboxes outside a form only work with JavaScript
		switch {
		case n.Data == "button", typ == "checkbox", typ == "radio", typ == "button", typ == "submit":
			return
		}
	}
	var label string
	var kind TargetKind
	switch n.Data {
	case "input":
		switch typ {
		case "hidden":
			return
		case "submit", "button", "image", "reset":
			v := attr(n, "value")
			if v == "" {
				v = attr(n, "alt")
			}
			if v == "" {
				v = "Submit"
			}
			label, kind = "[ "+v+" ]", TSubmit
		case "checkbox", "radio":
			on := p.val(n) != ""
			switch {
			case typ == "checkbox" && on:
				label = "[x]"
			case typ == "checkbox":
				label = "[ ]"
			case on:
				label = "(*)"
			default:
				label = "( )"
			}
			kind = TCheck
		default:
			label, kind = f.fieldLabel(n, typ), TText
		}
	case "button":
		t := strings.Join(strings.Fields(textContent(n)), " ")
		if t == "" {
			t = "Button"
		}
		label, kind = "[ "+t+" ]", TSubmit
		if typ == "button" || typ == "reset" {
			kind = TSubmit
		}
	case "textarea":
		label, kind = f.fieldLabel(n, "textarea"), TText
	case "select":
		label, kind = "[ "+p.selectedText(n)+" ▾ ]", TSelect
	}
	cs.T = f.c.addTarget(Target{Kind: kind, Node: n, Form: form})
	f.toks = append(f.toks, tok{cells: cells(label, cs)})
}

func (f *flow) fieldLabel(n *html.Node, typ string) string {
	v := f.c.p.val(n)
	size := attrInt(n, "size")
	if size <= 0 {
		size = 20
	}
	if size > 32 {
		size = 32
	}
	if m := f.w - 4; size > m && m > 6 {
		size = m
	}
	shown := v
	dim := false
	if typ == "password" {
		shown = strings.Repeat("*", len([]rune(v)))
	}
	if v == "" {
		shown = attr(n, "placeholder")
		dim = true
	}
	_ = dim
	shown = strings.Join(strings.Fields(shown), " ")
	rs := []rune(shown)
	used := 0
	var sb strings.Builder
	for _, r := range rs {
		w := RuneWidth(r)
		if used+w > size {
			break
		}
		sb.WriteRune(r)
		used += w
	}
	sb.WriteString(strings.Repeat("_", size-used))
	return "[" + sb.String() + "]"
}

// ---- form values ----

func (p *Page) val(n *html.Node) string {
	if v, ok := p.vals[n]; ok {
		return v
	}
	switch n.Data {
	case "textarea":
		return strings.TrimPrefix(textContent(n), "\n")
	case "input":
		t := strings.ToLower(attr(n, "type"))
		if t == "checkbox" || t == "radio" {
			if hasAttr(n, "checked") {
				return "on"
			}
			return ""
		}
	}
	return attr(n, "value")
}

func (p *Page) SetVal(n *html.Node, v string) {
	// radios in one group are mutually exclusive
	if strings.ToLower(attr(n, "type")) == "radio" && v != "" {
		if form := ancestor(n, "form"); form != nil {
			name := attr(n, "name")
			var walk func(*html.Node)
			walk = func(x *html.Node) {
				if x.Type == html.ElementNode && x.Data == "input" && attr(x, "name") == name && strings.ToLower(attr(x, "type")) == "radio" {
					p.vals[x] = ""
				}
				for c := x.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
			}
			walk(form)
		}
	}
	p.vals[n] = v
}

func options(sel *html.Node) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				if c.Data == "option" {
					out = append(out, c)
				} else if c.Data == "optgroup" {
					walk(c)
				}
			}
		}
	}
	walk(sel)
	return out
}

func optText(o *html.Node) string {
	t := strings.Join(strings.Fields(textContent(o)), " ")
	if t == "" {
		t = attr(o, "value")
	}
	return t
}

func (p *Page) selectedIndex(sel *html.Node) int {
	opts := options(sel)
	if v, ok := p.vals[sel]; ok {
		i, _ := strconv.Atoi(v)
		if i >= 0 && i < len(opts) {
			return i
		}
	}
	for i, o := range opts {
		if hasAttr(o, "selected") {
			return i
		}
	}
	return 0
}

func (p *Page) selectedText(sel *html.Node) string {
	opts := options(sel)
	if len(opts) == 0 {
		return "-"
	}
	return optText(opts[p.selectedIndex(sel)])
}

// CycleSelect moves a <select> to its next option.
func (p *Page) CycleSelect(sel *html.Node) {
	if n := len(options(sel)); n > 0 {
		p.vals[sel] = strconv.Itoa((p.selectedIndex(sel) + 1) % n)
	}
}

// FormValues collects the fields of a form for submission.
func (p *Page) FormValues(form *html.Node, submit *html.Node) url.Values {
	v := url.Values{}
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.ElementNode {
			name := attr(x, "name")
			switch x.Data {
			case "input":
				typ := strings.ToLower(attr(x, "type"))
				switch {
				case name == "" || hasAttr(x, "disabled"):
				case typ == "submit" || typ == "button" || typ == "image" || typ == "reset":
					if x == submit {
						v.Add(name, attr(x, "value"))
					}
				case typ == "checkbox" || typ == "radio":
					if p.val(x) != "" {
						val := attr(x, "value")
						if val == "" {
							val = "on"
						}
						v.Add(name, val)
					}
				default:
					v.Add(name, p.val(x))
				}
			case "textarea":
				if name != "" {
					v.Add(name, p.val(x))
				}
			case "button":
				if name != "" && x == submit {
					v.Add(name, attr(x, "value"))
				}
			case "select":
				if name != "" {
					if opts := options(x); len(opts) > 0 {
						o := opts[p.selectedIndex(x)]
						val := attr(o, "value")
						if !hasAttr(o, "value") {
							val = optText(o)
						}
						v.Add(name, val)
					}
				}
			}
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(form)
	return v
}

// ---- lists ----

func (c *ctx) listDepth(n *html.Node) int {
	d := 0
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && (p.Data == "ul" || p.Data == "ol") {
			d++
		}
	}
	return d
}

func (c *ctx) list(n *html.Node, st Style, w int) []Line {
	var out []Line
	idx := 1
	if s := attr(n, "start"); s != "" {
		idx, _ = strconv.Atoi(s)
	}
	depth := c.listDepth(n)
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" || hidden(li) {
			continue
		}
		var marker string
		if n.Data == "ol" {
			marker = strconv.Itoa(idx) + ". "
		} else if depth%2 == 0 {
			marker = "• "
		} else {
			marker = "◦ "
		}
		idx++
		mw := len([]rune(marker))
		if mw < 2 {
			mw = 2
		}
		sub := trimBlank(c.layout(li, applyCSS(li, st), w-mw, alignLeft))
		first := cells(marker, Style{FG: colH2})
		for len(first) < mw {
			first = append(first, Cell{' ', Style{}})
		}
		for i, l := range sub {
			pre := spaces(mw, Style{})
			if i == 0 {
				pre = first
			}
			nl := make(Line, 0, mw+len(l))
			nl = append(nl, pre...)
			out = append(out, append(nl, l...))
		}
		if len(sub) == 0 {
			out = append(out, first)
		}
	}
	return out
}

// ---- URL / attribute helpers ----

func (c *ctx) resolve(href string) string {
	u, err := c.p.base.Parse(strings.TrimSpace(href))
	if err != nil {
		return href
	}
	// unwrap DuckDuckGo redirect links
	if strings.HasSuffix(u.Host, "duckduckgo.com") && u.Path == "/l/" {
		if t := u.Query().Get("uddg"); t != "" {
			return t
		}
	}
	return u.String()
}

func attrInt(n *html.Node, k string) int {
	v := strings.TrimSpace(strings.TrimSuffix(attr(n, k), "px"))
	i, err := strconv.Atoi(v)
	if err != nil || i < 0 {
		return 0
	}
	return i
}

// Value returns the current value of a form control.
func (p *Page) Value(n *html.Node) string { return p.val(n) }
