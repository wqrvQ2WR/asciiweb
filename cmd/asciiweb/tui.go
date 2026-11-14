package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"asciiweb/engine"
)

type navResult struct {
	p      *engine.Page
	err    error
	gen    int
	step   int // 0: new page, +-1: history move
	scroll int
	reload bool
}

type ui struct {
	b          *engine.Browser
	page       *engine.Page
	cols, rows int
	depth      int
	scroll     int
	cur        int // current target, -1 none
	msg        string
	msgUntil   time.Time
	loading    string
	gen        int
	dirty      bool

	prompt   bool
	pkind    string // "open" "find" "field"
	ptext    []rune
	pfield   *engine.Target
	lastFind string

	keys chan key
	navc chan navResult
	upd  chan struct{}
	quit bool
}

func runTUI(b *engine.Browser, input string) {
	restore, err := initTerm()
	if err != nil {
		fmt.Fprintln(os.Stderr, "needs an interactive terminal (use -dump to print a page):", err)
		os.Exit(1)
	}
	out := os.Stdout
	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l\x1b[?1000h\x1b[?1006h")
	cleanup := func() {
		fmt.Fprint(out, "\x1b[?1006l\x1b[?1000l\x1b[?25h\x1b[?1049l\x1b[0m")
		restore()
	}
	defer cleanup()

	u := &ui{b: b, cur: -1, keys: make(chan key, 64), navc: make(chan navResult, 4), upd: make(chan struct{}, 1)}
	u.depth = defaultDepth(os.Getenv)
	if !b.Opts.Color {
		u.depth = engine.NoColor
	}
	u.cols, u.rows = termSize()
	go readKeys(u.keys)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		u.keys <- key{name: "ctrlc"}
	}()

	if input == "" {
		input = "about:home"
	}
	u.open(engine.Normalize(input), true)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	u.draw()
	for !u.quit {
		select {
		case k := <-u.keys:
			u.handle(k)
			u.draw()
		case r := <-u.navc:
			u.onNav(r)
			u.draw()
		case <-u.upd:
			u.dirty = true
		case <-tick.C:
			c, r := termSize()
			if c != u.cols || r != u.rows {
				u.cols, u.rows = c, r
				u.relayout()
				u.dirty = true
			}
			if u.dirty {
				u.dirty = false
				u.relayout()
				u.draw()
			} else if u.msg != "" && time.Now().After(u.msgUntil) && !u.prompt {
				u.msg = ""
				u.draw()
			}
		}
	}
}

func (u *ui) say(s string) {
	u.msg = s
	u.msgUntil = time.Now().Add(4 * time.Second)
}

func (u *ui) view() int { return u.rows - 1 }

func (u *ui) maxScroll() int {
	if u.page == nil {
		return 0
	}
	m := len(u.page.Lines) - u.view()
	if m < 0 {
		m = 0
	}
	return m
}

func (u *ui) clamp() {
	if u.scroll > u.maxScroll() {
		u.scroll = u.maxScroll()
	}
	if u.scroll < 0 {
		u.scroll = 0
	}
}

func (u *ui) relayout() {
	if u.page != nil {
		u.page.Layout(u.cols)
		u.clamp()
		if u.cur >= len(u.page.Targets) {
			u.cur = -1
		}
	}
}

// ---- navigation ----

func (u *ui) open(raw string, push bool) { u.start("GET", raw, "", navResult{}) }

func (u *ui) start(method, raw, body string, tmpl navResult) {
	u.gen++
	gen := u.gen
	u.loading = raw
	go func() {
		p, err := u.b.Fetch(method, raw, body)
		tmpl.p, tmpl.err, tmpl.gen = p, err, gen
		u.navc <- tmpl
	}()
}

func (u *ui) onNav(r navResult) {
	if r.gen != u.gen {
		return
	}
	u.loading = ""
	if r.err != nil {
		u.say("error: " + r.err.Error())
		return
	}
	switch {
	case r.step != 0:
		u.b.MoveTo(r.step, r.p, u.scroll)
	case r.reload:
		u.b.Commit(r.p, false, u.scroll)
	default:
		u.b.Commit(r.p, true, u.scroll)
	}
	u.show(r.p, r.scroll)
}

func (u *ui) show(p *engine.Page, scroll int) {
	u.page = p
	u.cur = -1
	u.scroll = scroll
	u.relayout()
	p.LoadImages(u.b.Client, func() {
		select {
		case u.upd <- struct{}{}:
		default:
		}
	})
}

func (u *ui) step(d int) {
	raw, method, body, cached, scroll, ok := u.b.Step(d)
	if !ok {
		u.say("no more history")
		return
	}
	if cached != nil {
		u.b.MoveTo(d, cached, u.scroll)
		u.show(cached, scroll)
		return
	}
	u.start(method, raw, body, navResult{step: d, scroll: scroll})
}

// ---- input ----

func (u *ui) handle(k key) {
	if u.prompt {
		u.handlePrompt(k)
		return
	}
	v := u.view()
	switch k.name {
	case "ctrlc":
		u.quit = true
	case "esc":
		u.cur = -1
	case "up":
		u.scroll--
	case "down":
		u.scroll++
	case "pgdn":
		u.scroll += v - 1
	case "pgup":
		u.scroll -= v - 1
	case "home":
		u.scroll = 0
	case "end":
		u.scroll = u.maxScroll()
	case "wheelup":
		u.scroll -= 3
	case "wheeldown":
		u.scroll += 3
	case "tab":
		u.moveTarget(1)
	case "btab":
		u.moveTarget(-1)
	case "enter":
		if u.cur >= 0 {
			u.activate(u.cur)
		}
	case "right":
		u.step(1)
	case "left":
		u.step(-1)
	case "click":
		u.click(k.x-1, k.y-1)
	default:
		switch k.r {
		case 'q':
			u.quit = true
		case 'j':
			u.scroll++
		case 'k':
			u.scroll--
		case ' ', 'f':
			u.scroll += v - 1
		case 'b':
			u.scroll -= v - 1
		case 'd':
			u.scroll += v / 2
		case 'u':
			u.scroll -= v / 2
		case 'g':
			u.scroll = 0
		case 'G':
			u.scroll = u.maxScroll()
		case 'o':
			u.startPrompt("open", "")
		case 'O':
			if u.page != nil {
				u.startPrompt("open", u.page.URL.String())
			}
		case '/':
			u.startPrompt("find", "")
		case 'n':
			u.find(u.lastFind, 1)
		case 'N':
			u.find(u.lastFind, -1)
		case 'H':
			u.step(-1)
		case 'L':
			u.step(1)
		case 'h':
			u.open("about:home", true)
		case '?':
			u.open("about:help", true)
		case 'r':
			if u.page != nil {
				u.start(u.page.Method, u.page.URL.String(), u.page.Body, navResult{reload: true, scroll: u.scroll})
				u.say("reloading")
			}
		case 'i':
			u.b.Opts.Images = !u.b.Opts.Images
			if u.b.Opts.Images && u.page != nil {
				u.page.LoadImages(u.b.Client, func() {
					select {
					case u.upd <- struct{}{}:
					default:
					}
				})
			}
			u.relayout()
			u.say(map[bool]string{true: "images on", false: "images off"}[u.b.Opts.Images])
		case 'c':
			u.b.Opts.Color = !u.b.Opts.Color
			if u.b.Opts.Color {
				u.depth = defaultDepth(os.Getenv)
			} else {
				u.depth = engine.NoColor
			}
			u.relayout()
			u.say(map[bool]string{true: "color on", false: "color off"}[u.b.Opts.Color])
		}
	}
	u.clamp()
}

func (u *ui) click(x, y int) {
	if u.page == nil || y < 0 || y >= u.view() {
		return
	}
	ly := u.scroll + y
	if ly >= len(u.page.Lines) {
		return
	}
	l := u.page.Lines[ly]
	if x < 0 || x >= len(l) {
		return
	}
	if t := int(l[x].S.T) - 1; t >= 0 {
		u.cur = t
		u.activate(t)
	}
}

func (u *ui) moveTarget(d int) {
	p := u.page
	if p == nil || len(p.Targets) == 0 {
		return
	}
	n := len(p.Targets)
	visible := func(i int) bool {
		l := p.TargetLine[i]
		return l >= u.scroll && l < u.scroll+u.view()
	}
	next := -1
	if u.cur >= 0 && u.cur < n && visible(u.cur) {
		next = u.cur + d
	} else if d > 0 {
		for i := 0; i < n; i++ {
			if p.TargetLine[i] >= u.scroll {
				next = i
				break
			}
		}
	} else {
		for i := n - 1; i >= 0; i-- {
			if p.TargetLine[i] < u.scroll+u.view() {
				next = i
				break
			}
		}
	}
	if next < 0 || next >= n {
		u.say("no more links")
		return
	}
	u.cur = next
	if l := p.TargetLine[next]; l >= 0 && !visible(next) {
		u.scroll = l - u.view()/3
	}
}

func (u *ui) activate(id int) {
	p := u.page
	if p == nil || id < 0 || id >= len(p.Targets) {
		return
	}
	t := p.Targets[id]
	switch t.Kind {
	case engine.TLink:
		if strings.HasPrefix(t.URL, "#") || sameDoc(t.URL, p.URL.String()) {
			u.say("anchor on this page")
			return
		}
		u.open(t.URL, true)
	case engine.TText:
		u.pfield = &t
		u.startPrompt("field", p.Value(t.Node))
	case engine.TCheck:
		if strings.EqualFold(attrOf(t.Node, "type"), "radio") {
			p.SetVal(t.Node, "on")
		} else if p.Value(t.Node) != "" {
			p.SetVal(t.Node, "")
		} else {
			p.SetVal(t.Node, "on")
		}
		u.relayout()
	case engine.TSelect:
		p.CycleSelect(t.Node)
		u.relayout()
	case engine.TSubmit:
		u.submit(t.Form, t.Node)
	}
}

func (u *ui) submit(form, btn *htmlNode) {
	if form == nil {
		u.say("this button needs JavaScript")
		return
	}
	method, target, body := u.page.SubmitURL(form, btn)
	u.start(method, target, body, navResult{})
	u.say("loading...")
}

func sameDoc(a, b string) bool {
	if i := strings.IndexByte(a, '#'); i >= 0 {
		return a[:i] == strings.SplitN(b, "#", 2)[0]
	}
	return false
}

func (u *ui) find(q string, d int) {
	if q == "" || u.page == nil {
		return
	}
	q = strings.ToLower(q)
	n := len(u.page.Lines)
	for i := 1; i <= n; i++ {
		y := ((u.scroll+d*i)%n + n) % n
		if strings.Contains(strings.ToLower(u.page.Lines[y].String()), q) {
			u.scroll = y
			u.clamp()
			u.say(fmt.Sprintf("found at line %d", y+1))
			return
		}
	}
	u.say("not found: " + q)
}

// ---- prompt ----

func (u *ui) startPrompt(kind, pre string) {
	u.prompt, u.pkind, u.ptext = true, kind, []rune(pre)
}

func (u *ui) handlePrompt(k key) {
	switch k.name {
	case "esc", "ctrlc":
		u.prompt = false
	case "enter":
		text := string(u.ptext)
		u.prompt = false
		switch u.pkind {
		case "open":
			u.open(engine.Normalize(text), true)
		case "find":
			u.lastFind = text
			u.find(text, 1)
		case "field":
			t := u.pfield
			u.page.SetVal(t.Node, text)
			u.relayout()
			if t.Form != nil && t.Node.Data != "textarea" {
				u.submit(t.Form, nil)
			}
		}
	case "bs":
		if len(u.ptext) > 0 {
			u.ptext = u.ptext[:len(u.ptext)-1]
		}
	case "ctrlu":
		u.ptext = nil
	case "":
		if k.r != 0 {
			u.ptext = append(u.ptext, k.r)
		}
	}
}

// ---- drawing ----

func (u *ui) draw() {
	var sb strings.Builder
	sb.WriteString("\x1b[H")
	v := u.view()
	for i := 0; i < v; i++ {
		y := u.scroll + i
		if u.page != nil && y < len(u.page.Lines) {
			sb.WriteString(renderLine(u.page.Lines[y], u.cols, u.cur, u.depth))
		}
		sb.WriteString("\x1b[0m\x1b[K\r\n")
	}
	sb.WriteString(u.statusBar())
	sb.WriteString("\x1b[0m\x1b[K")
	os.Stdout.WriteString(sb.String())
}

func (u *ui) statusBar() string {
	left, right := u.statusText()
	if u.prompt {
		label := map[string]string{"open": "open: ", "find": "find: ", "field": "input: "}[u.pkind]
		left = label + string(u.ptext) + "_"
		right = "Enter ok  Esc cancel"
	}
	return gradientBar(left, right, u.cols, u.depth)
}

func (u *ui) statusText() (string, string) {
	left := ""
	switch {
	case u.msg != "":
		left = u.msg
	case u.loading != "":
		left = "loading " + u.loading
	case u.page != nil && u.cur >= 0 && u.cur < len(u.page.Targets):
		t := u.page.Targets[u.cur]
		switch t.Kind {
		case engine.TLink:
			left = t.URL
		case engine.TText:
			left = "text field (Enter to edit)"
		case engine.TCheck:
			left = "checkbox (Enter to toggle)"
		case engine.TSelect:
			left = "select (Enter for next option)"
		case engine.TSubmit:
			left = "button (Enter to press)"
		}
	case u.page != nil:
		left = u.page.Title
		if left == "" {
			left = u.page.URL.String()
		}
	}
	right := ""
	if u.page != nil {
		pct := 100
		if m := u.maxScroll(); m > 0 {
			pct = u.scroll * 100 / m
		}
		flags := ""
		if !u.b.Opts.Images {
			flags += " noimg"
		}
		right = fmt.Sprintf("%d%%%s  ?:help", pct, flags)
	}
	return left, right
}
