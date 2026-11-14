package com.hayoonjae.asciiweb

import android.app.Application
import android.webkit.WebView
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import com.hayoonjae.asciiweb.engine.Line
import com.hayoonjae.asciiweb.engine.LiveSettings

/** State of the real-render mode: a WebView whose screen is drawn as characters. */
class LiveVm(app: Application) : AndroidViewModel(app) {
    private val prefs = app.getSharedPreferences("asciiweb", 0)

    var settings by mutableStateOf(load())
        private set
    var art by mutableStateOf<List<Line>>(emptyList())
    var stamp = 0 // bumps when the captured screen changed
    var urlText by mutableStateOf("")
    var title by mutableStateOf("")
    var loading by mutableStateOf(false)
    var canBack by mutableStateOf(false)
    var showSettings by mutableStateOf(false)
    var epoch by mutableIntStateOf(0) // bumps when the capture must be redone
    var webView: WebView? = null
    var words by mutableStateOf<List<com.hayoonjae.asciiweb.engine.Word>>(emptyList())
    var cssWidth = 0
    var initialUrl: String? = null

    private fun load(): LiveSettings {
        val d = LiveSettings()
        return LiveSettings(
            real = prefs.getBoolean("l_real", d.real),
            style = prefs.getString("l_style", d.style) ?: d.style,
            color = prefs.getBoolean("l_color", d.color),
            bold = prefs.getBoolean("l_bold", d.bold),
            text = prefs.getBoolean("l_text", d.text),
            invert = prefs.getString("l_invert", d.invert) ?: d.invert,
            contrast = prefs.getInt("l_contrast", d.contrast),
            pageWidth = prefs.getInt("l_width", d.pageWidth),
            quality = prefs.getInt("l_quality", d.quality),
            renderScale = prefs.getInt("l_scale", d.renderScale),
            fps = prefs.getInt("l_fps", d.fps),
            mobile = prefs.getBoolean("l_mobile", d.mobile),
            search = prefs.getString("l_search", d.search) ?: d.search,
            home = prefs.getString("l_home", d.home) ?: d.home,
        )
    }

    fun update(s: LiveSettings) {
        val reload = s.mobile != settings.mobile
        settings = s
        prefs.edit()
            .putBoolean("l_real", s.real).putString("l_style", s.style).putBoolean("l_color", s.color)
            .putBoolean("l_bold", s.bold).putBoolean("l_text", s.text).putString("l_invert", s.invert).putInt("l_contrast", s.contrast)
            .putInt("l_width", s.pageWidth).putInt("l_quality", s.quality).putInt("l_scale", s.renderScale).putInt("l_fps", s.fps)
            .putBoolean("l_mobile", s.mobile).putString("l_search", s.search).putString("l_home", s.home)
            .apply()
        epoch++
        if (reload) {
            LiveWeb.apply(webView ?: return, s, getApplication(), cssWidth)
            webView?.reload()
        }
    }

    fun open(input: String) {
        webView?.loadUrl(settings.resolve(input))
    }

    fun saveLast() {
        val u = urlText
        if (u.startsWith("http")) prefs.edit().putString("l_last", u).apply()
    }

    fun lastUrl(): String? = prefs.getString("l_last", null)

    override fun onCleared() {
        webView?.destroy()
        webView = null
    }
}
