package com.slouchcat.app.mobile.search

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
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
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.MediaEmpty
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.PagingFooter
import com.slouchcat.app.mobile.media.defaultCaption
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.app.mobile.media.titleCount
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.PersonRef
import com.slouchcat.core.api.generated.models.RootContent
import com.slouchcat.core.design.SlouchcatChip
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter

/**
 * The search (mockup `/search`, reference photos handy 22 and 67): the field at the top with
 * «Suche leeren»; without a text the libraries («Stöbern») and the genres as tiles; with a text the
 * count, then the matches grouped as the server sends them (titles, people, collections), each as
 * a row with the match marked. Titles outside the library (stage 8) take the slot [missing] after
 * the groups.
 */
@Composable
fun SearchScreen(
    state: SearchUiState,
    images: ServerImages,
    libraries: List<LibraryNav>,
    onText: (String) -> Unit,
    onClear: () -> Unit,
    onSubmit: () -> Unit,
    onGenre: (String) -> Unit,
    onClearGenre: () -> Unit,
    onRetry: () -> Unit,
    onRetryGenres: () -> Unit,
    onMore: () -> Unit,
    onItem: (String) -> (() -> Unit)?,
    onPerson: (Long) -> (() -> Unit)?,
    onCollection: (String) -> (() -> Unit)?,
    onLibrary: (Long) -> (() -> Unit)?,
    modifier: Modifier = Modifier,
    missing: @Composable (query: String) -> Unit = {},
) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    val list = rememberLazyListState()
    val hits = (state.hits as? Loadable.Ready)?.value
    if (hits != null && hits.hasMore) {
        val nearEnd by remember(list) {
            derivedStateOf {
                val info = list.layoutInfo
                val last = info.visibleItemsInfo.lastOrNull()?.index ?: -1
                info.totalItemsCount > 0 && last >= info.totalItemsCount - NEAR_END
            }
        }
        LaunchedEffect(list, hits.items.size) {
            snapshotFlow { nearEnd }.distinctUntilChanged().filter { it }.collect { onMore() }
        }
    }
    Column(modifier.fillMaxSize().background(c.bg).imePadding()) {
        SearchField(
            state.text,
            onText = onText,
            onClear = onClear,
            onSubmit = onSubmit,
            modifier = Modifier
                .padding(start = gutter(), end = gutter(), top = 12.dp, bottom = 8.dp)
                .then(if (wide) Modifier.widthIn(max = 720.dp) else Modifier.fillMaxWidth()),
        )
        LazyColumn(state = list, modifier = Modifier.fillMaxSize()) {
            state.genre?.let { genre ->
                item("genre") {
                    val remove = stringResource(R.string.app_search_genreRemove, genre)
                    Box(Modifier.padding(horizontal = gutter(), vertical = 4.dp)) {
                        SlouchcatChip(
                            stringResource(R.string.app_search_genreFilter, genre),
                            selected = true,
                            onClick = onClearGenre,
                            icon = SlouchcatIcon.Close,
                            modifier = Modifier.semantics { contentDescription = remove },
                        )
                    }
                }
            }
            when (val found = state.hits) {
                null -> if (state.tooShort) {
                    item("short") { Status(stringResource(R.string.search_tooShort)) }
                } else {
                    entry(state, libraries, wide, onGenre, onRetryGenres, onLibrary)
                }
                Loadable.Empty -> item("empty") {
                    MediaEmpty(
                        stringResource(R.string.search_empty_title),
                        body = if (state.query.isNotEmpty()) {
                            stringResource(R.string.app_search_emptyBody, state.query)
                        } else {
                            stringResource(R.string.app_search_emptyGenre, state.genre.orEmpty())
                        },
                        icon = SlouchcatIcon.Search,
                        modifier = Modifier.padding(horizontal = gutter()),
                    )
                }
                is Loadable.Ready -> results(found.value, state.query, images, onItem, onPerson, onCollection, onMore)
                else -> item("state") { LoadableContent(found, onRetry = onRetry, modifier = Modifier.padding(horizontal = gutter())) {} }
            }
            if (state.hits != null && state.query.length >= 2) item("missing") { missing(state.query) }
            item("end") { Spacer(Modifier.heightIn(min = 24.dp)) }
        }
    }
}

private const val NEAR_END = 6

