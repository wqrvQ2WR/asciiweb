package com.hayoonjae.asciiweb

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

val Brand = Brush.horizontalGradient(listOf(Color(0xFFFF4D9D), Color(0xFF9B5CFF), Color(0xFF3DA9FF)))
private val Bg = Color(BG_COLOR)
private val Panel = Color(0xFF15151F)
private val Field = Color(0xFF242434)

private val Scheme = darkColorScheme(
    primary = Color(0xFFFF4D9D),
    secondary = Color(0xFF9B5CFF),
    tertiary = Color(0xFF3DA9FF),
    background = Bg,
    surface = Panel,
)

class MainActivity : ComponentActivity() {
    private val vm: BrowserVm by viewModels()
    private val live: LiveVm by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        ThumbDecoder.install()
        // draw() renders the whole document, so newly scrolled areas are never captured blank
        runCatching { android.webkit.WebView.enableSlowWholeDocumentDraw() }
        enableEdgeToEdge()
        if (savedInstanceState == null) pendingStart = intent?.dataString
        live.initialUrl = pendingStart ?: live.lastUrl()
        setContent { MaterialTheme(colorScheme = Scheme) { App(vm) } }
    }

    private var pendingStart: String? = null

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        intent.dataString?.let {
            if (live.settings.real) live.open(it)
            else if (vm.ready) vm.open(it) else pendingStart = it
        }
    }

    override fun onStop() {
        vm.saveLast()
        live.saveLast()
        super.onStop()
    }

    @Composable
    fun App(vm: BrowserVm) {
        val real = live.settings.real
        var kickoff by remember { mutableStateOf(false) }
        if (!real && vm.ready && !kickoff && vm.page == null && !vm.loading) {
            kickoff = true
            vm.start(pendingStart)
            pendingStart = null
        }
        BackHandler(enabled = if (real) live.canBack else vm.canBack) {
            if (real) live.webView?.goBack() else vm.back()
        }
        Column(Modifier.fillMaxSize().background(Bg).safeDrawingPadding()) {
            if (real) {
                TopBar(
                    urlText = live.urlText, canBack = live.canBack, canForward = false,
                    onBack = { live.webView?.goBack() }, onGo = { live.open(it) },
                    onReload = { live.webView?.reload() }, onForward = { live.webView?.goForward() },
                    onHome = { live.open(live.settings.home) },
                    vm = vm,
                )
            } else {
                TopBar(
                    urlText = vm.urlText, canBack = vm.canBack, canForward = vm.canForward,
                    onBack = { vm.back() }, onGo = { vm.open(it) }, onReload = { vm.reload() },
                    onForward = { vm.forward() }, onHome = { vm.open("about:home") },
                    vm = vm,
                )
            }
            val loading = if (real) live.loading else vm.loading
            Box(Modifier.fillMaxWidth().height(2.dp).background(if (loading) Brand else SolidColor(Color(0xFF2A2A3A))))
            Box(Modifier.weight(1f).fillMaxWidth()) {
                if (real) LiveView(live, vm.textSp) else GridView(vm)
                vm.message?.let {
                    Text(
                        it,
                        color = Color.White,
                        fontSize = 13.sp,
                        modifier = Modifier
                            .align(Alignment.BottomCenter)
                            .padding(12.dp)
                            .background(Brush.horizontalGradient(listOf(Color(0xE0E02F82), Color(0xE01F86E0))), RoundedCornerShape(12.dp))
                            .clickable { vm.message = null }
                            .padding(horizontal = 14.dp, vertical = 8.dp),
                    )
                }
            }
        }
        EditDialog(vm)
        SelectDialog(vm)
        SettingsDialog(vm)
    }

    @Composable
    private fun TopBar(
        urlText: String, canBack: Boolean, canForward: Boolean,
        onBack: () -> Unit, onGo: (String) -> Unit, onReload: () -> Unit,
        onForward: () -> Unit, onHome: () -> Unit, vm: BrowserVm,
    ) {
        var menu by remember { mutableStateOf(false) }
        var tf by remember { mutableStateOf(TextFieldValue(urlText)) }
        var focused by remember { mutableStateOf(false) }
        val focus = LocalFocusManager.current
        if (!focused && tf.text != urlText) tf = TextFieldValue(urlText)
        LaunchedEffect(focused) {
            if (focused) {
                kotlinx.coroutines.delay(80)
                tf = tf.copy(selection = TextRange(0, tf.text.length))
            }
        }
        Row(
            Modifier.fillMaxWidth().background(Panel).padding(horizontal = 4.dp, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = onBack, enabled = canBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, "back", tint = if (canBack) Color.White else Color(0xFF555566))
            }
            Box(
                Modifier.weight(1f).height(40.dp).background(Field, RoundedCornerShape(20.dp)).padding(horizontal = 14.dp),
                contentAlignment = Alignment.CenterStart,
            ) {
                BasicTextField(
                    value = tf,
                    onValueChange = { tf = it },
                    singleLine = true,
                    textStyle = TextStyle(color = Color.White, fontSize = 14.sp),
                    cursorBrush = Brand,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                    keyboardActions = KeyboardActions(onGo = {
                        focus.clearFocus()
                        onGo(tf.text)
                    }),
                    modifier = Modifier.fillMaxWidth().onFocusChanged { focused = it.isFocused },
                    decorationBox = { inner ->
                        if (tf.text.isEmpty()) Text("search or type a URL", color = Color(0xFF8A8AA0), fontSize = 14.sp)
                        inner()
                    },
                )
            }
            IconButton(onClick = onReload) { Icon(Icons.Filled.Refresh, "reload", tint = Color.White) }
            Box {
                IconButton(onClick = { menu = true }) { Icon(Icons.Filled.MoreVert, "menu", tint = Color.White) }
                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                    DropdownMenuItem(text = { Text("Home") }, leadingIcon = { Icon(Icons.Filled.Home, null) }, onClick = { menu = false; onHome() })
                    DropdownMenuItem(text = { Text("Forward") }, enabled = canForward || live.settings.real, onClick = { menu = false; onForward() })
                    DropdownMenuItem(text = { Text("Settings") }, onClick = { menu = false; live.showSettings = true })
                }
            }
        }
    }

    @Composable
    private fun EditDialog(vm: BrowserVm) {
        val t = vm.editing ?: return
        val node = t.node!!
        var text by remember(t) { mutableStateOf(TextFieldValue(vm.page?.value(node) ?: "")) }
        val multi = node.tagName() == "textarea"
        AlertDialog(
            onDismissRequest = { vm.editing = null },
            title = { Text(node.attr("name").ifEmpty { "Text" }) },
            text = {
                BasicTextField(
                    value = text,
                    onValueChange = { text = it },
                    singleLine = !multi,
                    textStyle = TextStyle(color = Color.White, fontSize = 16.sp),
                    cursorBrush = Brand,
                    keyboardOptions = KeyboardOptions(imeAction = if (multi) ImeAction.Default else ImeAction.Go),
                    keyboardActions = KeyboardActions(onGo = { vm.commitEdit(text.text) }),
                    modifier = Modifier.fillMaxWidth().background(Field, RoundedCornerShape(10.dp)).padding(12.dp),
                )
            },
            confirmButton = { TextButton(onClick = { vm.commitEdit(text.text) }) { Text(if (multi) "OK" else "Go") } },
            dismissButton = { TextButton(onClick = { vm.editing = null }) { Text("Cancel") } },
        )
    }

    @Composable
    private fun SelectDialog(vm: BrowserVm) {
        val t = vm.choosing ?: return
        val p = vm.page ?: return
        val opts = p.options(t.node!!)
        AlertDialog(
            onDismissRequest = { vm.choosing = null },
            confirmButton = {},
            dismissButton = { TextButton(onClick = { vm.choosing = null }) { Text("Cancel") } },
            text = {
                Column {
                    opts.forEachIndexed { i, o ->
                        Text(
                            p.optionText(o),
                            color = if (i == p.selectedIndex(t.node)) Color(0xFFFF4D9D) else Color.White,
                            modifier = Modifier.fillMaxWidth().clickable { vm.commitSelect(i) }.padding(vertical = 12.dp),
                        )
                    }
                }
            },
        )
    }

    @Composable
    private fun SettingRow(label: String, value: String, onClick: () -> Unit) {
        Row(
            Modifier.fillMaxWidth().clickable(onClick = onClick).padding(vertical = 11.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(label, color = Color.White, fontSize = 14.sp, modifier = Modifier.weight(1f))
            Text(value, color = Color(0xFFFF4D9D), fontSize = 14.sp)
        }
    }

    @Composable
    private fun SettingsDialog(vm: BrowserVm) {
        if (!live.showSettings) return
        val s = live.settings
        fun <T> next(list: List<T>, cur: T): T = list[(list.indexOf(cur).coerceAtLeast(0) + 1) % list.size]
        AlertDialog(
            onDismissRequest = { live.showSettings = false },
            title = { Text("Settings") },
            confirmButton = { TextButton(onClick = { live.showSettings = false }) { Text("Close") } },
            text = {
                Column(Modifier.verticalScroll(rememberScrollState())) {
                    SettingRow("Rendering", if (s.real) "real page" else "text reflow") { live.update(s.copy(real = !s.real)) }
                    SettingRow("Style", s.style + "  " + com.hayoonjae.asciiweb.engine.LiveSettings.STYLE_DESC[s.style]) {
                        live.update(s.copy(style = next(com.hayoonjae.asciiweb.engine.LiveSettings.STYLES, s.style)))
                    }
                    SettingRow("Color", if (s.color) "on" else "off") { live.update(s.copy(color = !s.color)) }
                    SettingRow("Real text overlay", if (s.text) "on" else "off") { live.update(s.copy(text = !s.text)) }
                    SettingRow("Bold characters", if (s.bold) "on" else "off") { live.update(s.copy(bold = !s.bold)) }
                    SettingRow("Invert (dark ink on light page)", s.invert) {
                        live.update(s.copy(invert = next(com.hayoonjae.asciiweb.engine.LiveSettings.INVERTS, s.invert)))
                    }
                    SettingRow("Contrast", "%+d".format(s.contrast)) {
                        live.update(s.copy(contrast = if (s.contrast >= 4) -4 else s.contrast + 1))
                    }
                    SettingRow("Page width (px)", if (s.pageWidth == 0) "auto" else s.pageWidth.toString()) {
                        live.update(s.copy(pageWidth = next(com.hayoonjae.asciiweb.engine.LiveSettings.WIDTHS, s.pageWidth)))
                    }
                    SettingRow("Sampling quality", "x${s.quality}") { live.update(s.copy(quality = if (s.quality >= 3) 1 else s.quality + 1)) }
                    SettingRow("Frame rate (fps)", s.fps.toString()) {
                        live.update(s.copy(fps = next(com.hayoonjae.asciiweb.engine.LiveSettings.FPSES, s.fps)))
                    }
                    SettingRow("Mobile sites", if (s.mobile) "on" else "off") { live.update(s.copy(mobile = !s.mobile)) }
                    SettingRow("Search engine", s.search) {
                        live.update(s.copy(search = next(com.hayoonjae.asciiweb.engine.LiveSettings.SEARCHES, s.search)))
                    }
                    SettingRow("Home page", s.home.removePrefix("https://")) {
                        if (live.urlText.startsWith("http")) live.update(s.copy(home = live.urlText))
                    }
                    SettingRow("Text size (smaller = finer)", "%.0f".format(vm.textSp)) {
                        vm.textSp = if (vm.textSp >= 20f) 7f else vm.textSp + 1f
                        vm.changeTextSize(0f)
                        live.epoch++
                    }
                }
            },
        )
    }
}
