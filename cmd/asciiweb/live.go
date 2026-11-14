package main

import (
	"fmt"
	"image"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/chromedp/kb"

	"asciiweb/engine"
	"asciiweb/render"
)

type setItem struct {
	label  string
	get    func() string
	change func(dir int)
	reload bool // needs the browser viewport to be re-applied
}

type live struct {
	set        render.Settings
	sess       *render.Session
	cols, rows int
	depth      int
	img        image.Image
	grid       []engine.Line
	prev       []string
	mode       string // page, url, settings
	text       []rune
	sel        int
	items      []setItem
	needApply  bool
	msg        string
	msgUntil   time.Time
	barKey     string
	keys       chan key
	quit       bool
}

func cycleS(cur string, list []string, dir int) string {
	for i, v := range list {
		if v == cur {
			return list[(i+dir+len(list))%len(list)]
		}
	}
	return list[0]
}

func cycleI(cur int, list []int, dir int) int {
	best := 0
	for i, v := range list {
		if v <= cur {
			best = i
		}
	}
	return list[(best+dir+len(list))%len(list)]
}

func onoff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (u *live) buildItems() {
	s := &u.set
	u.items = []setItem{
		{"Style", func() string { return s.Style + "  (" + render.StyleDesc[s.Style] + ")" }, func(d int) { s.Style = cycleS(s.Style, render.Styles, d) }, false},
		{"Color", func() string { return s.Color }, func(d int) { s.Color = cycleS(s.Color, render.Colors, d) }, false},
		{"Real text overlay", func() string { return onoff(s.Text) }, func(d int) { s.Text = !s.Text }, false},
		{"Bold characters", func() string { return onoff(s.Bold) }, func(d int) { s.Bold = !s.Bold }, false},
		{"Invert (dark ink on light page)", func() string { return s.Invert }, func(d int) { s.Invert = cycleS(s.Invert, render.Inverts, d) }, false},
		{"Contrast", func() string { return fmt.Sprintf("%+d", s.Contrast) }, func(d int) { s.Contrast += d; s.Normalize() }, false},
		{"Page width (px)", func() string {
			if s.PageWidth == 0 {
				return "auto"
			}
			return fmt.Sprint(s.PageWidth)
		}, func(d int) { s.PageWidth = cycleI(s.PageWidth, render.Widths, d) }, true},
		{"Frame quality (JPEG)", func() string { return fmt.Sprint(s.JPEG) }, func(d int) { s.JPEG = cycleI(s.JPEG, render.JPEGs, d) }, true},
		{"Frame rate (fps)", func() string { return fmt.Sprint(s.FPS) }, func(d int) { s.FPS = cycleI(s.FPS, render.FPSes, d) }, true},
		{"Mobile sites", func() string { return onoff(s.Mobile) }, func(d int) { s.Mobile = !s.Mobile }, true},
		{"Search engine", func() string { return s.Search }, func(d int) { s.Search = cycleS(s.Search, render.Searches, d) }, false},
		{"Home page", func() string { return s.Home }, func(d int) {}, false},
	}
}

func (u *live) frameRows() int { return u.rows - 1 }

func (u *live) viewport() (int, int) {
	w := u.set.PageWidth
	if w == 0 { // auto: about 5 px per cell, so a character spans one to two cells
		w = u.cols * 5
		if w < 360 {
			w = 360
		}
	}
	h := 2 * w * u.frameRows() / u.cols
	if h < 200 {
		h = 200
	}
	return w, h
}

func (u *live) colorDepth() int {
	switch u.set.Color {
	case "none":
		return engine.NoColor
	case "256":
		return engine.Color256
	case "true":
		return engine.TrueColor
	}
	return defaultDepth(os.Getenv)
}

func (u *live) say(s string) {
	u.msg, u.msgUntil = s, time.Now().Add(3*time.Second)
}

