package com.hayoonjae.asciiweb

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas as NativeCanvas
import android.graphics.Color as AColor
import android.graphics.Matrix
import android.view.MotionEvent
import android.webkit.CookieManager
import android.webkit.WebChromeClient
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.layout.positionInWindow
import androidx.compose.ui.layout.positionOnScreen
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.foundation.layout.requiredSize
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.window.DialogWindowProvider
import androidx.compose.ui.platform.LocalView
import android.view.Gravity
import android.view.PixelCopy
import android.view.WindowManager
import kotlin.coroutines.resume
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.input.pointer.pointerInteropFilter
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.unit.sp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.repeatOnLifecycle
import com.hayoonjae.asciiweb.engine.LiveSettings
import com.hayoonjae.asciiweb.engine.TEXT_SCRIPT
import com.hayoonjae.asciiweb.engine.Word
import com.hayoonjae.asciiweb.engine.convertScreen
import com.hayoonjae.asciiweb.engine.overlayText
import com.hayoonjae.asciiweb.engine.subPixels
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext

object LiveWeb {
    private const val DESKTOP_UA =
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

    @SuppressLint("SetJavaScriptEnabled")
    fun apply(w: WebView, s: LiveSettings, ctx: Context, cssWidth: Int) {
        val st = w.settings
        st.javaScriptEnabled = true
        st.domStorageEnabled = true
        st.useWideViewPort = true
        st.loadWithOverviewMode = false
        st.mediaPlaybackRequiresUserGesture = true
        st.setSupportMultipleWindows(false)
        st.mixedContentMode = WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE
        st.userAgentString = if (s.mobile) WebSettings.getDefaultUserAgent(ctx) else DESKTOP_UA
        // layout width in CSS px = view width in dp * 100 / scale
        val dm = ctx.resources.displayMetrics
        val viewDp = (w.width.takeIf { it > 0 } ?: dm.widthPixels) / dm.density
        w.setInitialScale((viewDp * 100f / (if (cssWidth > 0) cssWidth else 360)).toInt().coerceIn(20, 400))
    }

    @SuppressLint("SetJavaScriptEnabled")
    fun create(ctx: Context, vm: LiveVm): WebView {
        val w = WebView(ctx)
        w.setBackgroundColor(AColor.WHITE)
        CookieManager.getInstance().setAcceptCookie(true)
        CookieManager.getInstance().setAcceptThirdPartyCookies(w, true)
        w.webViewClient = object : WebViewClient() {
            override fun onPageStarted(v: WebView, url: String, f: Bitmap?) {
                vm.loading = true
                vm.urlText = url
            }

            override fun onPageFinished(v: WebView, url: String) {
                vm.loading = false
                vm.urlText = url
                vm.canBack = v.canGoBack()
            }

            override fun doUpdateVisitedHistory(v: WebView, url: String, isReload: Boolean) {
                vm.urlText = url
                vm.canBack = v.canGoBack()
            }
        }
        w.webChromeClient = object : WebChromeClient() {
            override fun onReceivedTitle(v: WebView, t: String) {
                vm.title = t
            }
        }
        apply(w, vm.settings, ctx, if (vm.cssWidth > 0) vm.cssWidth else 360)
        vm.webView?.destroy()
        vm.webView = w
        w.loadUrl(vm.initialUrl ?: vm.urlText.ifEmpty { vm.settings.home })
        vm.initialUrl = null
        return w
    }
}

private fun Context.findActivity(): android.app.Activity? {
    var c = this
    while (c is android.content.ContextWrapper) {
        if (c is android.app.Activity) return c
        c = c.baseContext
    }
    return null
}

/** Cheap fingerprint of a capture, used to skip work when the page did not change. */
private fun fingerprint(px: IntArray): Int {
    var h = 17
    var i = 0
    while (i < px.size) {
        h = h * 31 + px[i]
        i += 13
    }
    return h
}

