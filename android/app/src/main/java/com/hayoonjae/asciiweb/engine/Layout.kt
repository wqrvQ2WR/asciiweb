package com.hayoonjae.asciiweb.engine

import org.jsoup.nodes.Element
import org.jsoup.nodes.Node
import org.jsoup.nodes.TextNode

class Ctx(val p: Page) {
    val targets = ArrayList<Target>()
    var depth = 0

    fun addTarget(t: Target): Int {
        targets.add(t)
        return targets.size
    }

    /** Lays out the children of n as block content w cells wide. */
    fun layout(n: Node, st: Style, w: Int, align: Int): List<Line> {
        if (depth > 40) return emptyList()
        depth++
        try {
            val f = Flow(this, w, align)
            f.walk(n, st)
            f.flush()
            return f.out
        } finally {
            depth--
        }
    }

    fun listDepth(n: Element): Int {
        var d = 0
        var p = n.parent()
        while (p != null) {
            if (p.tagName() == "ul" || p.tagName() == "ol") d++
            p = p.parent()
        }
        return d
    }

    fun list(n: Element, st: Style, w: Int): List<Line> {
        val out = ArrayList<Line>()
        var idx = n.attr("start").toIntOrNull() ?: 1
        val depth = listDepth(n)
        for (li in n.children()) {
            if (li.tagName() != "li" || isHidden(li)) continue
            val marker = if (n.tagName() == "ol") "$idx. " else if (depth % 2 == 0) "• " else "◦ "
            idx++
            val mw = maxOf(2, marker.length)
            val sub = trimBlank(layout(li, applyCss(li, st), w - mw, ALIGN_LEFT))
            val first = cellsOf(marker, Style(fg = COL_H2))
            while (first.size < mw) first.add(Cell(' '.code, Style()))
            for ((i, l) in sub.withIndex()) {
                val nl = ArrayList<Cell>(mw + l.size)
                nl.addAll(if (i == 0) first else spaces(mw))
                nl.addAll(l)
                out.add(nl)
            }
            if (sub.isEmpty()) out.add(first)
        }
        return out
    }
}

private val SKIP = setOf(
    "head", "script", "style", "noscript", "template", "svg", "iframe", "frame", "canvas", "title",
    "link", "meta", "object", "embed", "map", "dialog",
)
private val BLOCK = setOf(
    "html", "body", "div", "section", "article", "header", "footer", "main", "nav", "aside", "form",
    "address", "figure", "figcaption", "fieldset", "legend", "details", "summary", "center", "dl", "dt",
    "menu", "dir", "hgroup", "search", "tr", "td", "th", "tbody", "thead", "tfoot", "caption", "optgroup",
)
private val WS = Regex("[ \n\t\r\u000c]+")

class Flow(val c: Ctx, val w: Int, var align: Int) {
    val out = ArrayList<Line>()
    private val toks = ArrayList<Tok>()

    fun flush() {
        if (toks.isNotEmpty()) {
            out.addAll(wrap(toks, w, align))
            toks.clear()
        }
    }

    /** One blank line between blocks. */
    fun gap() {
        if (out.isNotEmpty() && !isBlank(out.last())) out.add(emptyList())
    }

    private fun space(st: Style) {
        val l = toks.lastOrNull()
        if (l == null || l.space || l.br) return
        toks.add(Tok(arrayListOf(Cell(' '.code, st)), space = true))
    }

    private fun isSp(ch: Char) = ch == ' ' || ch == '\n' || ch == '\t' || ch == '\r' || ch == '\u000c'

    fun text(s: String, st: Style) {
        if (s.isEmpty()) return
        val words = s.split(WS).filter { it.isNotEmpty() }
        if (words.isEmpty()) {
            space(st); return
        }
        if (isSp(s.first())) space(st)
        for ((i, wd) in words.withIndex()) {
            if (i > 0) space(st)
            val cs = cellsOf(wd, st)
            if (cs.isNotEmpty()) toks.add(Tok(cs))
        }
        if (isSp(s.last())) space(st)
    }

