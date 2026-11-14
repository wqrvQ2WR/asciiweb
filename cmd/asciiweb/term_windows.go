//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

func initTerm() (func(), error) {
	in := windows.Handle(os.Stdin.Fd())
	out := windows.Handle(os.Stdout.Fd())
	fd := int(os.Stdin.Fd())
	st, err := term.MakeRaw(fd)
	if err != nil {
		return func() {}, err
	}
	var om uint32
	windows.GetConsoleMode(out, &om)
	windows.SetConsoleMode(out, om|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	var im uint32
	windows.GetConsoleMode(in, &im)
	windows.SetConsoleMode(in, im|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	windows.SetConsoleOutputCP(65001)
	return func() {
		term.Restore(fd, st)
		windows.SetConsoleMode(out, om)
	}, nil
}

func termSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		return 80, 24
	}
	return w, h
}

func defaultDepth(env func(string) string) int { return 2 }
