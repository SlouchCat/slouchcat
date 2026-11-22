package com.slouchcat.app.mobile.media

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter

/**
 * The head of a row (`.rowh`): its title as a heading and, when [onShowAll] leads somewhere,
 * «Alle ›» at the end (said as «Alle anzeigen: Weiterschauen»).
 */
@Composable
fun RowHeader(title: String, onShowAll: (() -> Unit)?, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Row(
        modifier
            .fillMaxWidth()
            .padding(start = gutter(), end = gutter() - 8.dp, top = 12.dp, bottom = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            title,
            style = SlouchcatType.strong.copy(fontSize = 19.sp, lineHeight = 24.sp),
            color = c.tx,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f).padding(vertical = 8.dp).semantics { heading() },
        )
        if (onShowAll != null) {
            val label = stringResource(R.string.app_browse_showAllLabel, title)
            Row(
                Modifier
                    .clip(RoundedCornerShape(24.dp))
                    .clickable(role = Role.Button, onClick = onShowAll)
                    .semantics(mergeDescendants = true) { contentDescription = label }
                    .heightIn(min = 48.dp)
                    .padding(horizontal = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(stringResource(R.string.app_browse_showAll), style = SlouchcatType.buttonSmall, color = c.tx2)
                SlouchcatIconImage(SlouchcatIcon.Right, contentDescription = null, size = 16.dp, tint = c.tx2)
            }
        }
    }
}

/**
 * A row of a page (mockup `hrow(titel, ids, opts, alleLink)`): [RowHeader] and the cards side by
 * side, scrolled sideways with the page's side margin. [content] adds the cards
 * (`items(list, key = { it.id }) { MediaCard(it, …) }`).
 */
@Composable
fun MediaRow(
    title: String,
    onShowAll: (() -> Unit)?,
    modifier: Modifier = Modifier,
    content: LazyListScope.() -> Unit,
) {
    Column(modifier.fillMaxWidth()) {
        RowHeader(title, onShowAll)
        LazyRow(
            contentPadding = PaddingValues(horizontal = gutter()),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.fillMaxWidth().padding(bottom = 4.dp),
            content = content,
        )
    }
}

/** The smallest column of a grid (`.grid`: 104 px on the phone, 140 px from 700 dp). */
@Composable
fun gridMinCell(): Dp = if (isWideLayout()) 140.dp else 104.dp

/**
 * A grid of cards (`.grid`, `auto-fill, minmax(104px, 1fr)`): as many columns as fit the width.
 * [header] spans the whole width above the cards (chips, the count line), [footer] below them.
 * [onNearEnd] runs when the last cards come into view: the page loads its next page there.
 * Every cell holds a [MediaCard] with `width = null`.
 */
@Composable
fun <T> MediaGrid(
    items: List<T>,
    key: (T) -> Any,
    modifier: Modifier = Modifier,
    state: LazyGridState = rememberLazyGridState(),
    onNearEnd: (() -> Unit)? = null,
    /** One column without gaps: the entries are list rows ([MediaListRow]), not cards. */
    list: Boolean = false,
    header: LazyGridScope.() -> Unit = {},
    footer: LazyGridScope.() -> Unit = {},
    card: @Composable (T) -> Unit,
) {
    val wide = isWideLayout()
    val side = gutter()
    if (onNearEnd != null) {
        val nearEnd by remember(state) {
            derivedStateOf {
                val info = state.layoutInfo
                val last = info.visibleItemsInfo.lastOrNull()?.index ?: -1
                info.totalItemsCount > 0 && last >= info.totalItemsCount - NEAR_END
            }
        }
        LaunchedEffect(state, items.size) {
            snapshotFlow { nearEnd }.distinctUntilChanged().filter { it }.collect { onNearEnd() }
        }
    }
    LazyVerticalGrid(
        columns = if (list) GridCells.Fixed(1) else GridCells.Adaptive(gridMinCell()),
        state = state,
        modifier = modifier,
        contentPadding = PaddingValues(start = side, end = side, bottom = 24.dp),
        horizontalArrangement = Arrangement.spacedBy(if (wide) 16.dp else 12.dp),
        verticalArrangement = Arrangement.spacedBy(if (list) 0.dp else if (wide) 22.dp else 16.dp),
    ) {
        header()
        items(items, key = key) { card(it) }
        footer()
    }
}

/** Cards before the end at which a grid asks for the next page. */
private const val NEAR_END = 6

/** One cell over the whole width of a grid, for its head and foot. */
fun LazyGridScope.fullWidth(key: Any? = null, content: @Composable () -> Unit) {
    item(key = key, span = { GridItemSpan(maxLineSpan) }) { content() }
}

/**
 * The foot of a paged list: a spinner while the next page loads, the error with «Erneut
 * versuchen» when it failed ([onRetry]), nothing otherwise.
 */
@Composable
fun PagingFooter(more: MoreState, onRetry: () -> Unit, modifier: Modifier = Modifier) {
    when (more) {
        MoreState.Idle -> Unit
        MoreState.Loading -> {
            val label = stringResource(R.string.app_state_loading)
            Box(modifier.fillMaxWidth().padding(vertical = 16.dp), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = SlouchcatTheme.colors.tx2, modifier = Modifier.semantics { contentDescription = label })
            }
        }
        MoreState.Failed -> Column(modifier.fillMaxWidth().padding(vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Note(stringResource(R.string.library_page_pageFailed), warning = true, modifier = Modifier.padding(horizontal = 0.dp))
            SlouchcatButton(stringResource(R.string.app_state_retry), onClick = onRetry, kind = ButtonKind.Secondary, block = true)
        }
    }
}

/**
 * `.empty` with an action (mockup `emptyBox(symbol, titel, text, ziel, knopf)`): [title], [body]
 * and, when [action] is set, a button below ([actionLabel]).
 */
@Composable
fun MediaEmpty(
    title: String,
    modifier: Modifier = Modifier,
    body: String? = null,
    icon: SlouchcatIcon = SlouchcatIcon.Info,
    actionLabel: String? = null,
    action: (() -> Unit)? = null,
) {
    Column(modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
        EmptyState(title, body = body, icon = icon)
        if (action != null && actionLabel != null) {
            SlouchcatButton(actionLabel, onClick = action, kind = ButtonKind.Secondary, modifier = Modifier.padding(bottom = 24.dp))
        }
    }
}
