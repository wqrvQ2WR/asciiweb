package engine

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func layoutHTML(t *testing.T, src string, cols int) *Page {
	t.Helper()
	root, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://x.test/")
	p := newPage(u, root, &Options{Images: false, Color: true})
	p.Layout(cols)
	return p
}

func text(p *Page) string {
	var sb strings.Builder
	for _, l := range p.Lines {
		sb.WriteString(l.String())
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestWrapAdjacentPiecesStayTogether(t *testing.T) {
	p := layoutHTML(t, `<p>aaaa bbbb (<a href="/x">cccc</a>) dddd</p>`, 14)
	for _, l := range p.Lines {
		if s := l.String(); strings.HasPrefix(s, ")") || strings.HasSuffix(s, "(") {
			t.Fatalf("punctuation split from link: %q", s)
		}
	}
}

func TestWideRunesDoNotOverflow(t *testing.T) {
	p := layoutHTML(t, `<p>한글한글한글한글한글한글 한글한글</p>`, 9)
	for _, l := range p.Lines {
		if len(l) > 9 {
			t.Fatalf("line wider than 9: %d %q", len(l), l.String())
		}
	}
	if !strings.Contains(strings.ReplaceAll(text(p), "\n", ""), "한글한글한글한글한글한글") {
		t.Fatal("text lost")
	}
}

func TestLinkTargets(t *testing.T) {
	p := layoutHTML(t, `<a href="/a">one two</a> <a href="javascript:void(0)">js</a> <a href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.org%2F">ddg</a>`, 40)
	if len(p.Targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(p.Targets))
	}
	if p.Targets[0].URL != "https://x.test/a" {
		t.Fatal(p.Targets[0].URL)
	}
	if p.Targets[1].URL != "https://example.org/" {
		t.Fatal(p.Targets[1].URL)
	}
	if p.TargetLine[0] != 0 {
		t.Fatal("target line")
	}
}

func TestTableFitsWidth(t *testing.T) {
	p := layoutHTML(t, `<table border=1><tr><td>aaaa aaaa aaaa aaaa</td><td>bbbb bbbb bbbb bbbb</td></tr></table>`, 30)
	for _, l := range p.Lines {
		if len(l) > 30 {
			t.Fatalf("table overflows: %d", len(l))
		}
	}
	if !strings.Contains(text(p), "┌") {
		t.Fatal("no border")
	}
}

func TestFormSubmit(t *testing.T) {
	p := layoutHTML(t, `<form action="/s" method="get"><input name="q" value="hi"><select name="k"><option value="1">a<option value="2" selected>b</select><input type="checkbox" name="c" checked><input type="submit" name="go" value="Go"></form>`, 60)
	var form, submit *html.Node
	for _, tg := range p.Targets {
		if tg.Kind == TSubmit {
			form, submit = tg.Form, tg.Node
		}
	}
	m, target, _ := p.SubmitURL(form, submit)
	if m != "GET" || target != "https://x.test/s?c=on&go=Go&k=2&q=hi" {
		t.Fatalf("%s %s", m, target)
	}
}

func TestHiddenAndSkipped(t *testing.T) {
	p := layoutHTML(t, `<p>a<script>x</script><span style="display:none">no</span><span hidden>no</span>b</p>`, 20)
	if got := strings.TrimSpace(text(p)); got != "ab" {
		t.Fatalf("%q", got)
	}
}

func TestImageArtInvertsPaper(t *testing.T) {
	// white paper with a black square: the square must become the dense part
	th := &Thumb{OW: 40, OH: 40, W: 40, H: 40, Pix: make([]uint8, 40*40*3)}
	for i := range th.Pix {
		th.Pix[i] = 255
	}
	for y := 10; y < 30; y++ {
		for x := 10; x < 30; x++ {
			i := (y*40 + x) * 3
			th.Pix[i], th.Pix[i+1], th.Pix[i+2] = 0, 0, 0
		}
	}
	art := th.art(20, false, 0)
	mid := art[len(art)/2].String()
	if !strings.ContainsAny(mid, "#%@") {
		t.Fatalf("ink not dense: %q", mid)
	}
	if strings.TrimSpace(art[0].String()) != "" {
		t.Fatalf("paper not empty: %q", art[0].String())
	}
}
