package com.hayoonjae.asciiweb.engine

import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt

/** Small RGB copy of an image; the original is dropped right away to keep memory low. */
class Thumb(val ow: Int, val oh: Int, val w: Int, val h: Int, val pix: ByteArray)

object Thumbs {
    const val MAX_W = 216
    const val MAX_H = 400

    /** Set by the Android layer (BitmapFactory). Tests build Thumb objects directly. */
    @Volatile
    var decoder: ((ByteArray) -> Thumb?)? = null
}

private const val RAMP = " .:-=+*#%@"

private class Px(val r: Double, val g: Double, val b: Double, var l: Double)

/** Renders the thumbnail as `cols` columns of ASCII characters. */
fun Thumb.art(cols0: Int, color: Boolean, target: Int): List<Line> {
    val cols = max(1, cols0)
    var rows = (cols.toDouble() * h / w * 0.5).roundToInt()
    rows = rows.coerceIn(1, 60)
    val grid = ArrayList<Px>(rows * cols)
    for (ry in 0 until rows) {
        val y0 = ry * h / rows
        var y1 = (ry + 1) * h / rows
        if (y1 <= y0) y1 = y0 + 1
        for (cx in 0 until cols) {
            val x0 = cx * w / cols
            var x1 = (cx + 1) * w / cols
            if (x1 <= x0) x1 = x0 + 1
            var sr = 0.0
            var sg = 0.0
            var sb = 0.0
            var n = 0.0
            var y = y0
            while (y < y1 && y < h) {
                var x = x0
                while (x < x1 && x < w) {
                    val i = (y * w + x) * 3
                    sr += pix[i].toInt() and 255
                    sg += pix[i + 1].toInt() and 255
                    sb += pix[i + 2].toInt() and 255
                    n++
                    x++
                }
                y++
            }
            if (n == 0.0) n = 1.0
            val r = sr / n
            val g = sg / n
            val b = sb / n
            grid.add(Px(r, g, b, (0.299 * r + 0.587 * g + 0.114 * b) / 255))
        }
    }
    // A bright border means "paper": draw the dark ink instead of the light.
    var sum = 0.0
    var cnt = 0.0
    for (cx in 0 until cols) {
        sum += grid[cx].l + grid[(rows - 1) * cols + cx].l
        cnt += 2
    }
    for (ry in 1 until rows - 1) {
        sum += grid[ry * cols].l + grid[ry * cols + cols - 1].l
        cnt += 2
    }
    val invert = cnt > 0 && sum / cnt > 0.8
    if (invert) for (p in grid) p.l = 1 - p.l
    val sorted = grid.map { it.l }.sorted()
    var lo = sorted[sorted.size * 2 / 100]
    var hi = sorted[(sorted.size - 1) * 98 / 100]
    if (hi - lo < 0.05) {
        lo = 0.0
        hi = 1.0
    }
    return List(rows) { ry ->
        val l = ArrayList<Cell>(cols)
        for (cx in 0 until cols) {
            val p = grid[ry * cols + cx]
            val v = ((p.l - lo) / (hi - lo)).coerceIn(0.0, 1.0)
            val ch = RAMP[(v * (RAMP.length - 1) + 0.5).toInt()]
            var fg = 0
            if (color && ch != ' ') fg = if (invert) normColor(p.r, p.g, p.b) else rgb(boost(p.r), boost(p.g), boost(p.b))
            l.add(Cell(ch.code, Style(fg = fg, t = target)))
        }
        trimRight(l)
    }
}

private fun boost(v: Double): Int = min(255.0, v * 1.2 + 24).toInt()

private fun normColor(r: Double, g: Double, b: Double): Int {
    val m = max(r, max(g, b))
    if (m < 40) return rgb(235, 235, 235)
    val k = 255 / m
    return rgb((r * k).toInt(), (g * k).toInt(), (b * k).toInt())
}
