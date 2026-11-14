package main

import (
	"flag"
	"fmt"
	"os"

	"asciiweb/engine"
)

func main() {
	dumpMode := flag.Bool("dump", false, "render one page to stdout and exit")
	plain := flag.Bool("plain", false, "with -dump: no ANSI colors")
	cols := flag.Int("cols", 80, "with -dump: width")
	noImg := flag.Bool("no-images", false, "do not download images")
	noColor := flag.Bool("no-color", false, "monochrome")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: asciiweb [flags] [URL | search words | file]")
		flag.PrintDefaults()
	}
	flag.Parse()
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
