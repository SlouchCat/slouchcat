package com.slouchcat.app.tv.search

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.utf16CodePoint
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.MediaCard
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.info
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvCard
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

/** A library the search offers before anything is typed (mockup «Bibliotheken»). */
data class SearchLibrary(val name: String, val icon: ImageVector, val open: () -> Unit)

/** Where the search leads; `null`: the page is not registered. */
class SearchLinks(
    val item: ((String) -> Unit)?,
    val person: ((Long) -> Unit)?,
    val collection: ((String) -> Unit)?,
    val libraries: List<SearchLibrary>,
    /** The voice input of the device (`RecognizerIntent`); `null` where the TV has none. */
    val voice: (() -> Unit)?,
)

/** The letters of the keyboard (mockup `KB_ROWS`): digits, then QWERTZ with the signs of addresses and names. */
private val LETTERS = listOf("1234567890", "qwertzuiop-", "asdfghjkl._", "yxcvbnm@:/&")
private val SIGNS = listOf("1234567890", "!?#$%*+=()~", "[]{}<>'\";,|", "\\^`€§°äöüéè")
private const val START = "k-1-0"
private const val DEVICE_KEYBOARD = "kbd"
private const val RETRY = "retry"

/** The groups of titles, in the mockup's order. */
private val GROUPS = listOf(
    ItemKind.MOVIE to R.string.watched_page_contents_movie,
    ItemKind.SERIES to R.string.watched_page_contents_series,
    ItemKind.EPISODE to R.string.item_episodes_title,
    ItemKind.AUDIOBOOK to R.string.watched_page_contents_audiobook,
    ItemKind.PODCAST to R.string.watched_page_contents_podcast,
)

/**
 * The search (mockup `/search`, `browse.js`): the own keyboard on the left (the TV's field needs no
 * system keyboard, which covers half the screen), the voice input of the device where it has one,
 * and on the right the hits as rows (titles by kind, people, collections) or, with nothing typed,
 * the libraries and the genres. A keyboard plugged into the TV types as well.
 */
