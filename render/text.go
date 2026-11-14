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
}

// TextScript lists the visible words of the page. It returns a JSON object
// {w, h, words:[{t,x,y,w,h,r,g,b}]}. The same script is used on Android.
const TextScript = `(function(){
var W=innerWidth,H=innerHeight,out=[],n=0;
var tw=document.createTreeWalker(document.body||document.documentElement,NodeFilter.SHOW_TEXT,null);
var rg=document.createRange(),node;
while((node=tw.nextNode())&&n<2500){
 var s=node.nodeValue; if(!s||!/\S/.test(s))continue;
 var el=node.parentElement; if(!el)continue;
 var tg=el.tagName; if(tg==='SCRIPT'||tg==='STYLE'||tg==='NOSCRIPT'||tg==='TEXTAREA')continue;
 rg.selectNodeContents(node); var bb=rg.getBoundingClientRect();
 if(bb.bottom<0||bb.top>H||bb.right<0||bb.left>W||bb.width===0)continue;
 var cs=getComputedStyle(el); if(cs.visibility==='hidden'||cs.display==='none'||+cs.opacity===0)continue;
 var m=/rgba?\((\d+),\s*(\d+),\s*(\d+)/.exec(cs.color)||[0,0,0,0];
 var re=/\S+/g,mm;
 while((mm=re.exec(s))&&n<2500){
  rg.setStart(node,mm.index); rg.setEnd(node,mm.index+mm[0].length);
  var r=rg.getBoundingClientRect(); if(r.width===0||r.bottom<0||r.top>H||r.right<0||r.left>W)continue;
  out.push({t:mm[0],x:r.left,y:r.top,w:r.width,h:r.height,r:+m[1],g:+m[2],b:+m[3]}); n++;
 }
}
return JSON.stringify({w:W,h:H,words:out});
})()`

// ParseWords decodes the result of TextScript.
func ParseWords(js string) []Word {
	var res struct {
		W, H  float64
		Words []Word `json:"words"`
	}
	if json.Unmarshal([]byte(js), &res) != nil {
		return nil
	}
	for i := range res.Words {
		res.Words[i].VW, res.Words[i].VH = res.W, res.H
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
// The art under each word is cleared so the text stays clean.
func OverlayText(grid []engine.Line, words []Word, s Settings, depth int) []engine.Line {
	if len(words) == 0 || len(grid) == 0 {
		return grid
	}
	rows, cols := len(grid), len(grid[0])
	color := depth != engine.NoColor && s.Color != "none"
	out := make([]engine.Line, rows)
	for i := range grid {
		out[i] = append(engine.Line(nil), grid[i]...)
	}
	sort.SliceStable(words, func(i, j int) bool {
		if math.Abs(words[i].Y-words[j].Y) > words[i].H/2 {
			return words[i].Y < words[j].Y
		}
		return words[i].X < words[j].X
	})
	rowLine := map[int]float64{} // row -> y of the text line that owns it
	cursor := map[int]int{}      // row -> next free column
	prevEnd := map[int]float64{} // row -> right edge (px) of the last word placed
	for _, w := range words {
		if w.VW <= 0 || w.VH <= 0 {
			continue
		}
		r0 := int(w.Y / w.VH * float64(rows))
		r1 := int((w.Y + w.H) / w.VH * float64(rows))
		row := int((w.Y + w.H/2) / w.VH * float64(rows))
		if row < 0 || row >= rows {
			continue
		}
		if y, used := rowLine[row]; used && math.Abs(y-w.Y) > w.H/2 {
			row++ // another line already owns this row: use the one below
			if row >= rows {
				continue
			}
			if y2, used2 := rowLine[row]; used2 && math.Abs(y2-w.Y) > w.H/2 {
				continue
			}
		}
		rowLine[row] = w.Y
		c0 := int(w.X / w.VW * float64(cols))
		c1 := int((w.X + w.W) / w.VW * float64(cols))
		// clear the art under the word's box
		for r := r0; r <= r1 && r < rows; r++ {
			if r < 0 {
				continue
			}
			for c := c0; c <= c1 && c < cols; c++ {
				if c >= 0 {
					out[r][c] = engine.Cell{R: ' '}
				}
			}
		}
		col := c0
		if cur, ok := cursor[row]; ok {
			// words of one sentence are packed together; far-apart columns keep their place
			pxPerChar := w.W / float64(len([]rune(w.T)))
			if gap := w.X - prevEnd[row]; gap < 2.2*pxPerChar {
				col = cur
			} else if cur > col {
				col = cur
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
		cursor[row] = col + 1
		prevEnd[row] = w.X + w.W
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
		time.Sleep(120 * time.Millisecond) // let rapid changes settle, coalesce kicks
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