/** `.sr-box`: the round field with the magnifier and, with a text, «Suche leeren». */
@Composable
private fun SearchField(text: String, onText: (String) -> Unit, onClear: () -> Unit, onSubmit: () -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val focus = LocalFocusManager.current
    val label = stringResource(R.string.search_label)
    val shape = RoundedCornerShape(28.dp)
    Row(
        modifier
            .heightIn(min = 56.dp)
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.ctl), shape)
            .padding(start = 20.dp, end = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        SlouchcatIconImage(SlouchcatIcon.Search, contentDescription = null, size = 22.dp, tint = c.tx2)
        Box(Modifier.weight(1f).padding(start = 12.dp, end = 4.dp), contentAlignment = Alignment.CenterStart) {
            if (text.isEmpty()) Text(stringResource(R.string.app_search_placeholder), style = SlouchcatType.body.copy(fontSize = 17.sp), color = c.tx3, maxLines = 1)
            BasicTextField(
                value = text,
                onValueChange = onText,
                singleLine = true,
                textStyle = SlouchcatType.body.copy(fontSize = 17.sp, color = c.tx),
                cursorBrush = SolidColor(c.tx),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                keyboardActions = KeyboardActions(onSearch = {
                    onSubmit()
                    focus.clearFocus()
                }),
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(min = 48.dp)
                    .semantics { contentDescription = label },
                decorationBox = { inner -> Box(Modifier.heightIn(min = 48.dp), contentAlignment = Alignment.CenterStart) { inner() } },
            )
        }
        if (text.isNotEmpty()) {
            SlouchcatIconButton(SlouchcatIcon.Close, contentDescription = stringResource(R.string.app_search_clear), onClick = onClear, tint = c.tx2)
        } else {
            Spacer(Modifier.width(12.dp))
        }
    }
}

@Composable
private fun Status(text: String) {
    Text(
        text,
        style = SlouchcatType.caption.copy(fontSize = 14.sp),
        color = SlouchcatTheme.colors.tx2,
        modifier = Modifier
            .padding(horizontal = gutter(), vertical = 6.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
    )
}

/** `.rowh`: a heading of the page, with a number at the end where the group has one. */
@Composable
private fun GroupHead(title: String, count: Int? = null) {
    val c = SlouchcatTheme.colors
    Row(Modifier.fillMaxWidth().padding(start = gutter(), end = gutter(), top = 14.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(title, style = SlouchcatType.strong.copy(fontSize = 19.sp, lineHeight = 24.sp), color = c.tx, modifier = Modifier.weight(1f).semantics { heading() })
        if (count != null) Text(count.toString(), style = SlouchcatType.label, color = c.tx2)
    }
}

/** Without a text: «Stöbern» (the libraries of the profile) and «Genres» (`listGenres`). */
private fun LazyListScope.entry(
    state: SearchUiState,
    libraries: List<LibraryNav>,
    wide: Boolean,
    onGenre: (String) -> Unit,
    onRetryGenres: () -> Unit,
    onLibrary: (Long) -> (() -> Unit)?,
) {
    val columns = if (wide) 4 else 2
    if (libraries.isNotEmpty()) {
        item("browse") { GroupHead(stringResource(R.string.app_search_browse)) }
        libraries.chunked(columns).forEachIndexed { index, row ->
            item("lib-$index") {
                TileRow(columns) {
                    row.forEach { library ->
                        Tile(library.name, titleCount(library.itemCount), libraryIcon(library), onLibrary(library.id), Modifier.weight(1f))
                    }
                    repeat(columns - row.size) { Spacer(Modifier.weight(1f)) }
                }
            }
        }
    }
    item("genres") { GroupHead(stringResource(R.string.app_search_genres)) }
    when (val genres = state.genres) {
        is Loadable.Ready -> genres.value.chunked(columns).forEachIndexed { index, row ->
            item("genre-$index") {
                TileRow(columns) {
                    row.forEach { genre: GenreRef -> Tile(genre.name, titleCount(genre.count), null, { onGenre(genre.name) }, Modifier.weight(1f)) }
                    repeat(columns - row.size) { Spacer(Modifier.weight(1f)) }
                }
            }
        }
        Loadable.Empty -> Unit
        else -> item("genres-state") { LoadableContent(genres, onRetry = onRetryGenres, modifier = Modifier.padding(horizontal = gutter())) {} }
    }
}

@Composable
private fun TileRow(columns: Int, content: @Composable androidx.compose.foundation.layout.RowScope.() -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = gutter(), vertical = 5.dp),
        horizontalArrangement = Arrangement.spacedBy(if (columns > 2) 12.dp else 10.dp),
        content = content,
    )
}

/** `.tile`: a library or a genre, its name and «505 Titel»; not a control while its page is missing. */
@Composable
private fun Tile(name: String, caption: String, icon: SlouchcatIcon?, onClick: (() -> Unit)?, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(14.dp)
    Column(
        modifier
            .heightIn(min = 92.dp)
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape)
            .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClick = onClick) else Modifier)
            .padding(horizontal = 14.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.Bottom,
    ) {
        if (icon != null) SlouchcatIconImage(icon, contentDescription = null, size = 20.dp, tint = c.accent)
        Spacer(Modifier.weight(1f, fill = false).heightIn(min = 8.dp))
        Text(name, style = SlouchcatType.strong, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
        Text(caption, style = SlouchcatType.caption, color = c.tx2, maxLines = 1)
    }
}

