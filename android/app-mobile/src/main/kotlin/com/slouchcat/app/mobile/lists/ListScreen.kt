package com.slouchcat.app.mobile.lists

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.CardTag
import com.slouchcat.app.mobile.media.MediaCard
import com.slouchcat.app.mobile.media.MediaEmpty
import com.slouchcat.app.mobile.media.MediaGrid
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.media.Paged
import com.slouchcat.app.mobile.media.PagingFooter
import com.slouchcat.app.mobile.media.defaultCaption
import com.slouchcat.app.mobile.media.fullWidth
import com.slouchcat.app.mobile.media.remainingText
import com.slouchcat.app.mobile.media.titleCount
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.strings.R

/** The title of a list: the name of its row (`home.rows.*`) or «User-Empfehlungen». */
@Composable
fun listTitle(kind: ListKind?): String = when (kind) {
    ListKind.Picks -> stringResource(R.string.home_picks)
    is ListKind.Row -> stringResource(
        when (kind.key) {
            HomeListKey.CONTINUE -> R.string.home_rows_continue
            HomeListKey.RECENT -> R.string.home_rows_recent
            HomeListKey.RELEASED -> R.string.home_rows_released
            else -> R.string.home_rows_spotlight
        },
    )
    null -> stringResource(R.string.home_list_notFound_title)
}

/**
 * A list behind a row of the start page (mockup `/list/:key`, reference photos 23 and 24): bar
 * with back, a line (the count, or the intro of the picks), the grid in pages. A list the server
 * does not have (the spotlight switched off) answers 404: «Liste nicht gefunden».
 */
@Composable
fun ListScreen(
    kind: ListKind?,
    state: Paged<MediaItem>,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onNearEnd: () -> Unit,
    onItem: ((MediaItem) -> Unit)?,
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    val tip = CardTag(stringResource(R.string.app_browse_tip), SlouchcatIcon.Heart)
    Column(modifier.fillMaxSize().background(c.bg)) {
        TopBar(listTitle(kind), navigationLabel = stringResource(R.string.common_back), onNavigation = onBack)
        MediaGrid(
            items = state.loaded,
            key = { it.id },
            onNearEnd = onNearEnd,
            modifier = Modifier.fillMaxSize(),
            header = {
                if (state.items is Loadable.Ready) {
                    fullWidth("intro") {
                        Text(
                            if (kind == ListKind.Picks) stringResource(R.string.picks_page_intro) else titleCount(state.total),
                            style = SlouchcatType.lede,
                            color = c.tx2,
                            modifier = Modifier.padding(bottom = 4.dp),
                        )
                    }
                } else {
                    fullWidth("state") {
                        val items = state.items
                        when {
                            items is Loadable.Empty -> MediaEmpty(stringResource(R.string.home_list_empty), icon = SlouchcatIcon.List)
                            items is Loadable.Failed && items.error == LoadError.NotFound -> MediaEmpty(
                                stringResource(R.string.home_list_notFound_title),
                                body = stringResource(R.string.home_list_notFound_body),
                                icon = SlouchcatIcon.List,
                            )
                            else -> LoadableContent(items, onRetry = onRetry) {}
                        }
                    }
                }
            },
            footer = { fullWidth("more") { PagingFooter(state.more, onRetry = onNearEnd) } },
        ) { item ->
            val continueRow = kind == ListKind.Row(HomeListKey.CONTINUE)
            MediaCard(
                item,
                onClick = onItem?.let { { it(item) } },
                caption = if (continueRow) remainingText(item) ?: defaultCaption(item) else defaultCaption(item),
                tag = if (kind == ListKind.Picks) tip else null,
                width = null,
            )
        }
    }
}
