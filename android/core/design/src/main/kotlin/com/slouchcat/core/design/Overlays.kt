package com.slouchcat.core.design

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.BottomSheetDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Snackbar
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.paneTitle
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog

/**
 * `sheet(...)`: a sheet from below on a phone, a centred dialog from 700 dp (the mockup does the
 * same with its container query). Content: optionally [SheetHeadIcon], then [SheetTitle], text and
 * [SheetActions]. Back and a tap outside call [onDismiss]. [title] names the pane for a screen reader.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SlouchcatSheet(title: String, onDismiss: () -> Unit, content: @Composable ColumnScope.() -> Unit) {
    val c = SlouchcatTheme.colors
    val body: @Composable () -> Unit = {
        Column(
            Modifier
                .fillMaxWidth()
                .semantics { paneTitle = title }
                .verticalScroll(rememberScrollState())
                .padding(bottom = 28.dp),
            content = content,
        )
    }
    if (isWideLayout()) {
        Dialog(onDismissRequest = onDismiss) {
            val shape = RoundedCornerShape(28.dp)
            Box(
                Modifier
                    .widthIn(max = 640.dp)
                    .clip(shape)
                    .background(c.elev)
                    .border(BorderStroke(1.dp, c.line), shape)
                    .padding(top = 22.dp),
            ) { body() }
        }
    } else {
        ModalBottomSheet(
            onDismissRequest = onDismiss,
            sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
            containerColor = c.elev,
            contentColor = c.tx,
            scrimColor = c.scrim,
            shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
            dragHandle = { BottomSheetDefaults.DragHandle(color = c.ctl, width = 36.dp, height = 4.dp) },
        ) {
            Box(Modifier.navigationBarsPadding()) { body() }
        }
    }
}

/** `.su-shead`: the round icon above the title of a sheet; [warning] in the warning colour. */
@Composable
fun SheetHeadIcon(icon: SlouchcatIcon, warning: Boolean = false) {
    val c = SlouchcatTheme.colors
    Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
        Box(
            Modifier
                .padding(top = 6.dp, bottom = 8.dp)
                .size(56.dp)
                .clip(CircleShape)
                .background(if (warning) c.s3 else c.s2)
                .then(if (warning) Modifier.border(BorderStroke(1.dp, c.warn), CircleShape) else Modifier),
            contentAlignment = Alignment.Center,
        ) {
            SlouchcatIconImage(icon, contentDescription = null, size = 28.dp, tint = if (warning) c.warn else c.tx2)
        }
    }
}

/** The title of a sheet (`.sheet h2`); [centered] under a [SheetHeadIcon]. */
@Composable
fun SheetTitle(text: String, centered: Boolean = false) {
    Text(
        text,
        style = SlouchcatType.sheetTitle,
        color = SlouchcatTheme.colors.tx,
        textAlign = if (centered) TextAlign.Center else TextAlign.Start,
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = gutter(), end = gutter(), top = 4.dp, bottom = 10.dp)
            .semantics { heading() },
    )
}

/** Running text in a sheet (`.su-p`). */
@Composable
fun SheetText(text: String, centered: Boolean = false) {
    Text(
        text,
        style = SlouchcatType.lede,
        color = SlouchcatTheme.colors.tx2,
        textAlign = if (centered) TextAlign.Center else TextAlign.Start,
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = gutter()),
    )
}

/** `.sheet .acts`: the buttons of a sheet side by side, each taking an equal share. */
@Composable
fun SheetActions(content: @Composable RowScope.() -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(start = gutter(), end = gutter(), top = 14.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        content = content,
    )
}

/** `toast(...)`: the host of short messages; show one with `SnackbarHostState.showSnackbar`. */
@Composable
fun SlouchcatSnackbarHost(state: SnackbarHostState, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    SnackbarHost(state, modifier = modifier.widthIn(max = 520.dp)) { data ->
        Snackbar(
            snackbarData = data,
            shape = RoundedCornerShape(10.dp),
            containerColor = c.tx,
            contentColor = c.bg,
            actionColor = c.bg,
        )
    }
}

/** `.wordmark`: the name of the app with its last syllable in the accent («Slouch» + «cat»). */
@Composable
fun Wordmark(name: String, modifier: Modifier = Modifier, wide: Boolean = false) {
    val c = SlouchcatTheme.colors
    val split = if (name.endsWith("cat")) name.length - 3 else name.length
    Text(
        buildAnnotatedString {
            withStyle(SpanStyle(color = c.tx)) { append(name.substring(0, split)) }
            withStyle(SpanStyle(color = c.accent)) { append(name.substring(split)) }
        },
        style = if (wide) SlouchcatType.wordmarkWide else SlouchcatType.wordmark,
        modifier = modifier,
    )
}