private fun libraryIcon(library: LibraryNav): SlouchcatIcon = when (library.kind) {
    LibraryKind.AUDIOBOOKS -> SlouchcatIcon.Headphones
    LibraryKind.PODCASTS -> SlouchcatIcon.Mic
    else -> when {
        library.contents == listOf(RootContent.SERIES) -> SlouchcatIcon.Tv
        library.contents.contains(RootContent.MOVIE) -> SlouchcatIcon.Film
        else -> SlouchcatIcon.Globe
    }
}

/** The matches: the count, then titles, people and collections, each a group with its number. */
private fun LazyListScope.results(
    hits: SearchHits,
    query: String,
    images: ServerImages,
    onItem: (String) -> (() -> Unit)?,
    onPerson: (Long) -> (() -> Unit)?,
    onCollection: (String) -> (() -> Unit)?,
    onMore: () -> Unit,
) {
    item("count") { Status(pluralStringResource(R.plurals.app_search_hits, hits.count, hits.count)) }
    if (hits.items.isNotEmpty()) {
        item("titles") { GroupHead(stringResource(R.string.search_sections_titles), hits.total) }
        items(hits.items, key = { "t-" + it.id }) { item ->
            ResultRow(highlight(item.title, query), titleCaption(item), item.poster, item.title, onItem(item.id), square = item.isAudio)
        }
        if (hits.more != MoreState.Idle) item("more") { Box(Modifier.padding(horizontal = gutter())) { PagingFooter(hits.more, onMore) } }
    }
    if (hits.people.isNotEmpty()) {
        item("people") { GroupHead(stringResource(R.string.search_sections_people), hits.people.size) }
        items(hits.people, key = { "p-" + it.id }) { person: PersonRef ->
            ResultRow(highlight(person.name, query), null, images.single(person.image), person.name, onPerson(person.id), round = true)
        }
    }
    if (hits.collections.isNotEmpty()) {
        item("collections") { GroupHead(stringResource(R.string.search_sections_collections), hits.collections.size) }
        items(hits.collections, key = { "c-" + it.id }) { collection: CollectionRef ->
            val cover = images.single(collection.image).or(images.posters(collection.cover?.images))
            ResultRow(highlight(collection.name, query), pluralStringResource(R.plurals.collection_count, collection.count, collection.count), cover, collection.name, onCollection(collection.id))
        }
    }
}

/** «2026 · Film · 2 Std. 25 Min.» */
@Composable
private fun titleCaption(item: MediaItem): String? {
    val parts = listOfNotNull(defaultCaption(item), item.runtimeMinutes?.takeIf { it > 0 && !item.isAudio }?.let { runtimeText(it) })
    return parts.takeIf { it.isNotEmpty() }?.joinToString(stringResource(R.string.format_separator))
}

/** The first place where [query] stands in [text], in the accent colour (`hl()` of the mockup). */
@Composable
private fun highlight(text: String, query: String): AnnotatedString {
    val accent = SlouchcatTheme.colors.accent
    val at = if (query.isBlank()) -1 else text.indexOf(query, ignoreCase = true)
    return buildAnnotatedString {
        if (at < 0) {
            append(text)
        } else {
            append(text.substring(0, at))
            withStyle(SpanStyle(color = accent)) { append(text.substring(at, at + query.length)) }
            append(text.substring(at + query.length))
        }
    }
}

/** `.lr`: cover, title with the match marked, caption and the arrow; one control for a screen reader. */
@Composable
private fun ResultRow(
    title: AnnotatedString,
    caption: String?,
    image: com.slouchcat.core.media.ImageSet,
    name: String,
    onClick: (() -> Unit)?,
    square: Boolean = false,
    round: Boolean = false,
) {
    val c = SlouchcatTheme.colors
    val said = listOfNotNull(name, caption).joinToString(stringResource(R.string.format_listSeparator))
    val width: Dp = 48.dp
    val height: Dp = if (square || round) 48.dp else 72.dp
    val shape = if (round) CircleShape else RoundedCornerShape(6.dp)
    Row(
        Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClick = onClick) else Modifier)
            .semantics(mergeDescendants = true) { contentDescription = said }
            .heightIn(min = 60.dp)
            .padding(horizontal = gutter(), vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.size(width, height).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
            MediaImage(image, name, Modifier.fillMaxSize())
        }
        Column(Modifier.weight(1f)) {
            Text(title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
            if (caption != null) Text(caption, style = SlouchcatType.caption, color = c.tx2, maxLines = 2, overflow = TextOverflow.Ellipsis)
        }
        if (onClick != null) SlouchcatIconImage(SlouchcatIcon.Right, contentDescription = null, size = 18.dp, tint = c.tx3)
    }
}
