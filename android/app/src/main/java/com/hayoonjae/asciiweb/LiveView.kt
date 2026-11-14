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
import com.hayoonjae.asciiweb.engine.convertScreen
import com.hayoonjae.asciiweb.engine.subPixels
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext

object LiveWeb {
    private const val DESKTOP_UA =
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

    @SuppressLint("SetJavaScriptEnabled")
    fun apply(w: WebView, s: LiveSettings, ctx: Context) {
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
        w.setInitialScale((viewDp * 100f / s.pageWidth).toInt().coerceIn(20, 400))
    }

    @SuppressLint("SetJavaScriptEnabled")
    fun create(ctx: Context, vm: LiveVm): WebView {
        val w = WebView(ctx)
        w.setBackgroundColor(AColor.WHITE)
        w.setLayerType(android.view.View.LAYER_TYPE_SOFTWARE, null) // lets draw() capture it reliably, also after scrolling
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
        apply(w, vm.settings, ctx)
        vm.webView?.destroy()
        vm.webView = w
        w.loadUrl(vm.initialUrl ?: vm.urlText.ifEmpty { vm.settings.home })
        vm.initialUrl = null
        return w
    }
}

@OptIn(ExperimentalComposeUiApi::class)
@Composable
fun LiveView(vm: LiveVm, textSp: Float, modifier: Modifier = Modifier) {
    val ctx = LocalContext.current
    val density = LocalDensity.current
    val px = with(density) { textSp.sp.toPx() }
    val paints = remember(px) { GridPaints(px) }
    val web = remember { LiveWeb.create(ctx, vm) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    BoxWithConstraints(modifier.fillMaxSize()) {
        val vw = constraints.maxWidth
        val vh = constraints.maxHeight
        val cols = (vw / paints.cellW).toInt().coerceAtLeast(8)
        val rows = (vh / paints.lineH).toInt().coerceAtLeast(4)
        val s = vm.settings
        // map overlay coordinates back to WebView coordinates
        val mx = vw / (cols * paints.cellW)
        val my = vh / (rows * paints.lineH)

        AndroidView(factory = { web }, modifier = Modifier.fillMaxSize(), update = {})
        LaunchedEffect(vw, s.pageWidth, s.mobile) {
            delay(150)
            LiveWeb.apply(web, vm.settings, ctx)
        }

        LaunchedEffect(cols, rows, s, vm.epoch) {
            lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
                var bmp: Bitmap? = null
                var pixels = IntArray(0)
                while (true) {
                    val (sx, sy) = subPixels(s.style)
                    val bw = cols * sx * s.quality
                    val bh = rows * sy * s.quality
                    if (web.width > 0 && web.height > 0) {
                        if (bmp == null || bmp.width != bw || bmp.height != bh) {
                            bmp = Bitmap.createBitmap(bw, bh, Bitmap.Config.ARGB_8888)
                            pixels = IntArray(bw * bh)
                        }
                        val c = NativeCanvas(bmp)
                        c.drawColor(AColor.WHITE)
                        c.scale(bw / web.width.toFloat(), bh / web.height.toFloat())
                        c.translate(-web.scrollX.toFloat(), -web.scrollY.toFloat())
                        web.draw(c)
                        bmp.getPixels(pixels, 0, bw, 0, 0, bw, bh)
                        val snap = pixels.copyOf()
                        val grid = withContext(Dispatchers.Default) { convertScreen(snap, bw, bh, cols, rows, s) }
                        if (grid != vm.lines) vm.lines = grid
                    }
                    delay(1000L / s.fps)
                }
            }
        }

        val lines = vm.lines
        Canvas(
            Modifier
                .fillMaxSize()
                .pointerInteropFilter { ev ->
                    val m = MotionEvent.obtain(ev)
                    val mat = Matrix()
                    mat.setScale(mx, my)
                    m.transform(mat)
                    web.requestFocus()
                    web.dispatchTouchEvent(m)
                    m.recycle()
                    true
                },
        ) {
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