    fun walk(n: Node, st: Style) {
        for (ch in n.childNodes()) when (ch) {
            is TextNode -> text(ch.wholeText, st)
            is Element -> elem(ch, st)
            else -> {}
        }
    }

    private fun block(n: Element, st: Style) {
        flush()
        val old = align
        val a = alignOf(n)
        if (a >= 0) align = a
        walk(n, st)
        flush()
        align = old
    }

    private fun indent(n: Element, st: Style, first: List<Cell>, rest: List<Cell>) {
        flush()
        val ww = maxOf(4, w - first.size)
        val lines = trimBlank(c.layout(n, st, ww, align))
        for ((i, l) in lines.withIndex()) {
            val nl = ArrayList<Cell>(first.size + l.size)
            nl.addAll(if (i == 0) first else rest)
            nl.addAll(l)
            out.add(nl)
        }
    }

    private fun elem(n: Element, st0: Style) {
        val tag = n.tagName()
        if (tag in SKIP || isHidden(n)) return
        var st = applyCss(n, st0)
        when (tag) {
            "br" -> toks.add(Tok(ArrayList(), br = true))
            "hr" -> {
                flush(); gap()
                out.add(cellsOf("─".repeat(minOf(w, 72)), Style(fg = COL_RULE)))
                gap()
            }
            "img" -> img(n, st)
            "a" -> {
                val href = n.attr("href")
                if (href.isNotEmpty() && !href.lowercase().startsWith("javascript:")) {
                    st = st.copy(t = c.addTarget(Target(T_LINK, url = c.p.resolve(href))), fg = COL_LINK, under = true)
                }
                walk(n, st)
            }
            "input", "button", "select", "textarea" -> control(n, st)
            "table" -> {
                flush(); gap()
                out.addAll(c.table(n, st, w))
                gap()
            }
            "ul", "ol" -> {
                flush()
                val top = c.listDepth(n) == 0
                if (top) gap()
                out.addAll(c.list(n, st, w))
                if (top) gap()
            }
            "li" -> indent(n, st, cellsOf("• ", Style(fg = COL_H2)), spaces(2))
            "p" -> {
                flush(); gap(); block(n, st); gap()
            }
            "h1", "h2", "h3", "h4", "h5", "h6" -> heading(n, st)
            "pre" -> pre(n, st)
            "blockquote" -> {
                gap()
                val q = Style(fg = COL_QUOTE)
                indent(n, st.copy(fg = COL_QUOTE), cellsOf("│ ", q), cellsOf("│ ", q))
                gap()
            }
            "dd" -> indent(n, st, spaces(4), spaces(4))
            "dt" -> block(n, st.copy(bold = true))
            "small" -> walk(n, st.copy(dim = true))
            "b", "strong" -> walk(n, st.copy(bold = true))
            "i", "em", "cite", "var", "dfn" -> walk(n, st.copy(italic = true))
            "u" -> walk(n, st.copy(under = true))
            "mark" -> walk(n, st.copy(rev = true))
            "code", "kbd", "samp", "tt" -> walk(n, st.copy(fg = COL_CODE))
            "del", "s", "strike" -> walk(n, st.copy(dim = true))
            "video", "audio" -> toks.add(Tok(cellsOf("[$tag]", Style(dim = true, t = st.t))))
            else -> if (tag in BLOCK) block(n, st) else walk(n, st)
        }
    }

    private fun heading(n: Element, st0: Style) {
        flush(); gap()
        val st = st0.copy(
            bold = true,
            fg = when (n.tagName()) {
                "h1" -> COL_H1
                "h2" -> COL_H2
                "h3" -> COL_H3
                else -> st0.fg
            },
        )
        val start = out.size
        block(n, st)
        if ((n.tagName() == "h1" || n.tagName() == "h2") && out.size > start) {
            val mw = minOf(w, maxWidth(out.subList(start, out.size)))
            if (mw > 0) out.add(cellsOf((if (n.tagName() == "h1") "━" else "─").repeat(mw), Style(fg = st.fg)))
        }
        gap()
    }

