package com.hayoonjae.asciiweb

import com.hayoonjae.asciiweb.engine.*
import org.jsoup.Jsoup
import org.junit.Assert.*
import org.junit.Test

class EngineTest {
    private fun page(html: String, cols: Int): Page {
        val p = Page("https://x.test/", Jsoup.parse(html, "https://x.test/"), Options(images = false))
        p.layout(cols)
        return p
    }

    private fun Page.dump() = lines.joinToString("\n") { it.text() }

    @Test fun adjacentPiecesStayTogether() {
        val p = page("<p>aaaa bbbb (<a href=\"/x\">cccc</a>) dddd</p>", 14)
        for (l in p.lines) {
            val s = l.text()
            assertFalse(s, s.startsWith(")") || s.endsWith("("))
        }
    }

    @Test fun wideRunesDoNotOverflow() {
        val p = page("<p>한글한글한글한글한글한글 한글한글</p>", 9)
        for (l in p.lines) assertTrue(l.size <= 9)
        assertTrue(p.dump().replace("\n", "").contains("한글한글한글한글한글한글"))
    }

    @Test fun linkTargets() {
        val p = page(
            "<a href=\"/a\">one two</a> <a href=\"javascript:void(0)\">js</a> " +
                "<a href=\"https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.org%2F\">ddg</a>", 40,
        )
        assertEquals(2, p.targets.size)
        assertEquals("https://x.test/a", p.targets[0].url)
        assertEquals("https://example.org/", p.targets[1].url)
        assertEquals(0, p.targetLine[0])
    }

    @Test fun tableFitsWidth() {
        val p = page("<table border=1><tr><td>aaaa aaaa aaaa aaaa</td><td>bbbb bbbb bbbb bbbb</td></tr></table>", 30)
        for (l in p.lines) assertTrue(l.size <= 30)
        assertTrue(p.dump().contains("┌"))
    }

    @Test fun formSubmit() {
        val p = page(
            "<form action=\"/s\" method=\"get\"><input name=\"q\" value=\"hi\"><select name=\"k\">" +
                "<option value=\"1\">a<option value=\"2\" selected>b</select><input type=\"checkbox\" name=\"c\" checked>" +
                "<input type=\"submit\" name=\"go\" value=\"Go\"></form>", 60,
        )
        val sub = p.targets.first { it.kind == T_SUBMIT }
        val (m, url, _) = p.submitRequest(sub.form!!, sub.node)
        assertEquals("GET", m)
        assertEquals("https://x.test/s?q=hi&k=2&c=on&go=Go", url)
    }

    @Test fun hiddenAndSkipped() {
        val p = page("<p>a<script>x</script><span style=\"display:none\">no</span><span hidden>no</span>b</p>", 20)
        assertEquals("ab", p.dump().trim())
    }

    @Test fun imageArtInvertsPaper() {
        val pix = ByteArray(40 * 40 * 3) { 255.toByte() }
        for (y in 10 until 30) for (x in 10 until 30) {
            val i = (y * 40 + x) * 3
            pix[i] = 0; pix[i + 1] = 0; pix[i + 2] = 0
        }
        val art = Thumb(40, 40, 40, 40, pix).art(20, false, 0)
        assertTrue(art[art.size / 2].text().any { it in "#%@" })
        assertTrue(art[0].text().isBlank())
    }

    @Test fun normalizeInput() {
        assertEquals("https://example.com", Browser.normalize("example.com"))
        assertEquals("about:home", Browser.normalize(""))
        assertTrue(Browser.normalize("hello world").startsWith("https://html.duckduckgo.com/html/?q=hello"))
    }
}
