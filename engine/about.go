package engine

import (
	"fmt"
	"net/url"
	"strings"

	nethtml "golang.org/x/net/html"
)

const banner = `
    _    ____   ____ ___ ___ __        _______ ____
   / \  / ___| / ___|_ _|_ _|\ \      / / ____| __ )
  / _ \ \___ \| |    | | | |  \ \ /\ / /|  _| |  _ \
 / ___ \ ___) | |___ | | | |   \ V  V / | |___| |_) |
/_/   \_\____/ \____|___|___|   \_/\_/  |_____|____/
`

func (b *Browser) about(raw string) (*Page, error) {
	var body string
	switch raw {
	case "about:home":
		body = `<html><head><title>AsciiWeb</title></head><body>
<pre>` + strings.ReplaceAll(banner, "<", "&lt;") + `</pre>
<center><p>The web, drawn with text. Images become ASCII art.</p>
<form action="https://html.duckduckgo.com/html/" method="get">
<input type="text" name="q" size="36" placeholder="search or type a URL (press o)"> <input type="submit" value="Search">
</form></center>
<h2>Quick keys</h2>
<ul>
<li><b>o</b> open a URL or search &nbsp; <b>Tab</b> next link &nbsp; <b>Enter</b> follow</li>
<li><b>j / k</b> scroll &nbsp; <b>Space / b</b> page &nbsp; <b>g / G</b> top / bottom</li>
<li><b>H / L</b> back / forward &nbsp; <b>r</b> reload &nbsp; <b>/</b> find &nbsp; <b>n / N</b> next / prev match</li>
<li><b>i</b> images on/off &nbsp; <b>c</b> color on/off &nbsp; <b>?</b> help &nbsp; <b>q</b> quit</li>
</ul>
<p>Examples: <a href="https://example.com">example.com</a> &nbsp; <a href="https://en.wikipedia.org/wiki/ASCII_art">ASCII art on Wikipedia</a> &nbsp; <a href="https://news.ycombinator.com">Hacker News</a></p>
</body></html>`
	case "about:help":
		body = `<html><head><title>Help</title></head><body>
<h1>AsciiWeb help</h1>
<h2>Moving</h2>
<ul><li>j k / arrows : line</li><li>Space b / PgDn PgUp : page</li><li>g G / Home End : top bottom</li><li>mouse wheel : scroll, click : follow link</li></ul>
<h2>Links and forms</h2>
<ul><li>Tab, Shift-Tab : next / previous link or field</li><li>Enter : follow link, edit a text field and submit its form, toggle a checkbox, change a select, press a button</li></ul>
<h2>Pages</h2>
<ul><li>o : open URL or search words</li><li>H L : back forward</li><li>r : reload</li><li>/ n N : find</li><li>i : images on or off</li><li>c : color on or off</li><li>h : home</li><li>q : quit</li></ul>
<h2>Not supported</h2>
<p>JavaScript and stylesheets are not run (just like w3m). Sites that need them show their plain HTML.</p>
</body></html>`
	default:
		return nil, fmt.Errorf("unknown page: %s", raw)
	}
	root, err := nethtml.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(raw)
	// forms on the home page post to a real https base
	p := newPage(u, root, &b.Opts)
	return p, nil
}
