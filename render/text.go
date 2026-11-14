package render

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/chromedp/chromedp"

	"asciiweb/engine"
)

// Word is one visible word of the page with its box in viewport pixels.
type Word struct {
	T  string  `json:"t"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	W  float64 `json:"w"`
	H  float64 `json:"h"`
	R  int     `json:"r"`
	G  int     `json:"g"`
	B  int     `json:"b"`
	VW float64 `json:"-"`
	VH float64 `json:"-"`
	SY float64 `json:"-"` // page scroll (CSS px) when the word was read
}

// TextScript lists the visible words of the page. It returns a JSON object
// {w, h, words:[{t,x,y,w,h,r,g,b}]}. The same script is used on Android.
const TextScript = `(function(){
var W=innerWidth,H=innerHeight,out=[],n=0;
var tw=document.createTreeWalker(document.body||document.documentElement,NodeFilter.SHOW_TEXT,null);
var rg=document.createRange(),node;
while((node=tw.nextNode())&&n<1500){
 var s=node.nodeValue; if(!s||!/\S/.test(s))continue;
 var el=node.parentElement; if(!el)continue;
 var tg=el.tagName; if(tg==='SCRIPT'||tg==='STYLE'||tg==='NOSCRIPT'||tg==='TEXTAREA')continue;
 rg.selectNodeContents(node); var bb=rg.getBoundingClientRect();
 if(bb.bottom<0||bb.top>H||bb.right<0||bb.left>W||bb.width===0)continue;
 var cs=getComputedStyle(el); if(cs.visibility==='hidden'||cs.display==='none'||+cs.opacity===0)continue;
 var m=/rgba?\((\d+),\s*(\d+),\s*(\d+)/.exec(cs.color)||[0,0,0,0];
 var re=/\S+/g,mm;
 while((mm=re.exec(s))&&n<1500){
  rg.setStart(node,mm.index); rg.setEnd(node,mm.index+mm[0].length);
  var r=rg.getBoundingClientRect(); if(r.width===0||r.bottom<0||r.top>H||r.right<0||r.left>W)continue;
  out.push({t:mm[0],x:r.left,y:r.top,w:r.width,h:r.height,r:+m[1],g:+m[2],b:+m[3]}); n++;
 }
}
return JSON.stringify({w:W,h:H,sy:(window.pageYOffset||0),words:out});
})()`

// ParseWords decodes the result of TextScript.
func ParseWords(js string) []Word {
	var res struct {
		W, H, SY float64
		Words    []Word `json:"words"`
	}
	if json.Unmarshal([]byte(js), &res) != nil {
		return nil
	}
	for i := range res.Words {
		res.Words[i].VW, res.Words[i].VH, res.Words[i].SY = res.W, res.H, res.SY
	}
	return res.Words
}

// textColor makes page text readable on a dark terminal: dark text becomes
// light, colored text keeps its hue but is lifted.
func textColor(r, g, b int, color bool) uint32 {
	if !color {
		return 0
	}
	lum := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255
	mx, mn := r, r
	for _, v := range []int{g, b} {
		if v > mx {
			mx = v
		}
		if v < mn {
			mn = v
		}
	}
	sat := 0.0
	if mx > 0 {
		sat = float64(mx-mn) / float64(mx)
	}
	if lum < 0.5 {
		if sat < 0.25 {
			return engine.Rgb(238, 238, 245)
		}
		f := 0.5
		return engine.Rgb(uint8(float64(r)*(1-f)+255*f), uint8(float64(g)*(1-f)+255*f), uint8(float64(b)*(1-f)+255*f))
	}
	return engine.Rgb(uint8(r), uint8(g), uint8(b))
}

// OverlayText writes the page's words onto the grid as real characters.
// Words are grouped into text lines first, each line owns one grid row, and
// the art under each word is cleared so the text stays clean.
func OverlayText(grid []engine.Line, words []Word, s Settings, depth int, scrollY float64) []engine.Line {
	if len(words) == 0 || len(grid) == 0 {
		return grid
	}
	rows, cols := len(grid), len(grid[0])
	color := depth != engine.NoColor && s.Color != "none"
	out := make([]engine.Line, rows)
	for i := range grid {
		out[i] = append(engine.Line(nil), grid[i]...)
	}
	cy := func(w Word) float64 { return w.Y + w.H/2 }
	ws := append([]Word(nil), words...)
	// words were read at an older scroll position: slide them to where the page is now
	for i := range ws {
		ws[i].Y += ws[i].SY - scrollY
	}
	sort.Slice(ws, func(i, j int) bool { return cy(ws[i]) < cy(ws[j]) })
	// group into lines by vertical centre
	var lines [][]Word
	var lineCy []float64
	for _, w := range ws {
		n := len(lines)
		if n > 0 && math.Abs(cy(w)-lineCy[n-1]) <= 0.6*math.Max(w.H, lines[n-1][0].H) {
			lines[n-1] = append(lines[n-1], w)
			continue
		}
		lines = append(lines, []Word{w})
		lineCy = append(lineCy, cy(w))
	}
	// pass 1: clear the art under every word (so later clears never erase earlier text)
	for _, w := range ws {
		if w.VW <= 0 || w.VH <= 0 {
			continue
		}
		r0, r1 := int(w.Y/w.VH*float64(rows)), int((w.Y+w.H)/w.VH*float64(rows))
		c0, c1 := int(w.X/w.VW*float64(cols)), int((w.X+w.W)/w.VW*float64(cols))
		for r := r0; r <= r1 && r < rows; r++ {
			for c := c0; c <= c1 && c < cols; c++ {
				if r >= 0 && c >= 0 {
					out[r][c] = engine.Cell{R: ' '}
				}
			}
		}
	}
	// pass 2: write the text
	owner := map[int]bool{}
	for li, line := range lines {
		sort.Slice(line, func(i, j int) bool { return line[i].X < line[j].X })
		w0 := line[0]
		if w0.VW <= 0 || w0.VH <= 0 {
			continue
		}
		row := int(lineCy[li] / w0.VH * float64(rows))
		pick := -1
		for _, r := range []int{row, row + 1, row - 1} {
			if r >= 0 && r < rows && !owner[r] {
				pick = r
				break
			}
		}
		if pick < 0 {
			continue
		}
		row = pick
		owner[row] = true
		cursor, prevEnd := -1, 0.0
		for _, w := range line {
			c0 := int(w.X / w.VW * float64(cols))
			col := c0
			if cursor >= 0 {
				// words of one sentence are packed together; far-apart columns keep their place
				pxPerChar := w.W / float64(len([]rune(w.T)))
				if gap := w.X - prevEnd; gap < 2.2*pxPerChar || cursor > col {
					col = cursor
				}
			}
			st := engine.Style{FG: textColor(w.R, w.G, w.B, color), Bold: s.Bold}
			for _, ch := range w.T {
				cw := engine.RuneWidth(ch)
				if cw == 0 {
					continue
				}
				if col < 0 || col+cw > cols {
					break
				}
				out[row][col] = engine.Cell{R: ch, S: st}
				if cw == 2 {
					out[row][col+1] = engine.Cell{R: 0, S: st}
				}
				col += cw
			}
			cursor, prevEnd = col+1, w.X+w.W
		}
	}
	return out
}

// textLoop fetches the visible words whenever the screen changes.
func (s *Session) textLoop(kick <-chan struct{}) {
	t := time.NewTicker(1500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-kick:
		case <-t.C:
		case <-s.ctx.Done():
			return
		}
		time.Sleep(150 * time.Millisecond) // coalesce kicks; text positions are corrected by the scroll offset
		for len(kick) > 0 {
			<-kick
		}
		var res string
		cctx, cancel := context.WithTimeout(s.ctx, 4*time.Second)
		err := chromedp.Run(cctx, chromedp.Evaluate(TextScript, &res))
		cancel()
		if err != nil {
			continue
		}
		ws := ParseWords(res)
		s.mu.Lock()
		s.words = ws
		s.mu.Unlock()
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
}

// Words returns the latest visible words.
func (s *Session) Words() []Word {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Word(nil), s.words...)
}
