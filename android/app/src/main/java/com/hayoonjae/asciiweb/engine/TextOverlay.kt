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

/**
 * Writes the page's words onto the grid as real characters. Words are grouped into text lines, each line
 * owns one row, and the art under every word is cleared first so later clears never erase earlier text.
 */
fun overlayText(grid: List<Line>, words: List<Word>, s: LiveSettings): List<Line> {
    if (words.isEmpty() || grid.isEmpty()) return grid
    val rows = grid.size
    val cols = grid[0].size
    val out = Array(rows) { ArrayList(grid[it]) }
    val blank = Cell(' '.code, Style())
    fun cy(w: Word) = w.y + w.h / 2
    val ws = words.sortedBy { cy(it) }
    // pass 1: clear
    for (w in ws) {
        if (w.vw <= 0 || w.vh <= 0) continue
        val r0 = (w.y / w.vh * rows).toInt()
        val r1 = ((w.y + w.h) / w.vh * rows).toInt()
        val c0 = (w.x / w.vw * cols).toInt()
        val c1 = ((w.x + w.w) / w.vw * cols).toInt()
        for (r in maxOf(r0, 0)..minOf(r1, rows - 1)) for (c in maxOf(c0, 0)..minOf(c1, cols - 1)) out[r][c] = blank
    }
    // group into lines by vertical centre
    val lines = ArrayList<ArrayList<Word>>()
    val lineCy = ArrayList<Float>()
    for (w in ws) {
        val last = lines.lastOrNull()
        if (last != null && abs(cy(w) - lineCy.last()) <= 0.6f * maxOf(w.h, last[0].h)) last.add(w)
        else {
            lines.add(arrayListOf(w)); lineCy.add(cy(w))
        }
    }
    // pass 2: write
    val owner = HashSet<Int>()
    for ((li, line) in lines.withIndex()) {
        line.sortBy { it.x }
        val w0 = line[0]
        if (w0.vw <= 0 || w0.vh <= 0) continue
        val base = (lineCy[li] / w0.vh * rows).toInt()
        val row = listOf(base, base + 1, base - 1).firstOrNull { it in 0 until rows && it !in owner } ?: continue
        owner.add(row)
        var cursor = -1
        var prevEnd = 0f
        for (w in line) {
            val c0 = (w.x / w.vw * cols).toInt()
            var col = c0
            if (cursor >= 0) {
                // words of one sentence are packed together; far-apart columns keep their place
                val pxPerChar = w.w / w.t.codePointCount(0, w.t.length)
                if (w.x - prevEnd < 2.2f * pxPerChar || cursor > col) col = cursor
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
            cursor = col + 1
            prevEnd = w.x + w.w
        }
    }
    return out.toList()
}
