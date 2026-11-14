package com.hayoonjae.asciiweb.engine

/** Style of one cell. fg==0 is the default color, otherwise RGBF or 0xRRGGBB. t is the target id (index+1). */
data class Style(
    val fg: Int = 0,
    val bg: Int = 0,
    val bold: Boolean = false,
    val italic: Boolean = false,
    val under: Boolean = false,
    val dim: Boolean = false,
    val rev: Boolean = false,
    val t: Int = 0,
)

const val RGBF = 1 shl 24
fun rgb(r: Int, g: Int, b: Int) = RGBF or (r shl 16) or (g shl 8) or b

val COL_LINK = rgb(0x3D, 0xA9, 0xFF)
val COL_H1 = rgb(0xFF, 0x4D, 0x9D)
val COL_H2 = rgb(0x9B, 0x5C, 0xFF)
val COL_H3 = rgb(0x3D, 0xA9, 0xFF)
val COL_CODE = rgb(0x7C, 0xE3, 0xA1)
val COL_CTRL = rgb(0xFF, 0xD2, 0x4D)
val COL_QUOTE = rgb(0x9A, 0xA0, 0xB5)
val COL_RULE = rgb(0x6B, 0x72, 0x90)

/** One terminal column. A double-width character takes two cells; the second has r == 0. */
data class Cell(val r: Int, val s: Style)

typealias Line = List<Cell>

fun Line.text(): String {
    val sb = StringBuilder()
    for (c in this) if (c.r != 0) sb.appendCodePoint(c.r)
    return sb.toString()
}

fun runeWidth(cp: Int): Int {
    if (cp == 0 || cp < 32 || cp in 0x7f..0x9f) return 0
    if (cp < 0x300) return 1
    when (Character.getType(cp)) {
        Character.NON_SPACING_MARK.toInt(), Character.ENCLOSING_MARK.toInt(), Character.FORMAT.toInt() -> return 0
    }
    if (cp in 0xFE00..0xFE0F) return 0
    return if (cp in 0x1100..0x115F || cp in 0x2E80..0x303E || cp in 0x3041..0x33FF ||
        cp in 0x3400..0x4DBF || cp in 0x4E00..0x9FFF || cp in 0xA000..0xA4CF ||
        cp in 0xAC00..0xD7A3 || cp in 0xF900..0xFAFF || cp in 0xFE30..0xFE6F ||
        cp in 0xFF01..0xFF60 || cp in 0xFFE0..0xFFE6 || cp in 0x1F300..0x1F64F ||
        cp in 0x1F900..0x1F9FF || cp in 0x20000..0x3FFFD
    ) 2 else 1
}

fun cellsOf(s: String, st: Style): ArrayList<Cell> {
    val out = ArrayList<Cell>(s.length)
    var i = 0
    while (i < s.length) {
        var cp = s.codePointAt(i)
        i += Character.charCount(cp)
        if (cp == 0xa0 || cp == '\t'.code) cp = ' '.code
        val w = runeWidth(cp)
        if (w == 0) continue
        out.add(Cell(cp, st))
        if (w == 2) out.add(Cell(0, st))
    }
    return out
}

fun spaces(n: Int, st: Style = Style()): ArrayList<Cell> {
    val l = ArrayList<Cell>(maxOf(n, 0))
    repeat(n) { l.add(Cell(' '.code, st)) }
    return l
}

fun isBlank(l: Line) = l.all { it.r == ' '.code || it.r == 0 }

fun trimRight(l: Line): Line {
    var n = l.size
    while (n > 0) {
        val c = l[n - 1]
        if (c.r == ' '.code && c.s.t == 0 && !c.s.rev) n-- else break
    }
    return if (n == l.size) l else l.subList(0, n)
}

fun trimBlank(ls: List<Line>): List<Line> {
    var a = 0
    var b = ls.size
    while (a < b && isBlank(ls[a])) a++
    while (b > a && isBlank(ls[b - 1])) b--
    return ls.subList(a, b)
}

fun maxWidth(ls: List<Line>): Int = ls.maxOfOrNull { trimRight(it).size } ?: 0
