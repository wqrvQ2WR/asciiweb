package com.hayoonjae.asciiweb.engine

import okhttp3.OkHttpClient
import okhttp3.Request
import org.jsoup.nodes.Document
import org.jsoup.nodes.Element
import org.jsoup.nodes.Node
import org.jsoup.nodes.TextNode
import java.net.URL
import java.net.URLDecoder
import java.net.URLEncoder
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

class Options(@Volatile var images: Boolean = true, @Volatile var color: Boolean = true)

const val T_LINK = 1
const val T_TEXT = 2
const val T_CHECK = 3
const val T_SELECT = 4
const val T_SUBMIT = 5

class Target(val kind: Int, val url: String = "", val node: Element? = null, val form: Element? = null)

const val USER_AGENT = "Mozilla/5.0 (Linux; Android 13; Mobile) AsciiWeb/0.1"

/** One loaded document plus its current layout. */
class Page(val url: String, val doc: Document, val opts: Options) {
    var method = "GET"
    var body = ""
    val title: String = doc.title().trim().replace(Regex("\\s+"), " ")

    @Volatile var lines: List<Line> = emptyList()
    @Volatile var targets: List<Target> = emptyList()
    @Volatile var targetLine: IntArray = IntArray(0)

    val vals = HashMap<Element, String>()
    val thumbs = ConcurrentHashMap<String, Thumb>()
    @Volatile private var cancelled = false
    private var pool: ExecutorService? = null

    /** Lays the document out for a terminal `cols` wide. */
    fun layout(cols: Int) {
        val c = Ctx(this)
        val ls = trimBlank(c.layout(doc, Style(), maxOf(8, cols), ALIGN_LEFT))
        val tl = IntArray(c.targets.size) { -1 }
        for ((y, l) in ls.withIndex()) for (cell in l) {
            val t = cell.s.t - 1
            if (t >= 0 && t < tl.size && tl[t] < 0) tl[t] = y
        }
        targets = c.targets
        targetLine = tl
        lines = ls
    }

    fun thumb(u: String): Thumb? = thumbs[u]

    fun resolve(href: String): String {
        val h = href.trim()
        val abs = try {
            URL(URL(doc.baseUri().ifEmpty { url }), h).toString()
        } catch (e: Exception) {
            h
        }
        // unwrap DuckDuckGo redirect links
        try {
            val u = URL(abs)
            if (u.host.endsWith("duckduckgo.com") && u.path == "/l/" && u.query != null) {
                for (kv in u.query.split('&')) if (kv.startsWith("uddg=")) return URLDecoder.decode(kv.substring(5), "UTF-8")
            }
        } catch (_: Exception) {
        }
        return abs
    }

    fun imgSrc(n: Element): String {
        var v = n.attr("src")
        if (v.isEmpty() || v.startsWith("data:")) {
            v = ""
            for (k in listOf("data-src", "data-original", "data-lazy-src")) if (n.attr(k).isNotEmpty()) {
                v = n.attr(k); break
            }
        }
        if (v.isEmpty()) {
            val ss = n.attr("srcset").trim()
            if (ss.isNotEmpty()) v = ss.split(',')[0].trim().split(Regex("\\s+"))[0]
        }
        if (v.isEmpty() || v.startsWith("data:")) return ""
        return resolve(v)
    }

    private fun imageUrls(): List<String> {
        val seen = LinkedHashSet<String>()
        doc.select("img").forEach { e -> imgSrc(e).takeIf { it.isNotEmpty() }?.let { seen.add(it) } }
        return seen.toList()
    }

    /** Downloads images in the background (4 at a time) and calls onUpdate when one arrives. */
    fun loadImages(client: OkHttpClient, onUpdate: () -> Unit) {
        if (!opts.images) return
        val urls = imageUrls().filter { !thumbs.containsKey(it) }.take(40)
        if (urls.isEmpty()) return
        cancelled = false
        val p = Executors.newFixedThreadPool(4)
        pool = p
        for (u in urls) p.execute {
            if (cancelled) return@execute
            val th = fetchThumb(client, u)
            if (th != null && !cancelled) {
                thumbs[u] = th
                onUpdate()
            }
        }
        p.shutdown()
    }

    fun stop() {
        cancelled = true
        pool?.shutdownNow()
    }