/**
 * The page is shown by a normal WebView (GPU, native scrolling). Its composited pixels are copied with
 * PixelCopy, turned into characters, and shown in a separate click-through overlay window on top.
 * That keeps memory low and never captures the overlay itself.
 */
@Composable
fun LiveView(vm: LiveVm, textSp: Float, modifier: Modifier = Modifier) {
    val ctx = LocalContext.current
    val density = LocalDensity.current
    val px = with(density) { textSp.sp.toPx() }
    val paints = remember(px) { GridPaints(px) }
    val web = remember { LiveWeb.create(ctx, vm) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    var screenX by remember { mutableIntStateOf(0) }
    var screenY by remember { mutableIntStateOf(0) }
    var winX by remember { mutableIntStateOf(0) }
    var winY by remember { mutableIntStateOf(0) }
    BoxWithConstraints(
        modifier.fillMaxSize().onGloballyPositioned {
            val sp = it.positionOnScreen()
            val wp = it.positionInWindow()
            screenX = sp.x.toInt(); screenY = sp.y.toInt()
            winX = wp.x.toInt(); winY = wp.y.toInt()
        },
    ) {
        val vw = constraints.maxWidth
        val vh = constraints.maxHeight
        val cols = (vw / paints.cellW).toInt().coerceAtLeast(8)
        val rows = (vh / paints.lineH).toInt().coerceAtLeast(4)
        val s = vm.settings

        // the page is rendered small (less GPU memory) and shown scaled up; touches map through the scale
        val rs = (s.renderScale / 100f).coerceIn(0.2f, 1f)
        val webW = (vw * rs).toInt().coerceAtLeast(64)
        val webH = (vh * rs).toInt().coerceAtLeast(64)
        AndroidView(
            factory = { web },
            modifier = Modifier
                .requiredSize(with(density) { webW.toDp() }, with(density) { webH.toDp() })
                .graphicsLayer(scaleX = vw / webW.toFloat(), scaleY = vh / webH.toFloat(), transformOrigin = TransformOrigin(0f, 0f)),
            update = {},
        )

        // auto width: about 5 CSS px per cell, so a character spans one to two cells
        val cssW = if (s.pageWidth == 0) maxOf(320, cols * 5) else s.pageWidth
        LaunchedEffect(vw, cssW, s.mobile, s.renderScale) {
            delay(150)
            val changed = vm.cssWidth != 0 && vm.cssWidth != cssW
            vm.cssWidth = cssW
            LiveWeb.apply(web, vm.settings, ctx, cssW)
            if (changed) web.reload()
        }

        // screen capture -> characters; idle pages cost almost nothing
        LaunchedEffect(cols, rows, s.style, s.color, s.bold, s.invert, s.contrast, s.quality, s.fps, s.renderScale, vm.epoch) {
            val activity = ctx.findActivity() ?: return@LaunchedEffect
            val handler = android.os.Handler(android.os.Looper.getMainLooper())
            lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
                var bmp: Bitmap? = null
                var pixels = IntArray(0)
                var lastHash = 0
                var haveLast = false
                var idle = 0
                while (true) {
                    val (sx, sy) = subPixels(s.style)
                    val bw = cols * sx * s.quality
                    val bh = rows * sy * s.quality
                    if (web.width > 0 && web.height > 0 && vw > 0) {
                        if (bmp == null || bmp.width != bw || bmp.height != bh) {
                            bmp = Bitmap.createBitmap(bw, bh, Bitmap.Config.ARGB_8888)
                            pixels = IntArray(bw * bh)
                            haveLast = false
                        }
                        val src = android.graphics.Rect(winX, winY, winX + vw, winY + vh)
                        val ok = kotlinx.coroutines.suspendCancellableCoroutine<Boolean> { cont ->
                            PixelCopy.request(activity.window, src, bmp, { r -> if (cont.isActive) cont.resume(r == PixelCopy.SUCCESS) {} }, handler)
                        }
                        if (ok) {
                            bmp.getPixels(pixels, 0, bw, 0, 0, bw, bh)
                            val h = fingerprint(pixels)
                            if (!haveLast || h != lastHash) {
                                haveLast = true
                                lastHash = h
                                idle = 0
                                vm.stamp++
                                val snap = pixels.copyOf()
                                vm.art = withContext(Dispatchers.Default) { convertScreen(snap, bw, bh, cols, rows, s) }
                            } else idle++
                        }
                    }
                    delay(if (idle > 6) 450L else 1000L / s.fps)
                }
            }
        }

        // real page text: read only after the screen changed
        LaunchedEffect(s.text) {
            lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
                var seen = -1
                while (true) {
                    if (s.text && vm.stamp != seen) {
                        seen = vm.stamp
                        val res = kotlinx.coroutines.suspendCancellableCoroutine<String?> { cont ->
                            web.evaluateJavascript(TEXT_SCRIPT) { r -> if (cont.isActive) cont.resume(r) {} }
                        }
                        vm.words = withContext(Dispatchers.Default) { parseWords(res) }
                        delay(350)
                    } else delay(120)
                }
            }
        }
        if (!s.text && vm.words.isNotEmpty()) vm.words = emptyList()

        val lines = remember(vm.art, vm.words, s.text, s.bold, s.color) {
            if (s.text) overlayText(vm.art, vm.words, s) else vm.art
        }
        OverlayWindow(screenX, screenY, vw, vh) {
            Canvas(Modifier.fillMaxSize()) {
                drawRect(androidx.compose.ui.graphics.Color(BG_COLOR))
                drawIntoCanvas { cv ->
                    val nc = cv.nativeCanvas
                    for ((i, line) in lines.withIndex()) {
                        nc.save()
                        nc.translate(0f, i * paints.lineH)
                        paints.drawLine(nc, line, 0)
                        nc.restore()
                    }
                }
            }
        }
    }
}

