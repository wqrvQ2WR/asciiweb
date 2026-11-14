//go:build !windows

package main

import (
	"os"

	"golang.org/x/term"
)

func initTerm() (func(), error) {
	fd := int(os.Stdin.Fd())
	st, err := term.MakeRaw(fd)
	if err != nil {
		return func() {}, err
	}
	return func() { term.Restore(fd, st) }, nil
}

func termSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		return 80, 24
	}
	return w, h
}

func defaultDepth(env func(string) string) int {
	ct := env("COLORTERM")
	if ct == "truecolor" || ct == "24bit" || env("WT_SESSION") != "" || env("TERM_PROGRAM") == "iTerm.app" || env("TERM_PROGRAM") == "WezTerm" || env("TERM_PROGRAM") == "vscode" {
		return 2
	}
	return 1
}
