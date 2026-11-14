package com.hayoonjae.asciiweb.engine

/** Options of the real-render mode (what the page looks like as characters). */
data class LiveSettings(
    val real: Boolean = true,        // real page rendering; false = text re-flow mode
    val style: String = "ascii",     // ascii, detail, blocks, braille
    val color: Boolean = true,
    val bold: Boolean = true,
    val invert: String = "auto",     // auto, on, off
    val contrast: Int = 0,           // -4..4
    val pageWidth: Int = 480,        // CSS px the page lays out at
    val quality: Int = 2,            // sampling resolution 1..3
    val fps: Int = 8,
    val mobile: Boolean = true,
    val search: String = "duckduckgo",
    val home: String = "https://duckduckgo.com",
) {
    companion object {
        val STYLES = listOf("ascii", "detail", "blocks", "braille")
        val INVERTS = listOf("auto", "on", "off")
        val WIDTHS = listOf(320, 360, 480, 640, 800, 1024, 1280)
        val FPSES = listOf(3, 5, 8, 12, 20)
        val SEARCHES = listOf("duckduckgo", "google", "naver", "bing")
        val STYLE_DESC = mapOf(
            "ascii" to "ASCII by brightness",
            "detail" to "ASCII, 70 levels",
            "blocks" to "half blocks, true pixels",
            "braille" to "braille dots, 2x4",
        )
    }

    fun searchUrl(q: String): String {
        val e = java.net.URLEncoder.encode(q, "UTF-8")
        return when (search) {
            "google" -> "https://www.google.com/search?q=$e"
            "naver" -> "https://search.naver.com/search.naver?query=$e"
            "bing" -> "https://www.bing.com/search?q=$e"
            else -> "https://duckduckgo.com/?q=$e"
        }
    }

    /** Turns typed text into a URL or a search for the chosen engine. */
    fun resolve(s0: String): String {
        val s = s0.trim()
        return when {
            s.isEmpty() -> home
            s.contains("://") -> s
            !s.contains(' ') && (s.contains('.') || s.startsWith("localhost")) -> "https://$s"
            else -> searchUrl(s)
        }
    }
}
