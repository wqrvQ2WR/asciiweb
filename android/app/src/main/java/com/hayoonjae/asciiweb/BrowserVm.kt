package com.hayoonjae.asciiweb

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.hayoonjae.asciiweb.engine.Browser
import com.hayoonjae.asciiweb.engine.Line
import com.hayoonjae.asciiweb.engine.Page
import com.hayoonjae.asciiweb.engine.T_CHECK
import com.hayoonjae.asciiweb.engine.T_LINK
import com.hayoonjae.asciiweb.engine.T_SELECT
import com.hayoonjae.asciiweb.engine.T_SUBMIT
import com.hayoonjae.asciiweb.engine.T_TEXT
import com.hayoonjae.asciiweb.engine.Target
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

@OptIn(ExperimentalCoroutinesApi::class)
class BrowserVm(app: Application) : AndroidViewModel(app) {
    private val prefs = app.getSharedPreferences("asciiweb", 0)
    val browser = Browser()

    var page by mutableStateOf<Page?>(null)
    var lines by mutableStateOf<List<Line>>(emptyList())
    var title by mutableStateOf("")
    var urlText by mutableStateOf("")
    var loading by mutableStateOf(false)
    var message by mutableStateOf<String?>(null)
    var canBack by mutableStateOf(false)
    var canForward by mutableStateOf(false)
    var textSp by mutableStateOf(prefs.getFloat("textSp", 12f))
    var images by mutableStateOf(prefs.getBoolean("images", true))
    var color by mutableStateOf(prefs.getBoolean("color", true))

    /** Scroll bookkeeping: the UI reports the first visible line; restoreSeq asks it to jump. */
    var scrollNow = 0
    var restoreTo by mutableIntStateOf(0)
    var restoreSeq by mutableIntStateOf(0)

    var editing by mutableStateOf<Target?>(null)
    var choosing by mutableStateOf<Target?>(null)
    var flash by mutableIntStateOf(0)

    private var colsState by mutableIntStateOf(0)
    private var cols: Int
        get() = colsState
        set(v) { colsState = v }
    private var gen = 0
    private var started = false
    private var relayoutJob: Job? = null
    private val layoutDispatcher = Dispatchers.Default.limitedParallelism(1)

    init {
        browser.opts.images = images
        browser.opts.color = color
    }

    /** Called by the UI once it knows how many columns fit. */
    fun updateCols(c: Int) {
        if (c < 8 || c == cols) return
        cols = c
        if (!started) {
            started = true
            return
        }
        relayout(0)
    }

    val ready get() = cols > 0

    fun start(initial: String?) {
        val first = initial ?: prefs.getString("last", null) ?: "about:home"
        go("GET", first, "", Mode.PUSH, 0)
    }

    fun saveLast() {
        page?.let { if (!it.url.startsWith("about:") && it.method == "GET") prefs.edit().putString("last", it.url).apply() }
    }

    enum class Mode { PUSH, RELOAD, BACK, FORWARD }

    fun open(input: String) = go("GET", Browser.normalize(input), "", Mode.PUSH, 0)

    private fun go(method: String, url: String, body: String, mode: Mode, restore: Int) {
        val my = ++gen
        loading = true
        message = null
        viewModelScope.launch(Dispatchers.IO) {
            try {
                val p = browser.fetch(method, url, body)
                withContext(layoutDispatcher) { p.layout(cols) }
                withContext(Dispatchers.Main) {
                    if (my != gen) return@withContext
                    when (mode) {
                        Mode.PUSH -> browser.commit(p, true, scrollNow)
                        Mode.RELOAD -> browser.commit(p, false, scrollNow)
                        Mode.BACK -> browser.moveTo(-1, p, scrollNow)
                        Mode.FORWARD -> browser.moveTo(1, p, scrollNow)
                    }
                    show(p, restore)
                }
            } catch (e: Exception) {
                withContext(Dispatchers.Main) {
                    if (my == gen) {
                        loading = false
                        message = e.message ?: e.javaClass.simpleName
                    }
                }
            }
        }
    }

    private fun show(p: Page, restore: Int) {
        page = p
        lines = p.lines
        title = p.title
        urlText = if (p.url.startsWith("about:")) "" else p.url
        loading = false
        canBack = browser.canBack
        canForward = browser.canForward
        restoreTo = restore
        restoreSeq++
        p.loadImages(browser.client) { relayout(120) }
    }

    fun relayout(delayMs: Long) {
        val p = page ?: return
        relayoutJob?.cancel()
        relayoutJob = viewModelScope.launch(layoutDispatcher) {
            if (delayMs > 0) delay(delayMs)
            p.layout(cols)
            withContext(Dispatchers.Main) { if (page === p) lines = p.lines }
        }
    }

    private fun step(d: Int) {
        val s = browser.step(d) ?: return
        val mode = if (d < 0) Mode.BACK else Mode.FORWARD
        if (s.cached != null) {
            browser.moveTo(d, s.cached, scrollNow)
            s.cached.layout(cols)
            show(s.cached, s.scroll)
        } else go(s.method, s.url, s.body, mode, s.scroll)
    }

    fun back() = step(-1)
    fun forward() = step(1)

    fun reload() {
        val p = page ?: return
        go(p.method, p.url, p.body, Mode.RELOAD, scrollNow)
    }

    fun toggleImages() {
        images = !images
        browser.opts.images = images
        prefs.edit().putBoolean("images", images).apply()
        page?.loadImages(browser.client) { relayout(120) }
        relayout(0)
    }

    fun toggleColor() {
        color = !color
        browser.opts.color = color
        prefs.edit().putBoolean("color", color).apply()
        relayout(0)
    }

    fun changeTextSize(delta: Float) {
        textSp = (textSp + delta).coerceIn(7f, 24f)
        prefs.edit().putFloat("textSp", textSp).apply()
    }

    // ---- taps ----

    fun tap(id: Int) {
        val p = page ?: return
        val t = p.targets.getOrNull(id - 1) ?: return
        flash = id
        viewModelScope.launch {
            delay(220)
            if (flash == id) flash = 0
        }
        when (t.kind) {
            T_LINK -> {
                val cur = p.url.substringBefore('#')
                if (t.url.startsWith("#") || t.url.substringBefore('#') == cur && t.url.contains('#')) message = "anchor on this page"
                else open(t.url)
            }
            T_TEXT -> editing = t
            T_CHECK -> {
                val n = t.node!!
                if (n.attr("type").equals("radio", true)) p.setValue(n, "on")
                else p.setValue(n, if (p.value(n).isNotEmpty()) "" else "on")
                relayout(0)
            }
            T_SELECT -> choosing = t
            T_SUBMIT -> submit(t)
        }
    }

    private fun submit(t: Target) {
        val p = page ?: return
        val form = t.form
        if (form == null) {
            message = "this button needs JavaScript"
            return
        }
        val (m, url, body) = p.submitRequest(form, t.node)
        go(m, url, body, Mode.PUSH, 0)
    }

    fun commitEdit(text: String) {
        val t = editing ?: return
        editing = null
        val p = page ?: return
        p.setValue(t.node!!, text)
        relayout(0)
        if (t.form != null && t.node.tagName() != "textarea") submit(Target(T_SUBMIT, node = null, form = t.form))
    }

    fun commitSelect(i: Int) {
        val t = choosing ?: return
        choosing = null
        page?.selectOption(t.node!!, i)
        relayout(0)
    }
}
