package com.hayoonjae.asciiweb

import android.graphics.Canvas as NativeCanvas
import android.graphics.Paint
import android.graphics.Typeface
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.sp
import com.hayoonjae.asciiweb.engine.Cell
import com.hayoonjae.asciiweb.engine.Line
import com.hayoonjae.asciiweb.engine.Style
import kotlin.math.ceil
import kotlin.math.max
import kotlin.math.min

const val BG_COLOR = 0xFF0B0B14.toInt()
private const val FG_COLOR = 0xFFE6E6EF.toInt()

/** Paints for the character grid: every cell is placed by column, so wide glyphs and box lines stay aligned. */
class GridPaints(val px: Float) {
    private fun mk(bold: Boolean, italic: Boolean) = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        textSize = px
        typeface = Typeface.create(
            Typeface.MONOSPACE,
            when {
                bold && italic -> Typeface.BOLD_ITALIC
                bold -> Typeface.BOLD
                italic -> Typeface.ITALIC
                else -> Typeface.NORMAL
            },
        )
    }

    private val reg = mk(false, false)
    private val bold = mk(true, false)
    private val ital = mk(false, true)
    private val bi = mk(true, true)
    val cellW = reg.measureText("M")
    private val fm = reg.fontMetrics
    val lineH = ceil(fm.descent - fm.ascent + 3f)
    val baseline = -fm.ascent + 1.5f
    private val fill = Paint()
    private val stroke = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeCap = Paint.Cap.BUTT
    }
    private val strokeW = max(1.4f, px / 9f)

    fun pick(s: Style) = when {
        s.bold && s.italic -> bi
        s.bold -> bold
        s.italic -> ital
        else -> reg
    }

    private fun color(s: Style): Int {
        val base = if (s.fg == 0) FG_COLOR else 0xFF000000.toInt() or (s.fg and 0xFFFFFF)
        if (!s.dim) return base
        return (base and 0x00FFFFFF) or (0xA0 shl 24)
    }

    fun drawLine(c: NativeCanvas, line: Line, hl: Int) {
        var i = 0
        while (i < line.size) {
            val cell = line[i]
            val s0 = cell.s
            val rev = s0.rev || (hl != 0 && s0.t == hl)
            // run of cells with the same look
            var j = i
            while (j < line.size && line[j].s.look() == s0.look() && (line[j].s.t == hl) == (s0.t == hl)) j++
            val x0 = i * cellW
            val fg = color(s0)
            val p = pick(s0)
            if (s0.bg != 0) {
                fill.color = 0xFF000000.toInt() or (s0.bg and 0xFFFFFF)
                c.drawRect(x0, 0f, j * cellW, lineH, fill)
            }
            if (rev) {
                fill.color = fg
                c.drawRect(x0, 0f, j * cellW, lineH, fill)
            }
            val textColor = if (rev) BG_COLOR else fg
            p.color = textColor
            stroke.color = textColor
            // text: ASCII runs as strings, everything else per cell
            val sb = StringBuilder()
            var runStart = i
            fun flushRun(endExclusive: Int) {
                if (sb.isNotEmpty()) {
                    c.drawText(sb.toString(), runStart * cellW, baseline, p)
                    sb.setLength(0)
                }
            }
            for (k in i until j) {
                val r = line[k].r
                when {
                    r == 0 -> {}
                    r in 32..126 -> {
                        if (sb.isEmpty()) runStart = k
                        sb.append(r.toChar())
                    }
                    else -> {
                        flushRun(k)
                        drawGlyph(c, r, k, line, p)
                    }
                }
            }
            flushRun(j)
            if (s0.under) {
                stroke.strokeWidth = max(1f, strokeW * 0.8f)
                val y = baseline + 2f
                c.drawLine(x0, y, j * cellW, y, stroke)
            }
            i = j
        }
    }

    private fun drawGlyph(c: NativeCanvas, r: Int, col: Int, line: List<Cell>, p: Paint) {
        val x = col * cellW
        if (drawBox(c, r, x)) return
        if (r in 0x2800..0x28FF) { drawBraille(c, r - 0x2800, x, p); return }
        if (drawShade(c, r, x, p)) return
        val wide = col + 1 < line.size && line[col + 1].r == 0
        val s = String(Character.toChars(r))
        val gw = p.measureText(s)
        val cw = if (wide) cellW * 2 else cellW
        val dx = if (gw < cw) (cw - gw) / 2 else 0f
        c.drawText(s, x + dx, baseline, p)
    }

    /** Braille patterns are drawn as dots so they stay crisp whatever the font. */
    private fun drawBraille(c: NativeCanvas, bits: Int, x: Float, p: Paint) {
        fill.color = p.color
        val rad = min(cellW / 4.2f, lineH / 8.6f) * (if (p.isFakeBoldText || p.typeface?.isBold == true) 1.2f else 0.95f)
        val dots = intArrayOf(1, 2, 4, 64, 8, 16, 32, 128) // column 0 top..bottom, column 1 top..bottom
        for (i in 0 until 8) if (bits and dots[i] != 0) {
            val cx = x + cellW * (if (i < 4) 0.28f else 0.72f)
            val cy = lineH * (0.125f + 0.25f * (i % 4))
            c.drawCircle(cx, cy, rad, fill)
        }
    }

    /** Half block and shade characters. */
    private fun drawShade(c: NativeCanvas, r: Int, x: Float, p: Paint): Boolean {
        val a = when (r) {
            0x2591 -> 0.25f
            0x2592 -> 0.5f
            0x2593 -> 0.75f
            0x2588 -> 1f
            0x2580 -> -1f
            else -> return false
        }
        fill.color = p.color
        if (a < 0) {
            c.drawRect(x, 0f, x + cellW, lineH / 2f, fill)
        } else {
            fill.alpha = (255 * a).toInt()
            c.drawRect(x, 0f, x + cellW, lineH, fill)
            fill.alpha = 255
        }
        return true
    }

    /** Box drawing characters are drawn as lines so borders connect between rows. */
    private fun drawBox(c: NativeCanvas, r: Int, x: Float): Boolean {
        var l = false
        var rt = false
        var u = false
        var d = false
        when (r.toChar()) {
            '─', '━' -> { l = true; rt = true }
            '│' -> { u = true; d = true }
            '┌' -> { rt = true; d = true }
            '┐' -> { l = true; d = true }
            '└' -> { rt = true; u = true }
            '┘' -> { l = true; u = true }
            '├' -> { u = true; d = true; rt = true }
            '┤' -> { u = true; d = true; l = true }
            '┬' -> { l = true; rt = true; d = true }
            '┴' -> { l = true; rt = true; u = true }
            '┼' -> { l = true; rt = true; u = true; d = true }
            else -> return false
        }
        stroke.strokeWidth = if (r == '━'.code) strokeW * 1.8f else strokeW
        val mx = x + cellW / 2
        val my = lineH / 2
        if (l) c.drawLine(x, my, mx, my, stroke)
        if (rt) c.drawLine(mx, my, x + cellW, my, stroke)
        if (u) c.drawLine(mx, 0f, mx, my, stroke)
        if (d) c.drawLine(mx, my, mx, lineH, stroke)
        return true
    }
}

