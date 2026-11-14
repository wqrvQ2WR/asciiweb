package com.hayoonjae.asciiweb.engine

import org.jsoup.nodes.Element

// Only inline style="" and a few legacy attributes are understood (like w3m, no stylesheets).

fun styleProps(n: Element): Map<String, String>? {
    val s = n.attr("style")
    if (s.isEmpty()) return null
    val m = HashMap<String, String>()
    for (d in s.split(';')) {
        val i = d.indexOf(':')
        if (i > 0) m[d.substring(0, i).trim().lowercase()] = d.substring(i + 1).trim().lowercase()
    }
    return m
}

fun isHidden(n: Element): Boolean {
    if (n.hasAttr("hidden") || n.attr("aria-hidden") == "true") return true
    if (n.tagName() == "input" && n.attr("type").equals("hidden", true)) return true
    val m = styleProps(n) ?: return false
    val d = (m["display"] ?: "").replace(" ", "")
    return d.startsWith("none") || m["visibility"] == "hidden"
}

fun alignOf(n: Element): Int {
    if (n.tagName() == "center") return ALIGN_CENTER
    var v = n.attr("align").lowercase()
    val m = styleProps(n)
    if (m != null && !m["text-align"].isNullOrEmpty()) v = m["text-align"]!!
    return when (v) {
        "center", "middle" -> ALIGN_CENTER
        "right", "end" -> ALIGN_RIGHT
        "left", "start" -> ALIGN_LEFT
        else -> -1
    }
}

fun applyCss(n: Element, st0: Style): Style {
    var st = st0
    if (n.tagName() == "font") parseColor(n.attr("color"))?.let { st = st.copy(fg = it) }
    val m = styleProps(n) ?: return st
    m["color"]?.let { c -> parseColor(c)?.let { st = st.copy(fg = it) } }
    val w = m["font-weight"] ?: ""
    if (w == "bold" || w == "bolder" || (w.length == 3 && w[0] in '6'..'9')) st = st.copy(bold = true)
    if (m["font-style"] == "italic") st = st.copy(italic = true)
    if ((m["text-decoration"] ?: "").contains("underline")) st = st.copy(under = true)
    return st
}

private val NAMED = mapOf(
    "red" to 0xFF5555, "green" to 0x55DD77, "blue" to 0x5599FF, "yellow" to 0xFFDD55, "orange" to 0xFFA040,
    "purple" to 0xB070FF, "pink" to 0xFF77BB, "cyan" to 0x55DDEE, "magenta" to 0xFF55EE, "lime" to 0x77FF55,
    "white" to 0xFFFFFF, "gray" to 0xAAAAAA, "grey" to 0xAAAAAA, "silver" to 0xCCCCCC, "gold" to 0xFFD24D,
    "teal" to 0x44CCBB, "navy" to 0x6677FF, "maroon" to 0xFF6677, "olive" to 0xCCCC55, "aqua" to 0x55DDEE,
)

/** #rgb, #rrggbb, rgb(...) and a few names. Colors unreadable on a dark background are rejected. */
fun parseColor(s0: String): Int? {
    val s = s0.trim().lowercase()
    val r: Int
    val g: Int
    val b: Int
    when {
        s.isEmpty() -> return null
        s.startsWith("#") && s.length == 4 -> {
            val v = s.substring(1).toIntOrNull(16) ?: return null
            r = (v shr 8 and 15) * 17; g = (v shr 4 and 15) * 17; b = (v and 15) * 17
        }
        s.startsWith("#") && s.length == 7 -> {
            val v = s.substring(1).toIntOrNull(16) ?: return null
            r = v shr 16 and 255; g = v shr 8 and 255; b = v and 255
        }
        s.startsWith("rgb") -> {
            val i = s.indexOf('('); val j = s.indexOf(')')
            if (i < 0 || j < i) return null
            val parts = s.substring(i + 1, j).split(',', ' ', '/').filter { it.isNotEmpty() }
            if (parts.size < 3) return null
            val v = parts.take(3).map { it.removeSuffix("%").toIntOrNull() ?: return null }
            r = v[0]; g = v[1]; b = v[2]
        }
        else -> {
            val v = NAMED[s] ?: return null
            r = v shr 16; g = v shr 8 and 255; b = v and 255
        }
    }
    if ((299 * r + 587 * g + 114 * b) / 1000 < 110) return null
    return rgb(r.coerceIn(0, 255), g.coerceIn(0, 255), b.coerceIn(0, 255))
}
