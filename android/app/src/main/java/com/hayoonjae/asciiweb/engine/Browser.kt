package com.hayoonjae.asciiweb.engine

import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.FormBody
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.jsoup.Jsoup
import java.net.URLEncoder
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit

private class MemJar : CookieJar {
    private val m = ConcurrentHashMap<String, MutableList<Cookie>>()
    override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) {
        val l = m.getOrPut(url.host) { ArrayList() }
        synchronized(l) {
            for (c in cookies) {
                l.removeAll { it.name == c.name && it.path == c.path }
                l.add(c)
            }
        }
    }

    override fun loadForRequest(url: HttpUrl): List<Cookie> {
        val l = m[url.host] ?: return emptyList()
        synchronized(l) { return l.filter { it.matches(url) } }
    }
}

private const val BANNER = """
    _    ____   ____ ___ ___ __        _______ ____
   / \  / ___| / ___|_ _|_ _|\ \      / / ____| __ )
  / _ \ \___ \| |    | | | |  \ \ /\ / /|  _| |  _ \
 / ___ \ ___) | |___ | | | |   \ V  V / | |___| |_) |
/_/   \_\____/ \____|___|___|   \_/\_/  |_____|____/
"""

private const val HOME_HTML = """<html><head><title>AsciiWeb</title></head><body>
<pre>%BANNER%</pre>
<center><p>The web, drawn with text. Images become ASCII art.</p>
<form action="https://html.duckduckgo.com/html/" method="get">
<input type="text" name="q" size="28" placeholder="search or type a URL"> <input type="submit" value="Search">
</form></center>
<h2>Tips</h2>
<ul>
<li>Tap a link to open it. Tap a field to type.</li>
<li>Scroll by dragging. Back button goes back.</li>
<li>Menu: images on/off, color on/off, text size.</li>
</ul>
<p>Try: <a href="https://news.ycombinator.com">Hacker News</a> &nbsp; <a href="https://en.wikipedia.org/wiki/ASCII_art">ASCII art on Wikipedia</a></p>
</body></html>"""

class Browser {
    val opts = Options()
    val client: OkHttpClient = OkHttpClient.Builder()
        .cookieJar(MemJar())
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(25, TimeUnit.SECONDS)
        .build()

    private class Entry(val url: String, val method: String, val body: String, var scroll: Int, var page: Page?)

    private val hist = ArrayList<Entry>()
    private var pos = -1
    var cur: Page? = null
        private set

    val canBack get() = pos > 0
    val canForward get() = pos >= 0 && pos < hist.size - 1

    /** Fetches a page without touching browser state; call from a background thread. */
    fun fetch(method: String, raw: String, body: String): Page {
        if (raw.startsWith("about:")) return about(raw)
        val rb = Request.Builder().url(raw)
            .header("User-Agent", USER_AGENT)
            .header("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,image/*;q=0.8,*/*;q=0.5")
            .header("Accept-Language", "ko,en;q=0.8")
        if (method == "POST") rb.post(body.toRequestBody("application/x-www-form-urlencoded".toMediaType()))
        client.newCall(rb.build()).execute().use { resp ->
            val final = resp.request.url.toString()
            val ct = (resp.header("Content-Type") ?: "").lowercase()
            val data = readLimited(resp.body!!.byteStream(), 4 shl 20)
            if (resp.code >= 400 && data.isEmpty()) throw RuntimeException("HTTP ${resp.code}")
            val charset = Regex("charset=([\\w-]+)").find(ct)?.groupValues?.get(1)
            val page = when {
                ct.startsWith("image/") -> Jsoup.parse("<html><body><img src=\"$final\"></body></html>", final)
                ct.contains("html") || ct.contains("xml") || ct.isEmpty() ->
                    Jsoup.parse(data.inputStream(), charset, final)
                ct.startsWith("text/") -> {
                    val txt = String(data, charset?.let { runCatching { charset(it) }.getOrNull() } ?: Charsets.UTF_8)
                    Jsoup.parse("<html><body><pre>" + org.jsoup.nodes.Entities.escape(txt) + "</pre></body></html>", final)
                }
                else -> throw RuntimeException("unsupported content type: $ct")
            }
            return Page(final, page, opts).also { it.method = method; it.body = body }
        }
    }

    private fun about(raw: String): Page {
        val html = when (raw) {
            "about:home" -> HOME_HTML.replace("%BANNER%", BANNER.replace("<", "&lt;"))
            else -> throw RuntimeException("unknown page: $raw")
        }
        return Page(raw, Jsoup.parse(html, "https://about.invalid/"), opts)
    }

    /** Makes p the current page; push adds a history entry, otherwise the current one is replaced. */
    fun commit(p: Page, push: Boolean, scrollLeaving: Int) {
        cur?.stop()
        if (pos in hist.indices) hist[pos].scroll = scrollLeaving
        val e = Entry(p.url, p.method, p.body, 0, p)
        if (push || pos < 0) {
            while (hist.size > pos + 1) hist.removeAt(hist.size - 1)
            hist.add(e)
            pos = hist.size - 1
            if (pos > 3 && hist[pos - 4].method != "POST") hist[pos - 4].page = null // keep memory low
        } else hist[pos] = e
        cur = p
    }

    class Step(val url: String, val method: String, val body: String, val cached: Page?, val scroll: Int)

    fun step(d: Int): Step? {
        val n = pos + d
        if (n !in hist.indices) return null
        val e = hist[n]
        return Step(e.url, e.method, e.body, e.page, e.scroll)
    }

    fun moveTo(d: Int, p: Page, scrollLeaving: Int) {
        if (pos in hist.indices) hist[pos].scroll = scrollLeaving
        cur?.stop()
        pos += d
        hist[pos].page = p
        cur = p
    }

    companion object {
        /** Turns what the user typed into a URL or a web search. */
        fun normalize(s0: String): String {
            val s = s0.trim()
            return when {
                s.isEmpty() -> "about:home"
                s.startsWith("about:") || s.contains("://") -> s
                !s.contains(' ') && !s.contains('\t') && (s.contains('.') || s.startsWith("localhost")) -> "https://$s"
                else -> "https://html.duckduckgo.com/html/?q=" + URLEncoder.encode(s, "UTF-8")
            }
        }
    }
}