@Composable
fun SearchScreen(state: SearchUiState, images: ServerImages, links: SearchLinks, events: SearchEvents) {
    val focus = rememberFocusMemory()
    var signs by rememberSaveable { mutableStateOf(false) }
    // The device's input (code review m-4): every script the own keyboard does not have.
    var systemInput by rememberSaveable { mutableStateOf(false) }
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxSize()
            .padding(start = LocalContentStart.current, top = TvDimens.safeY)
            .onPreviewKeyEvent { event ->
                // A keyboard on the TV: a printable sign types it (the D-pad and OK are no signs),
                // Backspace deletes (code review m-4). Not while the device's input has the field.
                if (systemInput || event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                if (event.key == androidx.compose.ui.input.key.Key.Backspace) {
                    events.onText(state.text.dropLast(1))
                    return@onPreviewKeyEvent true
                }
                val code = event.utf16CodePoint
                if (code < ' '.code || code == DELETE) return@onPreviewKeyEvent false
                events.onText(state.text + code.toChar())
                true
            },
        horizontalArrangement = Arrangement.spacedBy(TvDimens.xxl),
    ) {
        Column(Modifier.width(TvDimens.Browse.SearchKeys)) {
            Text(stringResource(R.string.nav_search), style = TvType.h1, modifier = Modifier.semantics { heading() })
            Row(Modifier.padding(top = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(TvDimens.s), verticalAlignment = Alignment.CenterVertically) {
                if (systemInput) {
                    SystemField(state.text, events::onText, Modifier.weight(1f)) {
                        systemInput = false
                        focus.focus(DEVICE_KEYBOARD)
                    }
                } else {
                    Field(state.text, Modifier.weight(1f))
                }
                Key(
                    stringResource(R.string.tv_search_deviceKeyboard),
                    onClick = { systemInput = true },
                    modifier = Modifier.width(TvDimens.Browse.SearchKey * 2).focusKey(focus, DEVICE_KEYBOARD),
                    icon = com.slouchcat.app.tv.ui.TvIcons.Keyboard,
                )
                links.voice?.let { voice ->
                    Key(stringResource(R.string.tv_search_voice), onClick = voice, modifier = Modifier.width(TvDimens.Browse.SearchKey * 2).focusKey(focus, "mic"), icon = SlouchcatIcon.Mic.vector)
                }
            }
            Keyboard(if (signs) SIGNS else LETTERS, signs, focus, onType = { events.onText(state.text + it) }, onDelete = { events.onText(state.text.dropLast(1)) }, onSigns = { signs = !signs }, onClear = events::onClear)
            Text(stringResource(R.string.tv_search_hint), style = TvType.small, color = c.tx3, modifier = Modifier.padding(top = TvDimens.l))
        }
        CompositionLocalProvider(LocalContentStart provides 0.dp) {
            Results(state, images, links, events, focus, Modifier.weight(1f))
        }
    }
    focus.StartFocus(START)
}

/** What the search can be asked to do; the ViewModel does it, tests record it. */
interface SearchEvents {
    fun onText(text: String)
    fun onClear()
    fun onGenre(genre: String)
    fun onClearGenre()
    fun retry()
    fun onMore()
}

/** `.br-sf`: the field (no control: the keyboard types into it), the caret in the accent. */
@Composable
private fun Field(text: String, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val hint = stringResource(R.string.app_search_placeholder)
    Row(
        modifier
            .height(TvDimens.Field.Height)
            .border(BorderStroke(TvDimens.borderThick, c.tx), RoundedCornerShape(TvDimens.radiusField))
            .padding(horizontal = TvDimens.Field.Pad)
            .semantics(mergeDescendants = true) { contentDescription = text.ifEmpty { hint } },
        horizontalArrangement = Arrangement.spacedBy(TvDimens.m),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TvIcon(SlouchcatIcon.Search, TvDimens.Field.Icon, tint = c.tx2)
        Text(text.ifEmpty { hint }, style = TvType.field, color = if (text.isEmpty()) c.tx3 else c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
        Box(Modifier.width(TvDimens.borderThick * 2).height(TvDimens.Field.Icon).background(c.accent))
    }
}

/**
 * The field with the device's input: it takes the focus and opens the keyboard of the system;
 * every change searches at once; «Fertig» or Back closes it ([onClose]).
 */
@Composable
private fun SystemField(text: String, onText: (String) -> Unit, modifier: Modifier, onClose: () -> Unit) {
    val c = SlouchcatTheme.colors
    val requester = remember { androidx.compose.ui.focus.FocusRequester() }
    val keyboard = androidx.compose.ui.platform.LocalSoftwareKeyboardController.current
    val label = stringResource(R.string.search_label)
    val close = {
        keyboard?.hide()
        onClose()
    }
    androidx.activity.compose.BackHandler(onBack = close)
    androidx.compose.foundation.text.BasicTextField(
        value = text,
        onValueChange = onText,
        singleLine = true,
        textStyle = TvType.field.copy(color = c.tx),
        cursorBrush = androidx.compose.ui.graphics.SolidColor(c.accent),
        keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(imeAction = androidx.compose.ui.text.input.ImeAction.Search, autoCorrectEnabled = false),
        keyboardActions = androidx.compose.foundation.text.KeyboardActions(onSearch = { close() }, onDone = { close() }),
        modifier = modifier
            .height(TvDimens.Field.Height)
            .border(BorderStroke(TvDimens.borderThick, c.accent), RoundedCornerShape(TvDimens.radiusField))
            .padding(horizontal = TvDimens.Field.Pad)
            .focusRequester(requester)
            .semantics { contentDescription = label },
        decorationBox = { inner -> Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.CenterStart) { inner() } },
    )
    androidx.compose.runtime.LaunchedEffect(Unit) {
        androidx.compose.runtime.withFrameNanos { }
        runCatching { requester.requestFocus() }.getOrDefault(Unit)
        keyboard?.show()
    }
}

/** `.br-kb`: eleven columns of keys, the last row with the signs, the space and «Leeren». */
@Composable
private fun Keyboard(rows: List<String>, signs: Boolean, focus: FocusMemory, onType: (String) -> Unit, onDelete: () -> Unit, onSigns: () -> Unit, onClear: () -> Unit) {
    val gap = TvDimens.s
    Column(Modifier.padding(top = TvDimens.l), verticalArrangement = Arrangement.spacedBy(gap)) {
        rows.forEachIndexed { r, keys ->
            Row(horizontalArrangement = Arrangement.spacedBy(gap)) {
                keys.forEachIndexed { k, ch ->
                    Key(ch.toString(), onClick = { onType(ch.toString()) }, modifier = Modifier.weight(1f).focusKey(focus, "k-$r-$k"))
                }
                if (r == 0) Key(stringResource(R.string.tv_search_delete), onClick = onDelete, modifier = Modifier.weight(1f).focusKey(focus, "k-0-10"), icon = com.slouchcat.app.tv.ui.TvIcons.Backspace)
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(gap)) {
            Key(stringResource(if (signs) R.string.tv_search_letters else R.string.tv_search_symbols), onClick = onSigns, modifier = Modifier.weight(2f).focusKey(focus, "k-4-0"), small = true)
            Key(stringResource(R.string.tv_search_space), onClick = { onType(" ") }, modifier = Modifier.weight(6f).focusKey(focus, "k-4-1"), small = true)
            Key(stringResource(R.string.tv_search_clear), onClick = onClear, modifier = Modifier.weight(3f).focusKey(focus, "k-4-2"), small = true, icon = SlouchcatIcon.Close.vector, showText = true)
        }
    }
}

/** `.kb-k`: a key; with an [icon] only its description is said (and shown with [showText]). */
@Composable
private fun Key(text: String, onClick: () -> Unit, modifier: Modifier, icon: ImageVector? = null, small: Boolean = false, showText: Boolean = icon == null) {
    val c = SlouchcatTheme.colors
    Surface(
        onClick = onClick,
        modifier = modifier.height(TvDimens.Browse.SearchKey).semantics { contentDescription = text },
        shape = ClickableSurfaceDefaults.shape(shape = RoundedCornerShape(TvDimens.radiusMenuItem)),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = c.s2,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
        ),
        border = ClickableSurfaceDefaults.border(focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx))),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Row(Modifier.align(Alignment.Center), horizontalArrangement = Arrangement.spacedBy(TvDimens.s), verticalAlignment = Alignment.CenterVertically) {
            if (icon != null) TvIcon(icon, TvDimens.Browse.Icon)
            if (showText) Text(text, style = if (small) TvType.bodyStrong else TvType.large.copy(fontWeight = androidx.compose.ui.text.font.FontWeight.SemiBold), maxLines = 1)
        }
    }
}

