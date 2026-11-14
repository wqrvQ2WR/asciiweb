package engine

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const UserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 AsciiWeb/0.1"

const maxBody = 4 << 20

// Browser holds the HTTP client and the history. It is not safe for
// concurrent use except Fetch, which may run on a goroutine; Commit and the
// history calls belong to the UI thread.
type Browser struct {
	Opts   Options
	Client *http.Client
	Cur    *Page

	hist []hent
	pos  int
}

type hent struct {
	URL    string
	Method string
	Body   string
	Scroll int
	Page   *Page
}

func NewBrowser() *Browser {
	jar, _ := cookiejar.New(nil)
	return &Browser{
		Opts:   Options{Images: true, Color: true},
		Client: &http.Client{Timeout: 25 * time.Second, Jar: jar},
		pos:    -1,
	}
}

// Normalize turns what the user typed into a URL: a real URL, a file, or a
// web search.
func Normalize(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "about:home"
	case strings.HasPrefix(s, "about:"):
		return s
	case strings.Contains(s, "://"):
		return s
	case strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || strings.HasPrefix(s, "~/"):
		if strings.HasPrefix(s, "~/") {
			if h, err := os.UserHomeDir(); err == nil {
				s = filepath.Join(h, s[2:])
			}
		}
		abs, err := filepath.Abs(s)
		if err != nil {
			abs = s
		}
		return "file://" + filepath.ToSlash(abs)
	case fileExists(s):
		return Normalize("./" + s)
	case !strings.ContainsAny(s, " \t") && (strings.Contains(s, ".") || strings.HasPrefix(s, "localhost")):
		return "https://" + s
	}
	return "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(s)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Fetch loads a page without touching the browser state.
func (b *Browser) Fetch(method, raw, body string) (*Page, error) {
	if strings.HasPrefix(raw, "about:") {
		return b.about(raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	var data []byte
	ct := ""
	final := u
	switch u.Scheme {
	case "file":
		path := u.Path
		data, err = readLimited(mustOpen(path))
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".html", ".htm", ".xhtml":
			ct = "text/html"
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
			ct = "image/" + strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		default:
			ct = "text/plain"
		}
	case "http", "https":
		var req *http.Request
		if method == "POST" {
			req, err = http.NewRequest("POST", raw, strings.NewReader(body))
			if req != nil {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		} else {
			req, err = http.NewRequest("GET", raw, nil)
		}
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,image/*;q=0.8,*/*;q=0.5")
		req.Header.Set("Accept-Language", "ko,en;q=0.8")
		resp, err := b.Client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		final = resp.Request.URL
		ct = strings.ToLower(resp.Header.Get("Content-Type"))
		data, err = readLimited(resp.Body, nil)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 && len(bytes.TrimSpace(data)) == 0 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if ct == "" || strings.HasPrefix(ct, "application/octet-stream") {
		ct = strings.ToLower(http.DetectContentType(data))
	}
	var root *nethtml.Node
	switch {
	case strings.HasPrefix(ct, "image/"):
		root, _ = nethtml.Parse(strings.NewReader(`<html><body><img src="` + html.EscapeString(final.String()) + `"></body></html>`))
	case strings.Contains(ct, "html") || strings.Contains(ct, "xml"):
		r, err := charset.NewReader(bytes.NewReader(data), ct)
		if err != nil {
			r = bytes.NewReader(data)
		}
		root, err = nethtml.Parse(r)
		if err != nil {
			return nil, err
		}
	case strings.HasPrefix(ct, "text/"):
		r, err := charset.NewReader(bytes.NewReader(data), ct)
		if err != nil {
			r = bytes.NewReader(data)
		}
		txt, _ := io.ReadAll(r)
		root, _ = nethtml.Parse(strings.NewReader("<html><body><pre>" + html.EscapeString(string(txt)) + "</pre></body></html>"))
	default:
		return nil, fmt.Errorf("unsupported content type: %s", ct)
	}
	p := newPage(final, root, &b.Opts)
	p.Method, p.Body = method, body
	return p, nil
}

func mustOpen(path string) (io.Reader, func()) {
	f, err := os.Open(path)
	if err != nil {
		return errReader{err}, func() {}
	}
	return f, func() { f.Close() }
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func readLimited(r io.Reader, closeFn func()) ([]byte, error) {
	if closeFn != nil {
		defer closeFn()
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBody))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Commit makes p the current page. When push is true it is added to the
// history, otherwise it replaces the current entry (reload).
func (b *Browser) Commit(p *Page, push bool, scroll int) {
	if b.Cur != nil {
		b.Cur.Stop()
		if b.pos >= 0 && b.pos < len(b.hist) {
			b.hist[b.pos].Scroll = scroll
		}
	}
	e := hent{URL: p.URL.String(), Method: p.Method, Body: p.Body, Page: p}
	if push || b.pos < 0 {
		b.hist = append(b.hist[:b.pos+1], e)
		b.pos = len(b.hist) - 1
		if b.pos > 3 { // keep only recent GET pages in memory
			if old := &b.hist[b.pos-4]; old.Method != "POST" {
				old.Page = nil
			}
		}
	} else {
		b.hist[b.pos] = e
	}
	b.Cur = p
}

// Step moves in the history. It returns the target entry, or false.
func (b *Browser) Step(d int) (url string, method, body string, cached *Page, scroll int, ok bool) {
	n := b.pos + d
	if n < 0 || n >= len(b.hist) {
		return "", "", "", nil, 0, false
	}
	if b.Cur != nil && b.pos >= 0 {
		// scroll of the page we are leaving is saved by Commit
	}
	e := b.hist[n]
	return e.URL, e.Method, e.Body, e.Page, e.Scroll, true
}

// MoveTo sets the history position after a successful Step.
func (b *Browser) MoveTo(d int, p *Page, scrollLeaving int) {
	if b.pos >= 0 && b.pos < len(b.hist) {
		b.hist[b.pos].Scroll = scrollLeaving
	}
	if b.Cur != nil {
		b.Cur.Stop()
	}
	b.pos += d
	b.hist[b.pos].Page = p
	b.Cur = p
}

func (b *Browser) CanBack() bool    { return b.pos > 0 }
func (b *Browser) CanForward() bool { return b.pos >= 0 && b.pos < len(b.hist)-1 }

// SubmitURL computes the request for submitting a form.
func (p *Page) SubmitURL(form *nethtml.Node, submit *nethtml.Node) (method, target, body string) {
	vals := p.FormValues(form, submit).Encode()
	action := attr(form, "action")
	base := p.base
	u, err := base.Parse(action)
	if err != nil {
		u = base
	}
	if strings.ToLower(attr(form, "method")) == "post" {
		return "POST", u.String(), vals
	}
	u.RawQuery = vals
	u.Fragment = ""
	return "GET", u.String(), ""
}

var ErrNoPage = errors.New("no page")
