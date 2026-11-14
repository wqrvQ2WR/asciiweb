package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"asciiweb/engine"
)

// dump renders one page to stdout and exits (also handy for scripts and tests).
func dump(b *engine.Browser, input string, cols int, ansi bool) error {
	p, err := b.Fetch("GET", engine.Normalize(input), "")
	if err != nil {
		return err
	}
	done := make(chan struct{}, 64)
	p.LoadImages(b.Client, func() { done <- struct{}{} })
	if b.Opts.Images {
		deadline := time.After(12 * time.Second)
	wait:
		for {
			select {
			case <-done:
			case <-time.After(1500 * time.Millisecond):
				break wait
			case <-deadline:
				break wait
			}
		}
	}
	p.Layout(cols)
	var sb strings.Builder
	for _, l := range p.Lines {
		if ansi {
			sb.WriteString(renderLine(l, cols, -1, engine.TrueColor))
		} else {
			sb.WriteString(l.String())
		}
		sb.WriteByte('\n')
	}
	fmt.Fprint(os.Stdout, sb.String())
	return nil
}
