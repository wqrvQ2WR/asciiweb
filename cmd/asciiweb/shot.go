package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"asciiweb/engine"
	"asciiweb/render"
)

// shotFrame opens a page in the real browser and prints one converted frame.
func shotFrame(set render.Settings, input string, cols int) error {
	rows := cols * 9 / 16 / 2 * 2 // 16:9-ish page on 1:2 cells
	if rows < 10 {
		rows = 10
	}
	w := set.PageWidth
	if w == 0 {
		w = cols * 5
		if w < 360 {
			w = 360
		}
	}
	h := 2 * w * rows / cols
	s, err := render.Start(set, w, h)
	if err != nil {
		return err
	}
	defer s.Close()
	target := set.Home
	if input != "" {
		target = engine.Normalize(input)
	}
	s.Navigate(target)
	deadline := time.After(15 * time.Second)
	var last time.Time
	for {
		select {
		case <-s.Frames():
			last = time.Now()
		case <-time.After(400 * time.Millisecond):
			if !last.IsZero() && time.Since(last) > 1500*time.Millisecond {
				goto done
			}
		case <-deadline:
			goto done
		}
	}
done:
	img := s.Frame()
	if img == nil {
		return fmt.Errorf("no frame received")
	}
	depth := engine.NoColor
	if os.Getenv("SHOT_COLOR") != "" {
		depth = engine.TrueColor
	}
	grid := render.Convert(img, cols, rows, set, depth)
	if set.Text {
		time.Sleep(2 * time.Second)
		grid = render.OverlayText(grid, s.Words(), set, depth, s.FrameScroll())
	}
	for _, l := range grid {
		if depth == engine.NoColor {
			fmt.Println(strings.TrimRight(l.String(), " "))
		} else {
			fmt.Println(renderLine(l, cols, -1, depth))
		}
	}
	u, t := s.Info()
	fmt.Fprintln(os.Stderr, "page:", t, u)
	return nil
}
