package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// FindBrowser locates Chrome, Edge, Brave or Chromium.
func FindBrowser() (string, error) {
	if p := os.Getenv("ASCIIWEB_CHROME"); p != "" {
		return p, nil
	}
	var c []string
	switch runtime.GOOS {
	case "darwin":
		c = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "windows":
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")} {
			if base == "" {
				continue
			}
			c = append(c,
				filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(base, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"))
		}
	default:
		for _, n := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if p, err := exec.LookPath(n); err == nil {
				c = append(c, p)
			}
		}
	}
	for _, p := range c {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("Chrome, Edge or Brave not found (set ASCIIWEB_CHROME to its path)")
}

// Session is one headless browser whose screen arrives as frames.
type Session struct {
	ctx    context.Context
	cancel context.CancelFunc
	ops    chan func()

	mu     sync.Mutex
	frame  image.Image
	url    string
	title  string
	lastAt time.Time
	W, H   int
	set    Settings
	notify chan struct{}
}

// Start launches the browser at w x h CSS pixels and opens the home page.
func Start(set Settings, w, h int) (*Session, error) {
	exe, err := FindBrowser()
	if err != nil {
		return nil, err
	}
	profile := filepath.Join(ConfigDir(), "profile")
	os.MkdirAll(profile, 0o755)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(exe),
		chromedp.UserDataDir(profile),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-notifications", true),
		chromedp.Flag("lang", "ko-KR"),
		chromedp.WindowSize(w, h),
	)
	actx, cancelA := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelC := chromedp.NewContext(actx)
	s := &Session{ctx: ctx, W: w, H: h, set: set, ops: make(chan func(), 256), notify: make(chan struct{}, 1)}
	s.cancel = func() { cancelC(); cancelA() }

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *page.EventScreencastFrame:
			id := e.SessionID
			go chromedp.Run(ctx, page.ScreencastFrameAck(id))
			s.onFrame(e.Data)
		case *page.EventJavascriptDialogOpening:
			go chromedp.Run(ctx, page.HandleJavaScriptDialog(true))
		}
	})
	if err := chromedp.Run(ctx); err != nil { // starts the browser
		s.cancel()
		return nil, err
	}
	err = chromedp.Run(ctx,
		page.Enable(),
		// links that want a new tab open in this one
		chromedp.ActionFunc(func(c context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(`window.open=function(u){if(u)location.href=u;return null};
document.addEventListener('click',function(e){var a=e.target&&e.target.closest&&e.target.closest('a[target]');if(a)a.target='_self'},true);`).Do(c)
			return err
		}),
	)
	if err != nil {
		s.cancel()
		return nil, err
	}
	if err := s.Apply(set, w, h); err != nil {
		s.cancel()
		return nil, err
	}
	go s.worker()
	go s.poll()
	return s, nil
}

func (s *Session) onFrame(data string) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return
	}
	s.mu.Lock()
	gap := time.Second / time.Duration(s.set.FPS)
	skip := time.Since(s.lastAt) < gap
	if !skip {
		s.lastAt = time.Now()
	}
	s.mu.Unlock()
	if skip {
		return
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return
	}
	s.mu.Lock()
	s.frame = img
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *Session) worker() {
	for {
		select {
		case f := <-s.ops:
			f()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) poll() {
	t := time.NewTicker(700 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			var u, ti string
			cctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
			err := chromedp.Run(cctx, chromedp.Location(&u), chromedp.Title(&ti))
			cancel()
			if err == nil {
				s.mu.Lock()
				s.url, s.title = u, ti
				s.mu.Unlock()
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) do(a ...chromedp.Action) {
	select {
	case s.ops <- func() {
		cctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
		defer cancel()
		chromedp.Run(cctx, a...)
	}:
	default:
	}
}

// Apply sets the viewport size, user agent and frame quality and (re)starts
// the screencast. Call it whenever the size or those settings change.
func (s *Session) Apply(set Settings, w, h int) error {
	s.mu.Lock()
	s.set, s.W, s.H = set, w, h
	s.mu.Unlock()
	cctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
	defer cancel()
	return chromedp.Run(cctx,
		page.StopScreencast(),
		emulation.SetDeviceMetricsOverride(int64(w), int64(h), 1, set.Mobile),
		emulation.SetUserAgentOverride(userAgentOr(set)),
		page.StartScreencast().WithFormat(page.ScreencastFormatJpeg).WithQuality(int64(set.JPEG)).
			WithMaxWidth(int64(w)).WithMaxHeight(int64(h)),
	)
}

func userAgentOr(set Settings) string {
	if ua := set.UserAgent(); ua != "" {
		return ua
	}
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
}

// Frame returns the latest screen image (nil before the first one).
func (s *Session) Frame() image.Image {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frame
}

// Frames signals that a new frame arrived.
func (s *Session) Frames() <-chan struct{} { return s.notify }

func (s *Session) Info() (url, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.url, s.title
}

func (s *Session) Size() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.W, s.H
}

func (s *Session) Navigate(u string) { s.do(chromedp.Navigate(u)) }
func (s *Session) Back()             { s.do(chromedp.NavigateBack()) }
func (s *Session) Forward()          { s.do(chromedp.NavigateForward()) }
func (s *Session) Reload()           { s.do(chromedp.Reload()) }

// Click moves the pointer and clicks at CSS pixel (x, y).
func (s *Session) Click(x, y float64) {
	s.do(
		input.DispatchMouseEvent(input.MouseMoved, x, y),
		input.DispatchMouseEvent(input.MousePressed, x, y).WithButton(input.Left).WithClickCount(1),
		input.DispatchMouseEvent(input.MouseReleased, x, y).WithButton(input.Left).WithClickCount(1),
	)
}

func (s *Session) Move(x, y float64) { s.do(input.DispatchMouseEvent(input.MouseMoved, x, y)) }

func (s *Session) Wheel(x, y, dy float64) {
	s.do(input.DispatchMouseEvent(input.MouseWheel, x, y).WithDeltaY(dy))
}

// Key sends a named key (chromedp/kb constants) to the page.
func (s *Session) Key(k string) { s.do(chromedp.KeyEvent(k)) }

// Type sends printable text; ASCII goes through real key events, everything
// else is inserted directly (works for Korean and other IME text).
func (s *Session) Type(t string) {
	for _, r := range t {
		if r < 128 {
			s.do(chromedp.KeyEvent(string(r)))
		} else {
			s.do(input.InsertText(string(r)))
		}
	}
}

func (s *Session) Close() { s.cancel() }