/** A full-size window above the page that lets every touch and key fall through to the page. */
@Composable
private fun OverlayWindow(x: Int, y: Int, w: Int, h: Int, content: @Composable () -> Unit) {
    if (w <= 0 || h <= 0) return
    Dialog(
        onDismissRequest = {},
        properties = DialogProperties(
            dismissOnBackPress = false, dismissOnClickOutside = false,
            usePlatformDefaultWidth = false, decorFitsSystemWindows = false,
        ),
    ) {
        val window = (LocalView.current.parent as? DialogWindowProvider)?.window
        SideEffect {
            window?.let {
                it.setBackgroundDrawable(android.graphics.drawable.ColorDrawable(0))
                it.clearFlags(WindowManager.LayoutParams.FLAG_DIM_BEHIND)
                it.addFlags(
                    WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE or WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                        WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS or WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN,
                )
                val lp = it.attributes
                lp.gravity = Gravity.TOP or Gravity.START
                lp.x = x
                lp.y = y
                lp.width = w
                lp.height = h
                lp.dimAmount = 0f
                it.attributes = lp
            }
        }
        content()
    }
}

/** Decodes evaluateJavascript's result (a JSON string literal holding our JSON) into words. */
private fun parseWords(res: String?): List<Word> {
    if (res.isNullOrEmpty() || res == "null") return emptyList()
    return try {
        val inner = org.json.JSONTokener(res).nextValue() as? String ?: return emptyList()
        val o = org.json.JSONObject(inner)
        val vw = o.getDouble("w").toFloat()
        val vh = o.getDouble("h").toFloat()
        val a = o.getJSONArray("words")
        List(a.length()) { i ->
            val j = a.getJSONObject(i)
            Word(
                j.getString("t"), j.getDouble("x").toFloat(), j.getDouble("y").toFloat(),
                j.getDouble("w").toFloat(), j.getDouble("h").toFloat(),
                j.getInt("r"), j.getInt("g"), j.getInt("b"), vw, vh,
            )
        }
    } catch (e: Exception) {
        emptyList()
    }
}
