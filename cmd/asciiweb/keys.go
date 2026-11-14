package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type key struct {
	name string // "", or enter/esc/tab/btab/bs/up/down/... /wheelup/wheeldown/click
	r    rune
	x, y int
}

func readKeys(out chan<- key) {
	rd := bufio.NewReaderSize(os.Stdin, 512)
	for {
		r, _, err := rd.ReadRune()
		if err != nil {
			out <- key{name: "eof"}
			return
		}
		switch r {
		case 0x1b:
			if rd.Buffered() == 0 {
				out <- key{name: "esc"}
				continue
			}
			r2, _, _ := rd.ReadRune()
			if r2 != '[' && r2 != 'O' {
				continue // alt+key: ignored
			}
			var params []byte
			var final byte
			for {
				b, err := rd.ReadByte()
				if err != nil {
					break
				}
				if b >= 0x40 && b <= 0x7e {
					final = b
					break
				}
				params = append(params, b)
			}
			if k, ok := csiKey(string(params), final); ok {
				out <- k
			}
		case '\r', '\n':
			out <- key{name: "enter"}
		case '\t':
			out <- key{name: "tab"}
		case 0x7f, 8:
			out <- key{name: "bs"}
		case 3:
			out <- key{name: "ctrlc"}
		case 21:
			out <- key{name: "ctrlu"}
		default:
			if r >= 32 {
				out <- key{r: r}
			}
		}
	}
}

func csiKey(params string, final byte) (key, bool) {
	if strings.HasPrefix(params, "<") { // SGR mouse
		f := strings.Split(params[1:], ";")
		if len(f) != 3 {
			return key{}, false
		}
		b, _ := strconv.Atoi(f[0])
		x, _ := strconv.Atoi(f[1])
		y, _ := strconv.Atoi(f[2])
		switch {
		case b == 64:
			return key{name: "wheelup", x: x, y: y}, true
		case b == 65:
			return key{name: "wheeldown", x: x, y: y}, true
		case b == 0 && final == 'M':
			return key{name: "click", x: x, y: y}, true
		}
		return key{}, false
	}
	switch final {
	case 'A':
		return key{name: "up"}, true
	case 'B':
		return key{name: "down"}, true
	case 'C':
		return key{name: "right"}, true
	case 'D':
		return key{name: "left"}, true
	case 'H':
		return key{name: "home"}, true
	case 'F':
		return key{name: "end"}, true
	case 'Z':
		return key{name: "btab"}, true
	case '~':
		switch params {
		case "1", "7":
			return key{name: "home"}, true
		case "4", "8":
			return key{name: "end"}, true
		case "3":
			return key{name: "del"}, true
		case "5":
			return key{name: "pgup"}, true
		case "6":
			return key{name: "pgdn"}, true
		}
	}
	return key{}, false
}