@Composable
private fun Results(state: SearchUiState, images: ServerImages, links: SearchLinks, events: SearchEvents, focus: FocusMemory, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    LazyColumn(modifier.fillMaxSize(), contentPadding = PaddingValues(top = TvDimens.Browse.HeadHeight, bottom = TvDimens.Browse.DetailTop)) {
        val genre = state.genre
        if (genre != null) {
            item(key = "genre") {
                Row(Modifier.padding(bottom = TvDimens.m)) {
                    ControlChip(stringResource(R.string.app_search_genreFilter, genre), SlouchcatIcon.Close.vector, onClick = events::onClearGenre, on = true, modifier = Modifier.focusKey(focus, "genre"))
                }
            }
        }
        when (val hits = state.hits) {
            null -> {
                if (state.tooShort) item(key = "short") { Text(stringResource(R.string.search_tooShort), style = TvType.large, color = c.tx2) }
                if (links.libraries.isNotEmpty()) {
                    item(key = "libraries") {
                        TvCardRow(title = stringResource(R.string.tv_search_libraries), items = links.libraries, key = { it.name }, focus = focus, rowKey = "lib") { library, m ->
                            ControlChip(library.name, library.icon, onClick = library.open, modifier = m)
                        }
                    }
                }
                val genres = (state.genres as? Loadable.Ready)?.value.orEmpty()
                if (genres.isNotEmpty()) {
                    item(key = "genres") {
                        TvCardRow(title = stringResource(R.string.app_search_genres), items = genres, key = { it.name }, focus = focus, rowKey = "gen") { g, m ->
                            ControlChip(g.name, SlouchcatIcon.Filter.vector, onClick = { events.onGenre(g.name) }, modifier = m)
                        }
                    }
                }
            }
            Loadable.Empty -> item(key = "none") {
                EmptyState(SlouchcatIcon.Search.vector, stringResource(R.string.search_empty_title), if (state.query.isNotEmpty()) stringResource(R.string.app_search_emptyBody, state.query) else null)
            }
            is Loadable.Ready -> {
                val found = hits.value
                item(key = "count") {
                    val count = pluralCount(R.plurals.app_search_hits, found.count)
                    Text(if (found.query.isNotEmpty()) stringResource(R.string.tv_search_hitsFor, count, found.query) else count, style = TvType.h2, modifier = Modifier.padding(bottom = TvDimens.l))
                }
                for ((kind, label) in GROUPS) {
                    val group = found.items.filter { it.kind == kind }
                    if (group.isEmpty()) continue
                    item(key = "kind-${kind.value}") {
                        val infos = group.associate { it.id to it.info() }
                        TvCardRow(
                            title = "${stringResource(label)} · ${group.size}",
                            items = group,
                            key = { it.id },
                            focus = focus,
                            rowKey = "sr-${kind.value}",
                            info = { infos[it.id] },
                        ) { item, m ->
                            MediaCard(item, links.item?.let { open -> { open(item.id) } }, m.nearEnd(item, found.items, events))
                        }
                    }
                }
                if (found.people.isNotEmpty()) {
                    item(key = "people") {
                        TvCardRow(
                            title = "${stringResource(R.string.search_sections_people)} · ${found.people.size}",
                            items = found.people,
                            key = { it.id },
                            focus = focus,
                            rowKey = "sr-people",
                            info = { CardInfo(it.name) },
                        ) { person, m ->
                            TvCard(person.name, images.single(person.image).forWidth(TvDimens.Browse.CastWidth), onClick = { links.person?.invoke(person.id) }, modifier = m, caption = person.name, width = TvDimens.Browse.CastWidth)
                        }
                    }
                }
                if (found.collections.isNotEmpty()) {
                    item(key = "collections") {
                        TvCardRow(
                            title = "${stringResource(R.string.search_sections_collections)} · ${found.collections.size}",
                            items = found.collections,
                            key = { it.id },
                            focus = focus,
                            rowKey = "sr-col",
                            info = { CardInfo(it.name) },
                        ) { col, m ->
                            val cover = col.cover?.let { MediaItem.from(it, images) }
                            TvCard(col.name, images.single(col.image).or(cover?.poster ?: images.single(null)).forWidth(com.slouchcat.app.tv.ui.CardShape.Poster.width), onClick = { links.collection?.invoke(col.id) }, modifier = m)
                        }
                    }
                }
            }
            else -> item(key = "failed") { LoadableContent(hits, onRetry = events::retry, retryModifier = Modifier.focusKey(focus, RETRY)) {} }
        }
    }
}

/** The titles come in pages: near the last loaded card the next page is asked for. */
private fun Modifier.nearEnd(item: MediaItem, all: List<MediaItem>, events: SearchEvents): Modifier =
    onFocusChanged { if (it.isFocused && all.indexOf(item) >= all.size - NEAR_END) events.onMore() }

private const val NEAR_END = 6
private const val DELETE = 0x7f