private fun Style.look() = copy(t = 0)

@Composable
fun GridView(vm: BrowserVm, modifier: Modifier = Modifier) {
    val density = LocalDensity.current
    val px = with(density) { vm.textSp.sp.toPx() }
    val paints = remember(px) { GridPaints(px) }
    val lineDp = with(density) { paints.lineH.toDp() }
    BoxWithConstraints(modifier.fillMaxSize()) {
        val cols = (constraints.maxWidth / paints.cellW).toInt()
        LaunchedEffect(cols) { if (cols > 0) vm.updateCols(cols) }
        val listState = rememberLazyListState()
        LaunchedEffect(listState) {
            snapshotFlow { listState.firstVisibleItemIndex }.collect { vm.scrollNow = it }
        }
        LaunchedEffect(vm.restoreSeq) { listState.scrollToItem(min(vm.restoreTo, max(0, vm.lines.size - 1))) }
        val lines = vm.lines
        val flash = vm.flash
        LazyColumn(state = listState, modifier = Modifier.fillMaxSize()) {
            items(lines) { line ->
                Canvas(
                    Modifier
                        .fillMaxWidth()
                        .height(lineDp)
                        .pointerInput(line) {
                            detectTapGestures { off ->
                                val col = (off.x / paints.cellW).toInt()
                                for (k in intArrayOf(col, col - 1, col + 1)) {
                                    val t = line.getOrNull(k)?.s?.t ?: 0
                                    if (t > 0) {
                                        vm.tap(t)
                                        return@detectTapGestures
                                    }
                                }
                            }
                        },
                ) {
                    drawIntoCanvas { paints.drawLine(it.nativeCanvas, line, flash) }
                }
            }
        }
    }
}
