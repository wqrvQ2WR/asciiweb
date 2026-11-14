// Package render drives a real browser (Chrome/Edge over CDP) and turns its
// screen into characters.
package render

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Settings are the user's browser, quality and look options.
type Settings struct {
	Style     string `json:"style"`      // ascii, detail, blocks, braille
	Color     string `json:"color"`      // auto, none, 256, true
	Invert    string `json:"invert"`     // auto, on, off
	Contrast  int    `json:"contrast"`   // -4..4
	Bold      bool   `json:"bold"`       // draw characters in bold
	Text      bool   `json:"text"`       // draw page text as real characters
	PageWidth int    `json:"page_width"` // CSS pixels the page lays out at
	JPEG      int    `json:"jpeg"`       // frame quality 30..95
	FPS       int    `json:"fps"`        // max frames per second
	Mobile    bool   `json:"mobile"`     // mobile user agent
	Search    string `json:"search"`     // duckduckgo, google, naver, bing
	Home      string `json:"home"`
}

func Default() Settings {
	return Settings{Bold: true, Text: true, Style: "ascii", Color: "auto", Invert: "auto", PageWidth: 0, JPEG: 60, FPS: 6, Search: "duckduckgo", Home: "https://duckduckgo.com"}
}

// Option lists shown in the settings screen.
var (
	Styles    = []string{"ascii", "detail", "blocks", "braille"}
	Colors    = []string{"auto", "none", "256", "true"}
	Inverts   = []string{"auto", "on", "off"}
	Widths    = []int{0, 360, 480, 640, 800, 1024, 1280, 1600}
	JPEGs     = []int{30, 50, 70, 85, 95}
	FPSes     = []int{3, 5, 10, 15, 24}
	Searches  = []string{"duckduckgo", "google", "naver", "bing"}
	StyleDesc = map[string]string{
		"ascii":   "ASCII characters by brightness",
		"detail":  "ASCII, 70 level ramp",
		"blocks":  "half blocks, true pixels",
		"braille": "braille dots, 2x4 per cell",
	}
)

func (s *Settings) Normalize() {
	d := Default()
	if !contains(Styles, s.Style) {
		s.Style = d.Style
	}
	if !contains(Colors, s.Color) {
		s.Color = d.Color
	}
	if !contains(Inverts, s.Invert) {
		s.Invert = d.Invert
	}
	if !contains(Searches, s.Search) {
		s.Search = d.Search
	}
	s.Contrast = clamp(s.Contrast, -4, 4)
	if s.PageWidth != 0 {
		s.PageWidth = clamp(s.PageWidth, 320, 1920)
	}
	s.JPEG = clamp(s.JPEG, 20, 100)
	s.FPS = clamp(s.FPS, 1, 30)
	if strings.TrimSpace(s.Home) == "" {
		s.Home = d.Home
	}
}

func contains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "asciiweb")
}

// ConfigDir is where settings and the browser profile live.
func ConfigDir() string { return path() }

func Load() Settings {
	s := Default()
	if b, err := os.ReadFile(filepath.Join(path(), "settings.json")); err == nil {
		json.Unmarshal(b, &s)
	}
	s.Normalize()
	return s
}

func (s Settings) Save() error {
	if err := os.MkdirAll(path(), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(path(), "settings.json"), b, 0o644)
}

// SearchURL builds a search for the chosen engine.
func (s Settings) SearchURL(q string) string {
	e := url.QueryEscape(q)
	switch s.Search {
	case "google":
		return "https://www.google.com/search?q=" + e
	case "naver":
		return "https://search.naver.com/search.naver?query=" + e
	case "bing":
		return "https://www.bing.com/search?q=" + e
	}
	return "https://duckduckgo.com/?q=" + e
}

// UserAgent for the chosen device type.
func (s Settings) UserAgent() string {
	if s.Mobile {
		return "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Mobile Safari/537.36"
	}
	return ""
}
