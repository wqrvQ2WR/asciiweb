package com.hayoonjae.asciiweb.engine

class Tok(val cells: ArrayList<Cell>, val space: Boolean = false, val br: Boolean = false)

const val ALIGN_LEFT = 0
const val ALIGN_CENTER = 1
const val ALIGN_RIGHT = 2

fun aligned(l: Line, w: Int, align: Int): Line {
    val t = trimRight(l)
    if (align == ALIGN_LEFT || t.size >= w) return t
    var pad = w - t.size
    if (align == ALIGN_CENTER) pad /= 2
    val out = ArrayList<Cell>(pad + t.size)
    out.addAll(spaces(pad))
    out.addAll(t)
    return out
}

/** Breaks inline tokens into lines of at most w cells. */
fun wrap(toks: List<Tok>, w0: Int, align: Int): List<Line> {
    val w = maxOf(1, w0)
    // adjacent pieces without a space between them are one word
    val merged = ArrayList<Tok>(toks.size)
    for (t in toks) {
        val last = merged.lastOrNull()
        if (last != null && !t.space && !t.br && !last.space && !last.br) {
            val nc = ArrayList<Cell>(last.cells.size + t.cells.size)
            nc.addAll(last.cells)
            nc.addAll(t.cells)
            merged[merged.size - 1] = Tok(nc)
        } else merged.add(t)
    }
    val out = ArrayList<Line>()
    var cur = ArrayList<Cell>()
    var pend: List<Cell> = emptyList()
    fun emit() {
        out.add(aligned(cur, w, align))
        cur = ArrayList()
        pend = emptyList()
    }
    for (t in merged) {
        when {
            t.br -> emit()
            t.space -> if (cur.isNotEmpty()) pend = t.cells
            else -> {
                var c: List<Cell> = t.cells
                if (cur.isNotEmpty() && cur.size + pend.size + c.size > w) emit()
                if (cur.isNotEmpty()) cur.addAll(pend)
                pend = emptyList()
                while (c.size > w) {
                    var cut = w
                    if (c[cut].r == 0) cut--
                    if (cut <= 0) cut = minOf(c.size, 2)
                    if (cur.isNotEmpty()) emit()
                    cur.addAll(c.subList(0, cut))
                    emit()
                    c = c.subList(cut, c.size)
                }
                cur.addAll(c)
            }
        }
    }
    if (cur.isNotEmpty()) emit()
    return out
}
