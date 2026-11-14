package com.hayoonjae.asciiweb.engine

import kotlin.math.abs

/** One visible word of the page with its box in viewport pixels (vw x vh is the viewport). */
class Word(
    val t: String, val x: Float, val y: Float, val w: Float, val h: Float,
    val r: Int, val g: Int, val b: Int, val vw: Float, val vh: Float,
)

/** Lists the visible words of the page; same script as the desktop version. */
const val TEXT_SCRIPT = """(function(){
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
})()"""

/** Dark text becomes light, colored text keeps its hue but is lifted, so it reads on a dark screen. */
private fun textColor(r: Int, g: Int, b: Int, color: Boolean): Int {
    if (!color) return 0
    val lum = (0.299f * r + 0.587f * g + 0.114f * b) / 255f
    val mx = maxOf(r, g, b)
    val mn = minOf(r, g, b)
    val sat = if (mx > 0) (mx - mn).toFloat() / mx else 0f
    if (lum < 0.5f) {
        if (sat < 0.25f) return rgb(238, 238, 245)
        fun mix(v: Int) = (v * 0.5f + 255 * 0.5f).toInt()
        return rgb(mix(r), mix(g), mix(b))
    }
    return rgb(r, g, b)
}

/** Writes the page's words onto the grid as real characters; the art under each word is cleared. */
fun overlayText(grid: List<Line>, words: List<Word>, s: LiveSettings): List<Line> {
    if (words.isEmpty() || grid.isEmpty()) return grid
    val rows = grid.size
    val cols = grid[0].size
    val out = Array(rows) { ArrayList(grid[it]) }
    val sorted = words.sortedWith { a, b ->
        if (abs(a.y - b.y) > a.h / 2) a.y.compareTo(b.y) else a.x.compareTo(b.x)
    }
    val rowLine = HashMap<Int, Float>()
    val cursor = HashMap<Int, Int>()
    val prevEnd = HashMap<Int, Float>()
    val blank = Cell(' '.code, Style())
    for (w in sorted) {
        if (w.vw <= 0 || w.vh <= 0) continue
        val r0 = (w.y / w.vh * rows).toInt()
        val r1 = ((w.y + w.h) / w.vh * rows).toInt()
        var row = ((w.y + w.h / 2) / w.vh * rows).toInt()
        if (row < 0 || row >= rows) continue
        val y = rowLine[row]
        if (y != null && abs(y - w.y) > w.h / 2) {
            row++
            if (row >= rows) continue
            val y2 = rowLine[row]
            if (y2 != null && abs(y2 - w.y) > w.h / 2) continue
        }
        rowLine[row] = w.y
        val c0 = (w.x / w.vw * cols).toInt()
        val c1 = ((w.x + w.w) / w.vw * cols).toInt()
        for (r in maxOf(r0, 0)..minOf(r1, rows - 1)) for (c in maxOf(c0, 0)..minOf(c1, cols - 1)) out[r][c] = blank
        var col = c0
        cursor[row]?.let { cur ->
            // words of one sentence are packed together; far-apart columns keep their place
            val pxPerChar = w.w / w.t.codePointCount(0, w.t.length)
            val gap = w.x - (prevEnd[row] ?: 0f)
            if (gap < 2.2f * pxPerChar || cur > col) col = cur
        }
        val st = Style(fg = textColor(w.r, w.g, w.b, s.color), bold = s.bold)
        var i = 0
        while (i < w.t.length) {
            val cp = w.t.codePointAt(i)
            i += Character.charCount(cp)
            val cw = runeWidth(cp)
            if (cw == 0) continue
            if (col < 0 || col + cw > cols) break
            out[row][col] = Cell(cp, st)
            if (cw == 2) out[row][col + 1] = Cell(0, st)
            col += cw
        }
        cursor[row] = col + 1
        prevEnd[row] = w.x + w.w
    }
    return out.toList()
}
