package com.slouchcat.app.mobile.media

import android.content.res.Resources
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.collapse
import androidx.compose.ui.semantics.expand
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.Flow

/*
 * What the detail pages (title, person) share (code review m-10): the states before the page is
 * there, the round buttons over the picture, the text that opens with «Mehr anzeigen», and the
 * snackbar of a page's messages.
 */

/**
 * A detail page that is not there yet: the spinner, «not found» as an empty state ([notFoundTitle],
 * [notFoundBody], [icon]), any other error with «Erneut versuchen».
 */
@Composable
fun DetailState(state: Loadable<*>, notFoundTitle: String, notFoundBody: String, icon: SlouchcatIcon, onRetry: () -> Unit) {
    when (state) {
        is Loadable.Failed -> Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(start = gutter(), end = gutter(), top = 88.dp)) {
            if (state.error == LoadError.NotFound) {
                EmptyState(notFoundTitle, body = notFoundBody, icon = icon)
            } else {
                LoadableContent(state, onRetry = onRetry) {}
            }
        }
        is Loadable.Ready -> Unit
        else -> LoadableContent(state, onRetry = onRetry, modifier = Modifier.padding(top = 72.dp)) {}
    }
}

/** A round button over the picture of a detail page (mockup `topBack`): back, the search. */
@Composable
fun RoundIcon(icon: SlouchcatIcon, label: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    Box(modifier.padding(2.dp).clip(CircleShape).background(SlouchcatTheme.colors.scrim)) {
        SlouchcatIconButton(icon, contentDescription = label, onClick = onClick)
    }
}

/**
 * A long text in [lines] lines with [more]/[less] below it when it is longer (`.dt-ov`, web
 * `Biography`). The screen reader hears «zugeklappt»/«aufgeklappt» and may expand or collapse it
 * (code review L-2).
 */
@Composable
fun ClampedText(
    text: String,
    open: Boolean,
    onToggle: () -> Unit,
    more: String,
    less: String,
    style: TextStyle,
    lines: Int,
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    var clamped by remember(text) { mutableStateOf(false) }
    Column(modifier) {
        Text(
            text,
            style = style,
            color = c.tx2,
            maxLines = if (open) Int.MAX_VALUE else lines,
            overflow = TextOverflow.Ellipsis,
            onTextLayout = { if (!open) clamped = it.hasVisualOverflow },
        )
        if (clamped || open) {
            val state = stringResource(if (open) R.string.app_state_expanded else R.string.app_state_collapsed)
            Box(
                Modifier
                    .clip(RoundedCornerShape(8.dp))
                    .clickable(role = Role.Button, onClick = onToggle)
                    .semantics {
                        stateDescription = state
                        if (open) collapse { onToggle(); true } else expand { onToggle(); true }
                    }
                    .heightIn(min = 48.dp)
                    .padding(end = 8.dp),
                contentAlignment = Alignment.CenterStart,
            ) {
                Text(
                    if (open) less else more,
                    style = SlouchcatType.bodySmall.copy(fontSize = 14.5.sp, textDecoration = TextDecoration.Underline),
                    color = c.tx,
                )
            }
        }
    }
}

/** Shows each message of a page's [messages] in [snackbar], its text from [text]. */
@Composable
fun <M> ShowMessages(messages: Flow<M>, snackbar: SnackbarHostState, text: Resources.(M) -> String) {
    val resources = LocalResources.current
    LaunchedEffect(messages) {
        messages.collect { message -> snackbar.showSnackbar(resources.text(message)) }
    }
}
