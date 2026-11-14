package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"asciiweb/engine"
	"asciiweb/render"
)

func main() {
	dumpMode := flag.Bool("dump", false, "render one page to stdout and exit")
	plain := flag.Bool("plain", false, "with -dump: no ANSI colors")
	cols := flag.Int("cols", 80, "with -dump: width")
	noImg := flag.Bool("no-images", false, "do not download images")
	noColor := flag.Bool("no-color", false, "monochrome")
	textMode := flag.Bool("text", false, "lightweight text mode (re-flows pages, no browser needed)")
	shot := flag.Bool("shot", false, "render one real-browser frame to stdout and exit")
	width := flag.Int("width", 0, "page width in CSS pixels (overrides the saved setting)")
	style := flag.String("style", "", "ascii, detail, blocks, braille (overrides the saved setting)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: asciiweb [flags] [URL | search words | file]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if !*textMode && !*dumpMode {
		set := render.Load()
		if *style != "" {
			set.Style = *style
		}
		if *noColor {
			set.Color = "none"
		}
		if *width > 0 {
			set.PageWidth = *width
		}
		set.Normalize()
		input := strings.Join(flag.Args(), " ")
		if *shot {
			if err := shotFrame(set, input, *cols); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			return
		}
		if _, err := render.FindBrowser(); err == nil {
			if err := runLive(set, input); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			return
		}
		fmt.Fprintln(os.Stderr, "no Chrome/Edge found: using text mode (install Chrome for real page rendering)")
	}
	b := engine.NewBrowser()
	b.Opts.Images = !*noImg
	b.Opts.Color = !*noColor && !*plain
	input := ""
	if flag.NArg() > 0 {
		input = flag.Arg(0)
		for _, a := range flag.Args()[1:] {
			input += " " + a
		}
	}
	if *dumpMode {
		if err := dump(b, input, *cols, !*plain); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	runTUI(b, input)
}