func runLive(set render.Settings, input string) error {
	debug.SetGCPercent(40)
	debug.SetMemoryLimit(96 << 20)
	u := &live{set: set, mode: "page", keys: make(chan key, 128)}
	u.buildItems()
	restore, err := initTerm()
	if err != nil {
		return fmt.Errorf("needs an interactive terminal (use -shot to print one frame): %w", err)
	}
	out := os.Stdout
	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l\x1b[?1000h\x1b[?1006h\x1b[2J")
	defer func() {
		fmt.Fprint(out, "\x1b[?1006l\x1b[?1000l\x1b[?25h\x1b[?1049l\x1b[0m")
		restore()
	}()
	u.cols, u.rows = termSize()
	u.depth = u.colorDepth()
	fmt.Fprint(out, "\x1b[Hstarting the browser...")

	w, h := u.viewport()
	sess, err := render.Start(u.set, w, h)
	if err != nil {
		return err
	}
	u.sess = sess
	defer sess.Close()
	go readKeys(u.keys)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sig
		u.keys <- key{name: "ctrlc"}
		time.Sleep(2 * time.Second)
		sess.Close()
		os.Exit(0)
	}()

	start := u.set.Home
	if input != "" {
		start = u.resolve(input)
	}
	sess.Navigate(start)

	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	for !u.quit {
		select {
		case k := <-u.keys:
			u.handle(k)
			u.draw()
		case <-sess.Frames():
			u.img = sess.Frame()
			u.rebuild()
			u.draw()
		case <-tick.C:
			c, r := termSize()
			if c != u.cols || r != u.rows {
				u.cols, u.rows = c, r
				u.prev = nil
				fmt.Fprint(out, "\x1b[2J")
				w, h := u.viewport()
				go sess.Apply(u.set, w, h)
				u.rebuild()
				u.draw()
				continue
			}
			bk := u.barText()
			if bk != u.barKey || (u.msg != "" && time.Now().After(u.msgUntil)) {
				if time.Now().After(u.msgUntil) {
					u.msg = ""
				}
				u.draw()
			}
		}
	}
	return nil
}

func (u *live) resolve(in string) string {
	n := engine.Normalize(in)
	const ddg = "https://html.duckduckgo.com/html/?q="
	if strings.HasPrefix(n, ddg) {
		q, _ := url.QueryUnescape(strings.TrimPrefix(n, ddg))
		return u.set.SearchURL(q)
	}
	if n == "about:home" {
		return u.set.Home
	}
	return n
}

func (u *live) rebuild() {
	if u.img == nil || u.cols < 4 || u.frameRows() < 2 {
		return
	}
	u.grid = render.Convert(u.img, u.cols, u.frameRows(), u.set, u.depth)
	if u.set.Text {
		u.grid = render.OverlayText(u.grid, u.sess.Words(), u.set, u.depth, u.sess.FrameScroll())
	}
}

// ---- input ----

func (u *live) handle(k key) {
	switch u.mode {
	case "url":
		u.handleURL(k)
		return
	case "settings":
		u.handleSettings(k)
		return
	}
	if k.name == "ctrl" {
		switch k.r {
		case 'q':
			u.quit = true
		case 'l':
			cur, _ := u.sess.Info()
			u.mode, u.text = "url", []rune(cur)
		case 'r':
			u.sess.Reload()
			u.say("reloading")
		case 'b':
			u.sess.Back()
		case 'f':
			u.sess.Forward()
		case 's':
			u.mode = "settings"
			u.needApply = false
		case 'h':
			u.sess.Navigate(u.set.Home)
		}
		return
	}
	w, h := u.sess.Size()
	fr := u.frameRows()
	px := func(x, y int) (float64, float64) {
		return (float64(x-1) + 0.5) / float64(u.cols) * float64(w), (float64(y-2) + 0.5) / float64(fr) * float64(h)
	}
	switch k.name {
	case "ctrlc":
		u.quit = true
	case "click":
		if k.y >= 2 {
			x, y := px(k.x, k.y)
			u.sess.Click(x, y)
		} else {
			cur, _ := u.sess.Info()
			u.mode, u.text = "url", []rune(cur)
		}
	case "wheelup", "wheeldown":
		x, y := px(k.x, k.y)
		d := 120.0
		if k.name == "wheelup" {
			d = -d
		}
		u.sess.Wheel(x, y, d)
	case "enter":
		u.sess.Key(kb.Enter)
	case "bs":
		u.sess.Key(kb.Backspace)
	case "tab":
		u.sess.Key(kb.Tab)
	case "esc":
		u.sess.Key(kb.Escape)
	case "up":
		u.sess.Key(kb.ArrowUp)
	case "down":
		u.sess.Key(kb.ArrowDown)
	case "left":
		u.sess.Key(kb.ArrowLeft)
	case "right":
		u.sess.Key(kb.ArrowRight)
	case "pgup":
		u.sess.Key(kb.PageUp)
	case "pgdn":
		u.sess.Key(kb.PageDown)
	case "home":
		u.sess.Key(kb.Home)
	case "end":
		u.sess.Key(kb.End)
	case "del":
		u.sess.Key(kb.Delete)
	case "":
		if k.r != 0 {
			u.sess.Type(string(k.r))
		}
	}
}

func (u *live) handleURL(k key) {
	switch k.name {
	case "esc", "ctrlc":
		u.mode = "page"
	case "enter":
		u.mode = "page"
		if t := strings.TrimSpace(string(u.text)); t != "" {
			u.sess.Navigate(u.resolve(t))
		}
	case "bs":
		if len(u.text) > 0 {
			u.text = u.text[:len(u.text)-1]
		}
	case "ctrl":
		if k.r == 'u' {
			u.text = nil
		}
	case "ctrlu":
		u.text = nil
	case "":
		if k.r != 0 {
			u.text = append(u.text, k.r)
		}
	}
}

func (u *live) handleSettings(k key) {
	close := func() {
		u.mode = "page"
		u.set.Normalize()
		if err := u.set.Save(); err != nil {
			u.say("could not save settings: " + err.Error())
		}
		if u.needApply {
			w, h := u.viewport()
			go u.sess.Apply(u.set, w, h)
		}
	}
	it := &u.items[u.sel]
	switch {
	case k.name == "esc" || k.name == "ctrlc" || (k.name == "ctrl" && k.r == 's'):
		close()
	case k.name == "up":
		u.sel = (u.sel + len(u.items) - 1) % len(u.items)
	case k.name == "down" || k.name == "tab":
		u.sel = (u.sel + 1) % len(u.items)
	case k.name == "left" || k.name == "right" || k.name == "enter":
		if it.label == "Home page" {
			// use the current page as home
			if cur, _ := u.sess.Info(); cur != "" {
				u.set.Home = cur
			}
			return
		}
		d := 1
		if k.name == "left" {
			d = -1
		}
		it.change(d)
		if it.reload {
			u.needApply = true
		}
		u.depth = u.colorDepth()
		u.rebuild()
	}
}

// ---- drawing ----

func (u *live) barText() string {
	l, r := u.barLR()
	return l + "|" + r
}

func (u *live) barLR() (string, string) {
	switch u.mode {
	case "url":
		return "go to: " + string(u.text) + "_", "Enter go  Esc cancel"
	case "settings":
		return "Settings", "up/down choose  left/right change  Esc close"
	}
	cur, title := u.sess.Info()
	left := cur
	if title != "" {
		left = title + "  -  " + cur
	}
	if u.msg != "" {
		left = u.msg
	}
	return left, "^L url ^B back ^F fwd ^R reload ^S settings ^Q quit"
}

func (u *live) draw() {
	if u.cols < 4 || u.rows < 3 {
		return
	}
	l, r := u.barLR()
	u.barKey = l + "|" + r
	rows := make([]string, u.rows)
	rows[0] = gradientBar(l, r, u.cols, u.depth) + "\x1b[0m"
	grid := u.grid
	if u.mode == "settings" {
		grid = u.overlay(grid)
	}
	for i := 0; i < u.frameRows(); i++ {
		if i < len(grid) {
			rows[i+1] = renderLine(grid[i], u.cols, -1, u.depth)
		}
	}
	var sb strings.Builder
	if len(u.prev) != len(rows) {
		u.prev = make([]string, len(rows))
		for i := range u.prev {
			u.prev[i] = "\x00"
		}
	}
	for i, s := range rows {
		if s != u.prev[i] {
			fmt.Fprintf(&sb, "\x1b[%d;1H%s\x1b[0m\x1b[K", i+1, s)
			u.prev[i] = s
		}
	}
	if sb.Len() > 0 {
		os.Stdout.WriteString(sb.String())
	}
}

// overlay draws the settings panel over a copy of the frame.
func (u *live) overlay(grid []engine.Line) []engine.Line {
	out := make([]engine.Line, len(grid))
	for i := range grid {
		out[i] = append(engine.Line(nil), grid[i]...)
	}
	for len(out) < u.frameRows() {
		out = append(out, make(engine.Line, u.cols))
	}
	pw := u.cols - 4
	if pw > 78 {
		pw = 78
	}
	if pw < 24 {
		pw = u.cols
	}
	panel := engine.Style{FG: engine.Rgb(0xF2, 0xF2, 0xFA), BG: engine.Rgb(0x1B, 0x1B, 0x2B)}
	selSt := engine.Style{FG: engine.Rgb(255, 255, 255), BG: engine.Rgb(0xC2, 0x2F, 0x82), Bold: true}
	var lines []engine.Line
	pad := func(s string, st engine.Style) engine.Line {
		l := engine.Text(" "+s, st)
		if len(l) > pw {
			l = l[:pw]
		}
		for len(l) < pw {
			l = append(l, engine.Cell{R: ' ', S: st})
		}
		return l
	}
	lines = append(lines, pad("Browser settings", engine.Style{FG: engine.Rgb(0xFF, 0x4D, 0x9D), BG: panel.BG, Bold: true}))
	for i, it := range u.items {
		st := panel
		if i == u.sel {
			st = selSt
		}
		lines = append(lines, pad(fmt.Sprintf("%-32s %s", it.label, it.get()), st))
	}
	lines = append(lines, pad("changes to width, quality, fps and mobile apply when you close", engine.Style{FG: engine.Rgb(0x9A, 0xA0, 0xB5), BG: panel.BG}))
	y0 := (u.frameRows() - len(lines)) / 2
	if y0 < 0 {
		y0 = 0
	}
	x0 := (u.cols - pw) / 2
	for i, l := range lines {
		y := y0 + i
		if y >= len(out) {
			break
		}
		row := out[y]
		for len(row) < u.cols {
			row = append(row, engine.Cell{R: ' '})
		}
		copy(row[x0:], l)
		out[y] = row
	}
	return out
}
