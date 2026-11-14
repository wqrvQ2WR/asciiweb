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

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        ThumbDecoder.install()
        enableEdgeToEdge()
        setContent { MaterialTheme(colorScheme = Scheme) { App(vm) } }
        if (savedInstanceState == null) pendingStart = intent?.dataString
    }

    private var pendingStart: String? = null

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        intent.dataString?.let { if (vm.ready) vm.open(it) else pendingStart = it }
    }

    override fun onStop() {
        vm.saveLast()
        super.onStop()
    }

    @Composable
    fun App(vm: BrowserVm) {
        var kickoff by remember { mutableStateOf(false) }
        if (vm.ready && !kickoff && vm.page == null && !vm.loading) {
            kickoff = true
            vm.start(pendingStart)
            pendingStart = null
        }
        BackHandler(enabled = vm.canBack) { vm.back() }
        Column(Modifier.fillMaxSize().background(Bg).safeDrawingPadding()) {
            TopBar(vm)
            Box(Modifier.fillMaxWidth().height(2.dp).background(if (vm.loading) Brand else SolidColor(Color(0xFF2A2A3A))))
            Box(Modifier.weight(1f).fillMaxWidth()) {
                GridView(vm)
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
    }

    @Composable
    private fun TopBar(vm: BrowserVm) {
        var menu by remember { mutableStateOf(false) }
        var tf by remember { mutableStateOf(TextFieldValue(vm.urlText)) }
        var focused by remember { mutableStateOf(false) }
        val focus = LocalFocusManager.current
        if (!focused && tf.text != vm.urlText) tf = TextFieldValue(vm.urlText)
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
            IconButton(onClick = { vm.back() }, enabled = vm.canBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, "back", tint = if (vm.canBack) Color.White else Color(0xFF555566))
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
                        vm.open(tf.text)
                    }),
                    modifier = Modifier.fillMaxWidth().onFocusChanged { focused = it.isFocused },
                    decorationBox = { inner ->
                        if (tf.text.isEmpty()) Text("search or type a URL", color = Color(0xFF8A8AA0), fontSize = 14.sp)
                        inner()
                    },
                )
            }
            IconButton(onClick = { vm.reload() }) { Icon(Icons.Filled.Refresh, "reload", tint = Color.White) }
            Box {
                IconButton(onClick = { menu = true }) { Icon(Icons.Filled.MoreVert, "menu", tint = Color.White) }
                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                    DropdownMenuItem(text = { Text("Home") }, leadingIcon = { Icon(Icons.Filled.Home, null) }, onClick = { menu = false; vm.open("about:home") })
                    DropdownMenuItem(text = { Text("Forward") }, enabled = vm.canForward, onClick = { menu = false; vm.forward() })
                    DropdownMenuItem(text = { Text(if (vm.images) "Images: on" else "Images: off") }, onClick = { vm.toggleImages() })
                    DropdownMenuItem(text = { Text(if (vm.color) "Color: on" else "Color: off") }, onClick = { vm.toggleColor() })
                    DropdownMenuItem(text = { Text("Text size +") }, onClick = { vm.changeTextSize(1f) })
                    DropdownMenuItem(text = { Text("Text size -") }, onClick = { vm.changeTextSize(-1f) })
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
}
