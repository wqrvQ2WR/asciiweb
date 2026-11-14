package com.hayoonjae.asciiweb.engine

import org.jsoup.nodes.Element

private class TCell(val n: Element, val span: Int, val th: Boolean)

fun Ctx.table(n: Element, st: Style, w: Int): List<Line> {
    val rows = ArrayList<List<TCell>>()
    var caption: Element? = null
    fun collect(p: Element) {
        for (ch in p.children()) {
            if (isHidden(ch)) continue
            when (ch.tagName()) {
                "tr" -> {
                    val row = ch.children().filter { (it.tagName() == "td" || it.tagName() == "th") && !isHidden(it) }
                        .map { TCell(it, attrInt(it, "colspan").coerceIn(1, 20), it.tagName() == "th") }
                    if (row.isNotEmpty()) rows.add(row)
                }
                "thead", "tbody", "tfoot" -> collect(ch)
                "caption" -> caption = ch
            }
        }
    }
    collect(n)
    if (rows.isEmpty()) return emptyList()
    val ncols = rows.maxOf { r -> r.sumOf { it.span } }
    val b = n.attr("border")
    val bordered = b.isNotEmpty() && (b.toIntOrNull()?.let { it > 0 } ?: true)
    val out = ArrayList<Line>()
    caption?.let { out.addAll(trimBlank(layout(it, st, w, ALIGN_CENTER))) }
    val sep = if (bordered) 3 else 2
    val edge = if (bordered) 4 else 0
    val avail = w - edge - sep * (ncols - 1)

    fun cellStyle(c: TCell) = if (c.th) st.copy(bold = true) else st

    if (ncols == 1 || avail < ncols * 2 || depth > 14) {
        for (r in rows) for (c in r) {
            val a = alignOf(c.n).let { if (it >= 0) it else ALIGN_LEFT }
            out.addAll(trimBlank(layout(c.n, cellStyle(c), w, a)))
        }
        return out
    }

    // measure natural and minimum widths of single-span cells
    val nat = IntArray(ncols)
    val minw = IntArray(ncols)
    for (r in rows) {
        var col = 0
        for (c in r) {
            if (c.span == 1) {
                val t0 = targets.size
                val ls = layout(c.n, st, 300, ALIGN_LEFT)
                while (targets.size > t0) targets.removeAt(targets.size - 1)
                nat[col] = maxOf(nat[col], maxWidth(ls))
                minw[col] = maxOf(minw[col], longestWord(c.n))
            }
            col += c.span
        }
    }
    var total = 0
    var summin = 0
    for (i in 0 until ncols) {
        if (nat[i] < 1) nat[i] = 1
        if (minw[i] > nat[i]) minw[i] = nat[i]
        if (minw[i] < 1) minw[i] = 1
        total += nat[i]
        summin += minw[i]
    }
    val widths = IntArray(ncols)
    when {
        total <= avail -> for (i in 0 until ncols) widths[i] = nat[i]
        summin >= avail -> for (i in 0 until ncols) widths[i] = maxOf(1, avail * minw[i] / summin)
        else -> {
            val extra = avail - summin
            val need = total - summin
            for (i in 0 until ncols) widths[i] = minw[i] + extra * (nat[i] - minw[i]) / need
        }
    }

    val bs = Style(fg = COL_RULE)
    fun rule(l: String, m: String, r: String): Line {
        val sb = StringBuilder(l)
        for ((i, cw) in widths.withIndex()) {
            sb.append("─".repeat(cw + 2))
            if (i < widths.size - 1) sb.append(m)
        }
        sb.append(r)
        return cellsOf(sb.toString(), bs)
    }
    if (bordered) out.add(rule("┌", "┬", "┐"))
    var first = true
    for (r in rows) {
        val parts = ArrayList<List<Line>>()
        val pw = ArrayList<Int>()
        var col = 0
        var height = 0
        for (c in r) {
            var cw = (c.span - 1) * sep
            for (k in col until minOf(col + c.span, ncols)) cw += widths[k]
            val a = alignOf(c.n).let { if (it >= 0) it else ALIGN_LEFT }
            val ls = trimBlank(layout(c.n, cellStyle(c), cw, a))
            parts.add(ls)
            pw.add(cw)
            height = maxOf(height, ls.size)
            col += c.span
        }
        if (height == 0) continue
        if (bordered && !first) out.add(rule("├", "┼", "┤"))
        first = false
        for (y in 0 until height) {
            val l = ArrayList<Cell>()
            if (bordered) l.addAll(cellsOf("│ ", bs))
            for ((i, ls) in parts.withIndex()) {
                val cl: Line = if (y < ls.size) ls[y] else emptyList()
                l.addAll(cl)
                val pad = pw[i] - cl.size
                if (pad > 0) l.addAll(spaces(pad))
                if (i < parts.size - 1) l.addAll(if (bordered) cellsOf(" │ ", bs) else spaces(sep))
            }
            if (bordered) l.addAll(cellsOf(" │", bs))
            out.add(trimRight(l))
        }
    }
    if (bordered) out.add(rule("└", "┴", "┘"))
    return out
}

private fun longestWord(n: Element): Int {
    var m = 1
    for (w in textContent(n).split(Regex("\\s+"))) {
        var cw = 0
        var i = 0
        while (i < w.length) {
            val cp = w.codePointAt(i)
            cw += runeWidth(cp)
            i += Character.charCount(cp)
        }
        if (cw > m) m = cw
    }
    return m
}