    private fun pre(n: Element, st0: Style) {
        flush(); gap()
        var s = textContent(n).replace("\r", "").removePrefix("\n").trimEnd('\n', ' ')
        s = s.replace("\t", "    ")
        val st = st0.copy(fg = COL_CODE)
        for (ln in s.split("\n")) {
            var cs: List<Cell> = cellsOf(ln, st)
            while (cs.size > w && w > 2) {
                var cut = w
                if (cs[cut].r == 0) cut--
                out.add(cs.subList(0, cut))
                cs = cs.subList(cut, cs.size)
            }
            out.add(cs)
        }
        gap()
    }

    private fun img(n: Element, st: Style) {
        val alt = n.attr("alt").trim().replace(Regex("\\s+"), " ")
        val src = c.p.imgSrc(n)
        val th = if (c.p.opts.images && src.isNotEmpty()) c.p.thumb(src) else null
        if (th == null || th.ow < 32 || th.oh < 32) {
            val ph = Style(dim = true, t = st.t)
            if (alt.isNotEmpty()) toks.add(Tok(cellsOf("[$alt]", ph)))
            else if (th == null && src.isNotEmpty() && c.p.opts.images && attrInt(n, "width") != 1) toks.add(Tok(cellsOf("[img]", ph)))
            return
        }
        var wpx = attrInt(n, "width")
        if (wpx <= 0) wpx = th.ow
        val cols = (wpx / 6).coerceIn(6, maxOf(6, minOf(w, 76)))
        flush()
        var lines = th.art(cols, c.p.opts.color, st.t)
        if (align != ALIGN_LEFT) lines = lines.map { aligned(it, w, align) }
        out.addAll(lines)
    }

    private fun control(n: Element, st: Style) {
        val p = c.p
        val typ = n.attr("type").lowercase()
        val form = ancestor(n, "form")
        if (form == null && (n.tagName() == "button" || typ == "checkbox" || typ == "radio" || typ == "button" || typ == "submit")) return
        val label: String
        val kind: Int
        when (n.tagName()) {
            "input" -> when (typ) {
                "hidden" -> return
                "submit", "button", "image", "reset" -> {
                    val v = n.attr("value").ifEmpty { n.attr("alt") }.ifEmpty { "Submit" }
                    label = "[ $v ]"; kind = T_SUBMIT
                }
                "checkbox", "radio" -> {
                    val on = p.value(n).isNotEmpty()
                    label = if (typ == "checkbox") (if (on) "[x]" else "[ ]") else (if (on) "(*)" else "( )")
                    kind = T_CHECK
                }
                else -> { label = fieldLabel(n, typ); kind = T_TEXT }
            }
            "button" -> {
                val t = n.text().trim().ifEmpty { "Button" }
                label = "[ $t ]"; kind = T_SUBMIT
            }
            "textarea" -> { label = fieldLabel(n, "textarea"); kind = T_TEXT }
            else -> { label = "[ ${p.selectedText(n)} ▾ ]"; kind = T_SELECT }
        }
        val id = c.addTarget(Target(kind, node = n, form = form))
        toks.add(Tok(cellsOf(label, Style(fg = COL_CTRL, t = id))))
    }

    private fun fieldLabel(n: Element, typ: String): String {
        val v = c.p.value(n)
        var size = attrInt(n, "size").let { if (it <= 0) 20 else minOf(it, 32) }
        if (w - 4 in 7 until size) size = w - 4
        var shown = v
        if (typ == "password") shown = "*".repeat(v.codePointCount(0, v.length))
        if (v.isEmpty()) shown = n.attr("placeholder")
        shown = shown.trim().replace(Regex("\\s+"), " ")
        val sb = StringBuilder()
        var used = 0
        var i = 0
        while (i < shown.length) {
            val cp = shown.codePointAt(i)
            val cw = runeWidth(cp)
            if (used + cw > size) break
            sb.appendCodePoint(cp)
            used += cw
            i += Character.charCount(cp)
        }
        sb.append("_".repeat(maxOf(0, size - used)))
        return "[$sb]"
    }
}

fun attrInt(n: Element, k: String): Int {
    val v = n.attr(k).trim().removeSuffix("px")
    return v.toIntOrNull()?.takeIf { it >= 0 } ?: 0
}
