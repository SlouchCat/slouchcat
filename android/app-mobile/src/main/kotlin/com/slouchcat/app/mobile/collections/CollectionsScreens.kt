package com.slouchcat.app.mobile.collections

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.MediaCard
import com.slouchcat.app.mobile.media.MediaEmpty
import com.slouchcat.app.mobile.media.MediaGrid
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.fullWidth
import com.slouchcat.core.api.generated.models.Collection
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.browse.collections.CollectionPage
import com.slouchcat.core.browse.collections.CollectionUiState
import com.slouchcat.core.browse.collections.CollectionViewModel
import com.slouchcat.core.browse.collections.CollectionWatched
import com.slouchcat.core.browse.collections.CollectionsUiState
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

/** The poster of a collection: its own image, else the poster of its cover title. */
fun collectionPoster(images: ServerImages, image: com.slouchcat.core.api.generated.models.ImageRef?, cover: ItemRef?): ImageSet =
    images.single(image).or(images.posters(cover?.images))

/**
 * «Sammlungen» (mockup `/collections`, reference photo handy 27): the count, then every collection
 * as a poster card with its name and «4 Titel».
 */
@Composable
fun CollectionsScreen(
    state: CollectionsUiState,
    images: ServerImages,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onOpen: (String) -> (() -> Unit)?,
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    Column(modifier.fillMaxSize().background(c.bg)) {
        TopBar(stringResource(R.string.collection_all_title), navigationLabel = stringResource(R.string.common_back), onNavigation = onBack)
        LoadableContent(
            state.collections,
            onRetry = onRetry,
            empty = { MediaEmpty(stringResource(R.string.collection_all_empty), icon = SlouchcatIcon.Grid, modifier = Modifier.padding(horizontal = gutter())) },
        ) { list ->
            val cards = remember(list, images) {
                list.map { MediaItem(id = it.id, kind = ItemKind.MOVIE, title = it.name, poster = collectionPoster(images, it.image, it.cover)) to it }
            }
            MediaGrid(
                cards,
                key = { it.first.id },
                header = {
                    fullWidth("count") {
                        Text(
                            pluralStringResource(R.plurals.collection_all_count, list.size, list.size),
                            style = SlouchcatType.caption.copy(fontSize = 14.sp),
                            color = c.tx2,
                        )
                    }
                },
            ) { (card, ref: CollectionRef) ->
                MediaCard(card, onClick = onOpen(ref.id), caption = pluralStringResource(R.plurals.collection_count, ref.count, ref.count), width = null)
            }
        }
    }
}

/**
 * A collection (mockup `/collection/:id`, reference photo handy 28): the blurred poster behind the
 * head, the poster, «SAMMLUNG», name, «4 Titel», the buttons «Abspielen» (once the player exists),
 * «Alle merken» and «Gesehen», then «In dieser Sammlung» as a grid. From 700 dp the poster stands
 * beside the text.
 */
