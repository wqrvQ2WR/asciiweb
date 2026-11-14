package com.hayoonjae.asciiweb.engine

import kotlin.math.max
import kotlin.math.min

private const val RAMP_SIMPLE = " .:-=+*#%@"
private const val RAMP_DETAIL = " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@\$"

/** Sub-pixels per character cell for a style: (x, y). */
fun subPixels(style: String): Pair<Int, Int> = when (style) {
    "blocks" -> 1 to 2
    "braille" -> 2 to 4
    else -> 1 to 1
}

private class P(val r: Float, val g: Float, val b: Float) {
    val lum get() = (0.299f * r + 0.587f * g + 0.114f * b) / 255f
}

/** Box-averages argb pixels (w x h) down to a gw x gh grid. */
private fun sample(px: IntArray, w: Int, h: Int, gw: Int, gh: Int): Array<P> = Array(gw * gh) { i ->
    val x = i % gw
    val y = i / gw
    val x0 = x * w / gw
    val x1 = max(x0 + 1, (x + 1) * w / gw)
    val y0 = y * h / gh
    val y1 = max(y0 + 1, (y + 1) * h / gh)
    var r = 0f; var g = 0f; var b = 0f; var n = 0
    var yy = y0
    while (yy < y1 && yy < h) {
        var xx = x0
        while (xx < x1 && xx < w) {
            val c = px[yy * w + xx]
            r += (c shr 16) and 255; g += (c shr 8) and 255; b += c and 255; n++
            xx++
        }
        yy++
    }
    if (n == 0) n = 1
    P(r / n, g / n, b / n)
}

private fun contrast(l: Float, c: Int): Float = ((l - 0.5f) * (1 + 0.3f * c) + 0.5f).coerceIn(0f, 1f)

private fun bgLevel(v: FloatArray): Float {
    val h = IntArray(64)
    for (x in v) h[(x * 63.999f).toInt().coerceIn(0, 63)]++
    var best = 0
    for (i in h.indices) if (h[i] > h[best]) best = i
    return (best + 0.5f) / 64f
}

private fun fgColor(p: P, color: Boolean, ink: Boolean): Int {
    if (!color) return 0
    if (ink) {
        val m = max(p.r, max(p.g, p.b))
        if (m < 40) return rgb(235, 235, 235)
        val k = 255f / m
        return rgb((p.r * k).toInt(), (p.g * k).toInt(), (p.b * k).toInt())
    }
    fun f(v: Float) = min(255f, v * 1.15f + 20f).toInt()
    return rgb(f(p.r), f(p.g), f(p.b))
}

private fun pix(p: P) = rgb(p.r.toInt().coerceIn(0, 255), p.g.toInt().coerceIn(0, 255), p.b.toInt().coerceIn(0, 255))

/**
 * Turns a screen capture (argb, w x h) into cols x rows characters.
 * The capture should be roughly cols*subX*quality wide.
 */
fun convertScreen(px: IntArray, w: Int, h: Int, cols: Int, rows: Int, s: LiveSettings): List<Line> {
    val (sx, sy) = subPixels(s.style)
    val gw = cols * sx
    val gh = rows * sy
    val g = sample(px, w, h, gw, gh)
    var mean = 0f
    for (p in g) mean += p.lum
    mean /= g.size
    val invert = when (s.invert) { "on" -> true; "off" -> false; else -> mean > 0.55f }
    fun ink(p: P): Float {
        val l = contrast(p.lum, s.contrast)
        return if (invert) 1 - l else l
    }
    val bold = s.bold
    val out = ArrayList<Line>(rows)
    when (s.style) {
        "blocks" -> for (y in 0 until rows) {
            val l = ArrayList<Cell>(cols)
            for (x in 0 until cols) {
                val top = g[(2 * y) * gw + x]
                val bot = g[(2 * y + 1) * gw + x]
                if (!s.color) {
                    val v = (ink(top) + ink(bot)) / 2
                    l.add(Cell(" ░▒▓█"[(v * 4 + 0.5f).toInt()].code, Style(bold = bold)))
                } else l.add(Cell(0x2580, Style(fg = pix(top), bg = pix(bot))))
            }
            out.add(l)
        }
        "braille" -> {
            val all = FloatArray(g.size) { ink(g[it]) }
            val thr = min(0.9f, bgLevel(all) + 0.22f)
            val bit = arrayOf(intArrayOf(1, 8), intArrayOf(2, 16), intArrayOf(4, 32), intArrayOf(64, 128))
            for (y in 0 until rows) {
                val l = ArrayList<Cell>(cols)
                for (x in 0 until cols) {
                    var bits = 0
                    var sr = 0f; var sg = 0f; var sb = 0f; var n = 0
                    for (dy in 0 until 4) for (dx in 0 until 2) {
                        val p = g[(y * 4 + dy) * gw + x * 2 + dx]
                        if (ink(p) > thr) { bits = bits or bit[dy][dx]; sr += p.r; sg += p.g; sb += p.b; n++ }
                    }
                    if (bits == 0) l.add(Cell(' '.code, Style()))
                    else l.add(Cell(0x2800 + bits, Style(fg = fgColor(P(sr / n, sg / n, sb / n), s.color, invert), bold = bold)))
                }
                out.add(l)
            }
        }
        else -> {
            val ramp = if (s.style == "detail") RAMP_DETAIL else RAMP_SIMPLE
            val lums = FloatArray(g.size) { ink(g[it]) }
            val srt = lums.sortedArray()
            val lo = bgLevel(lums)
            var hi = srt[(srt.size - 1) * 98 / 100]
            if (hi - lo < 0.15f) hi = lo + 0.5f
            for (y in 0 until rows) {
                val l = ArrayList<Cell>(cols)
                for (x in 0 until cols) {
                    val v = (((lums[y * gw + x] - lo) / (hi - lo) - 0.1f) / 0.9f).coerceIn(0f, 1f)
                    val ch = ramp[(v * (ramp.length - 1) + 0.5f).toInt()]
                    l.add(if (ch == ' ') Cell(' '.code, Style()) else Cell(ch.code, Style(fg = fgColor(g[y * gw + x], s.color, invert), bold = bold)))
                }
                out.add(l)
            }
        }
    }
    return out
}
