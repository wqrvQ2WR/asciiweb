package engine

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

type Options struct {
	Images bool
	Color  bool
}

type TargetKind int

const (
	TLink TargetKind = iota + 1
	TText
	TCheck
	TSelect
	TSubmit
)

type Target struct {
	Kind TargetKind
	URL  string     // links
	Node *html.Node // controls
	Form *html.Node
}

// Page is one loaded document plus its current layout.
type Page struct {
	URL    *url.URL
	Title  string
	Root   *html.Node
	Method string
	Body   string // POST body, for reload
	Opts   *Options

	Lines      []Line
	Targets    []Target
	TargetLine []int

	vals   map[*html.Node]string
	mu     sync.Mutex
	thumbs map[string]*Thumb
	cancel context.CancelFunc
	base   *url.URL
}

func newPage(u *url.URL, root *html.Node, opts *Options) *Page {
	p := &Page{URL: u, Root: root, Opts: opts, vals: map[*html.Node]string{}, thumbs: map[string]*Thumb{}}
	p.base = u
	if b := findTag(root, "base"); b != nil {
		if h := attr(b, "href"); h != "" {
			if bu, err := u.Parse(h); err == nil {
				p.base = bu
			}
		}
	}
	if t := findTag(root, "title"); t != nil {
		p.Title = strings.Join(strings.Fields(textContent(t)), " ")
	}
	return p
}

// Layout lays the document out for the given terminal width.
func (p *Page) Layout(cols int) {
	if cols < 8 {
		cols = 8
	}
	c := &ctx{p: p}
	lines := c.layout(p.Root, Style{}, cols, alignLeft)
	p.Lines = trimBlank(lines)
	p.Targets = c.targets
	p.TargetLine = make([]int, len(p.Targets))
	for i := range p.TargetLine {
		p.TargetLine[i] = -1
	}
	for y, l := range p.Lines {
		for _, cell := range l {
			if t := int(cell.S.T) - 1; t >= 0 && t < len(p.TargetLine) && p.TargetLine[t] < 0 {
				p.TargetLine[t] = y
			}
		}
	}
}

func (p *Page) thumb(u string) *Thumb {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.thumbs[u]
}

// Stop cancels pending image downloads.
func (p *Page) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *Page) imageURLs() []string {
	seen := map[string]bool{}
	var out []string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			if u := p.imgSrc(n); u != "" && !seen[u] {
				seen[u] = true
				out = append(out, u)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(p.Root)
	return out
}

func (p *Page) imgSrc(n *html.Node) string {
	src := attr(n, "src")
	if src == "" || strings.HasPrefix(src, "data:") {
		for _, k := range []string{"data-src", "data-original", "data-lazy-src"} {
			if v := attr(n, k); v != "" {
				src = v
				break
			}
		}
	}
	if src == "" {
		if ss := attr(n, "srcset"); ss != "" {
			src = strings.Fields(strings.Split(ss, ",")[0])[0]
		}
	}
	if src == "" || strings.HasPrefix(src, "data:") {
		return ""
	}
	u, err := p.base.Parse(src)
	if err != nil {
		return ""
	}
	return u.String()
}

// LoadImages downloads images in the background (max 6 at a time) and calls
// onUpdate whenever one arrives.
func (p *Page) LoadImages(client *http.Client, onUpdate func()) {
	if !p.Opts.Images {
		return
	}
	urls := p.imageURLs()
	if len(urls) > 40 {
		urls = urls[:40]
	}
	if len(urls) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	sem := make(chan struct{}, 6)
	for _, u := range urls {
		if p.thumb(u) != nil {
			continue
		}
		go func(u string) {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			th := fetchThumb(ctx, client, u, p.URL)
			if th == nil || ctx.Err() != nil {
				return
			}
			p.mu.Lock()
			p.thumbs[u] = th
			p.mu.Unlock()
			if onUpdate != nil {
				onUpdate()
			}
		}(u)
	}
}

func fetchThumb(ctx context.Context, client *http.Client, u string, ref *url.URL) *Thumb {
	if strings.HasPrefix(u, "file://") {
		pu, err := url.Parse(u)
		if err != nil {
			return nil
		}
		f, err := os.Open(pu.Path)
		if err != nil {
			return nil
		}
		defer f.Close()
		th, err := decodeThumb(f)
		if err != nil {
			return nil
		}
		return th
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/gif,*/*;q=0.5")
	if ref != nil && ref.Scheme != "file" {
		req.Header.Set("Referer", ref.String())
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	th, err := decodeThumb(resp.Body)
	if err != nil {
		return nil
	}
	return th
}

// ---- DOM helpers ----

func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, k string) bool {
	for _, a := range n.Attr {
		if a.Key == k {
			return true
		}
	}
	return false
}

func findTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if r := findTag(c, tag); r != nil {
			return r
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func ancestor(n *html.Node, tag string) *html.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == tag {
			return p
		}
	}
	return nil
}