@Composable
fun CollectionScreen(
    state: CollectionUiState,
    images: ServerImages,
    snackbar: SnackbarHostState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onListAll: () -> Unit,
    onWatched: () -> Unit,
    onConfirm: (Boolean) -> Unit,
    onDismiss: () -> Unit,
    onOpen: (String) -> (() -> Unit)?,
    onPlay: (String) -> (() -> Unit)?,
    modifier: Modifier = Modifier,
    /** Stage 7: an audiobook or podcast episode plays in the audio player (`null`: [onPlay] for all). */
    onListen: ((String) -> (() -> Unit)?)? = null,
) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        when (val loaded = state.page) {
            is Loadable.Ready -> {
                val page = loaded.value
                MediaGrid(
                    page.items,
                    key = { it.id },
                    header = {
                        fullWidth("head") { Head(page, state, images, onListAll, onWatched, onOpen, onPlay, onListen) }
                        fullWidth("members") {
                            Text(
                                stringResource(R.string.app_collections_members),
                                style = SlouchcatType.strong.copy(fontSize = 19.sp, lineHeight = 24.sp),
                                color = c.tx,
                                modifier = Modifier.padding(top = 8.dp).semantics { heading() },
                            )
                        }
                    },
                ) { item -> MediaCard(item, onClick = onOpen(item.id), width = null) }
            }
            is Loadable.Failed -> Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(start = gutter(), end = gutter(), top = 88.dp)) {
                if (loaded.error == LoadError.NotFound) {
                    MediaEmpty(stringResource(R.string.collection_notFound_title), body = stringResource(R.string.collection_notFound_body), icon = SlouchcatIcon.Grid)
                } else {
                    LoadableContent(loaded, onRetry = onRetry) {}
                }
            }
            else -> LoadableContent(loaded, onRetry = onRetry, modifier = Modifier.padding(top = 72.dp)) {}
        }
        Box(Modifier.padding(horizontal = 8.dp, vertical = 6.dp).clip(CircleShape).background(c.scrim)) {
            SlouchcatIconButton(SlouchcatIcon.Back, contentDescription = stringResource(R.string.common_back), onClick = onBack)
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
    state.asking?.let { asking ->
        val name = (state.page as? Loadable.Ready)?.value?.collection?.name.orEmpty()
        val page = (state.page as? Loadable.Ready)?.value
        val title = stringResource(if (asking == CollectionWatched.Full) R.string.watched_ask_removeTitle else R.string.watched_ask_halfTitle, name)
        SlouchcatSheet(title, onDismiss = onDismiss) {
            SheetHeadIcon(SlouchcatIcon.Eye, warning = true)
            SheetTitle(title, centered = true)
            val total = page?.collection?.total ?: 0
            val seen = page?.collection?.seenCount ?: 0
            SheetText(
                if (asking == CollectionWatched.Full) {
                    pluralStringResource(R.plurals.collection_ask_removeBody, total, total)
                } else {
                    pluralStringResource(R.plurals.collection_ask_halfBody, seen, seen, total)
                },
                centered = true,
            )
            if (asking == CollectionWatched.Half) {
                Box(Modifier.padding(start = gutter(), end = gutter(), top = 14.dp)) {
                    SlouchcatButton(stringResource(R.string.collection_ask_markAll), onClick = { onConfirm(true) }, kind = ButtonKind.Primary, block = true)
                }
            }
            SheetActions {
                SlouchcatButton(stringResource(R.string.common_cancel), onClick = onDismiss, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
                SlouchcatButton(stringResource(R.string.watched_ask_removeAll), onClick = { onConfirm(false) }, kind = ButtonKind.Danger, modifier = Modifier.weight(1f))
            }
        }
    }
}

/** The label and the target of the big button (web `playTarget`); `null` without a next part. */
@Composable
private fun playLabel(collection: Collection): String? {
    val next = collection.next ?: return null
    val part = collection.nextPart ?: return null
    val group = CollectionViewModel.playsFromOwnPage(next)
    val again = if (group) next.watched == true else next.progress?.completed == true
    val running = !again && if (group) (next.watchedCount ?: 0) > 0 else (next.progress?.positionMs ?: 0) > 0
    return stringResource(
        when {
            again -> R.string.collection_rewatch
            running -> R.string.collection_resume
            else -> R.string.collection_play
        },
        part.toString(),
    )
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Head(
    page: CollectionPage,
    state: CollectionUiState,
    images: ServerImages,
    onListAll: () -> Unit,
    onWatched: () -> Unit,
    onOpen: (String) -> (() -> Unit)?,
    onPlay: (String) -> (() -> Unit)?,
    onListen: ((String) -> (() -> Unit)?)?,
) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    val collection = page.collection
    val poster = remember(collection.image, page.parts, images) { collectionPoster(images, collection.image, page.parts.firstOrNull()) }
    val next = collection.next
    val play = next?.let {
        when {
            CollectionViewModel.playsFromOwnPage(it) -> onOpen(it.id)
            onListen != null && com.slouchcat.core.media.isAudio(it.kind) -> onListen(it.id)
            else -> onPlay(it.id)
        }
    }
    val label = playLabel(collection)
    val text: @Composable (Alignment.Horizontal, TextAlign) -> Unit = { align, textAlign ->
        Column(horizontalAlignment = align, verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(stringResource(R.string.app_collections_overline).uppercase(), style = SlouchcatType.section, color = c.accent)
            Text(
                collection.name,
                style = SlouchcatType.pageTitle.copy(fontSize = if (wide) 38.sp else 28.sp, lineHeight = if (wide) 44.sp else 33.sp, fontWeight = FontWeight.ExtraBold),
                color = c.tx,
                textAlign = textAlign,
                modifier = Modifier.semantics { heading() },
            )
            Text(pluralStringResource(R.plurals.collection_count, collection.total, collection.total), style = SlouchcatType.bodySmall, color = c.tx2)
            if (collection.started == true) {
                val seen = collection.seenCount ?: 0
                Text(pluralStringResource(R.plurals.collection_seen, seen, seen, collection.total), style = SlouchcatType.caption, color = c.tx2)
            }
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(10.dp, if (wide) Alignment.Start else Alignment.CenterHorizontally),
                verticalArrangement = Arrangement.spacedBy(10.dp),
                modifier = Modifier.padding(top = 10.dp),
            ) {
                if (play != null && label != null) SlouchcatButton(label, onClick = play, kind = ButtonKind.Primary, leadingIcon = SlouchcatIcon.Play)
                SlouchcatButton(
                    stringResource(if (page.allListed) R.string.collection_watchlist_added else R.string.collection_watchlist_add),
                    onClick = onListAll,
                    kind = ButtonKind.Secondary,
                    busy = state.listing,
                    enabled = page.parts.isNotEmpty(),
                    leadingIcon = if (page.allListed) SlouchcatIcon.BookmarkOn else SlouchcatIcon.Bookmark,
                )
                SlouchcatButton(
                    stringResource(
                        when (page.watched) {
                            CollectionWatched.Full -> R.string.watched_state_full
                            CollectionWatched.Half -> R.string.watched_state_half
                            CollectionWatched.None -> R.string.watched_hint_set
                        },
                    ),
                    onClick = onWatched,
                    kind = ButtonKind.Secondary,
                    busy = state.marking,
                    leadingIcon = if (page.watched == CollectionWatched.None) SlouchcatIcon.Eye else SlouchcatIcon.Check,
                )
            }
        }
    }
    Box(Modifier.fillMaxWidth()) {
        // The blurred poster behind the head (`.colp-bg`), blurred on the CPU (no RenderEffect, stage 7).
        Box(Modifier.matchParentSize().clip(RoundedCornerShape(0.dp))) {
            com.slouchcat.app.mobile.media.BlurredImage(poster, Modifier.matchParentSize())
            Box(Modifier.matchParentSize().background(Brush.verticalGradient(listOf(c.scrim, c.bg))))
        }
        if (wide) {
            Row(
                Modifier.fillMaxWidth().padding(top = 72.dp, bottom = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(32.dp),
                verticalAlignment = Alignment.Bottom,
            ) {
                Poster(poster, collection.name, Modifier.width(200.dp))
                Box(Modifier.weight(1f).padding(bottom = 4.dp)) { text(Alignment.Start, TextAlign.Start) }
            }
        } else {
            Column(
                Modifier.fillMaxWidth().padding(top = 64.dp, bottom = 12.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(18.dp),
            ) {
                Poster(poster, collection.name, Modifier.width(164.dp))
                text(Alignment.CenterHorizontally, TextAlign.Center)
            }
        }
    }
}

@Composable
private fun Poster(images: ImageSet, name: String, modifier: Modifier = Modifier) {
    val shape = RoundedCornerShape(14.dp)
    Box(
        modifier
            .aspectRatio(2f / 3f)
            .clip(shape)
            .border(BorderStroke(1.dp, SlouchcatTheme.colors.frame), shape),
    ) { MediaImage(images, name, Modifier.fillMaxSize()) }
}