    private fun fetchThumb(client: OkHttpClient, u: String): Thumb? = try {
        val req = Request.Builder().url(u).header("User-Agent", USER_AGENT)
            .header("Accept", "image/webp,image/png,image/jpeg,image/gif,*/*;q=0.5")
            .header("Referer", url).build()
        client.newCall(req).execute().use { resp ->
            if (!resp.isSuccessful) null
            else {
                val bytes = readLimited(resp.body!!.byteStream(), 6 shl 20)
                Thumbs.decoder?.invoke(bytes)
            }
        }
    } catch (e: Exception) {
        null
    }

    // ---- forms ----

    fun value(n: Element): String {
        vals[n]?.let { return it }
        when (n.tagName()) {
            "textarea" -> return n.wholeText().removePrefix("\n")
            "input" -> {
                val t = n.attr("type").lowercase()
                if (t == "checkbox" || t == "radio") return if (n.hasAttr("checked")) "on" else ""
            }
        }
        return n.attr("value")
    }

    fun setValue(n: Element, v: String) {
        if (n.attr("type").lowercase() == "radio" && v.isNotEmpty()) {
            val form = ancestor(n, "form")
            val name = n.attr("name")
            form?.select("input[type=radio]")?.forEach { if (it.attr("name") == name) vals[it] = "" }
        }
        vals[n] = v
    }

    fun options(sel: Element): List<Element> = sel.select("option")
    fun optionText(o: Element): String = o.text().trim().ifEmpty { o.attr("value") }

    fun selectedIndex(sel: Element): Int {
        val opts = options(sel)
        vals[sel]?.toIntOrNull()?.let { if (it in opts.indices) return it }
        val i = opts.indexOfFirst { it.hasAttr("selected") }
        return if (i >= 0) i else 0
    }

    fun selectedText(sel: Element): String {
        val o = options(sel)
        return if (o.isEmpty()) "-" else optionText(o[selectedIndex(sel)])
    }

    fun selectOption(sel: Element, i: Int) {
        vals[sel] = i.toString()
    }

    fun cycleSelect(sel: Element) {
        val n = options(sel).size
        if (n > 0) vals[sel] = ((selectedIndex(sel) + 1) % n).toString()
    }

    fun formValues(form: Element, submit: Element?): List<Pair<String, String>> {
        val out = ArrayList<Pair<String, String>>()
        for (x in form.select("input, textarea, select, button")) {
            val name = x.attr("name")
            if (name.isEmpty() || x.hasAttr("disabled")) continue
            when (x.tagName()) {
                "input" -> when (x.attr("type").lowercase()) {
                    "submit", "button", "image", "reset" -> if (x === submit) out.add(name to x.attr("value"))
                    "checkbox", "radio" -> if (value(x).isNotEmpty()) out.add(name to x.attr("value").ifEmpty { "on" })
                    else -> out.add(name to value(x))
                }
                "textarea" -> out.add(name to value(x))
                "button" -> if (x === submit) out.add(name to x.attr("value"))
                "select" -> {
                    val o = options(x)
                    if (o.isNotEmpty()) {
                        val sel = o[selectedIndex(x)]
                        out.add(name to if (sel.hasAttr("value")) sel.attr("value") else optionText(sel))
                    }
                }
            }
        }
        return out
    }

    /** Returns (method, url, body) for submitting the form. */
    fun submitRequest(form: Element, submit: Element?): Triple<String, String, String> {
        val enc = formValues(form, submit).joinToString("&") {
            URLEncoder.encode(it.first, "UTF-8") + "=" + URLEncoder.encode(it.second, "UTF-8")
        }
        val action = form.attr("action").let { if (it.isEmpty()) url else resolve(it) }
        if (form.attr("method").equals("post", true)) return Triple("POST", action, enc)
        return Triple("GET", action.substringBefore('#').substringBefore('?') + "?" + enc, "")
    }
}

fun readLimited(ins: java.io.InputStream, max: Int): ByteArray {
    val out = java.io.ByteArrayOutputStream()
    val buf = ByteArray(16 * 1024)
    var total = 0
    while (total < max) {
        val n = ins.read(buf, 0, minOf(buf.size, max - total))
        if (n < 0) break
        out.write(buf, 0, n)
        total += n
    }
    return out.toByteArray()
}

fun ancestor(n: Element, tag: String): Element? {
    var p = n.parent()
    while (p != null) {
        if (p.tagName() == tag) return p
        p = p.parent()
    }
    return null
}

fun textContent(n: Node): String {
    val sb = StringBuilder()
    fun walk(x: Node) {
        when (x) {
            is TextNode -> sb.append(x.wholeText)
            is Element -> if (x.tagName() != "script" && x.tagName() != "style") x.childNodes().forEach { walk(it) }
            else -> {}
        }
    }
    walk(n)
    return sb.toString()
}
