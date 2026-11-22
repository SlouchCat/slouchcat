package com.slouchcat.app.tv.item

import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.IconAction
import com.slouchcat.app.tv.browse.KindLine
import com.slouchcat.app.tv.browse.MediaCard
import com.slouchcat.app.tv.browse.MetaLine
import com.slouchcat.app.tv.browse.ProgressBar
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.kindName
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.browse.runtimeText
import com.slouchcat.app.tv.browse.screenWidth
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.home.TitleOrLogo
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.BackdropMode
import com.slouchcat.app.tv.ui.BadgeTone
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.CardShape
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvActions
import com.slouchcat.app.tv.ui.TvBackdrop
import com.slouchcat.app.tv.ui.TvBadge
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvCard
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.app.tv.ui.TvDetailRows
import com.slouchcat.app.tv.ui.TvDialog
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.DetailValue
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.CastMember
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.PlayTarget
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.hasCheck
import com.slouchcat.core.media.listable
import com.slouchcat.core.media.playTarget
import com.slouchcat.core.media.share
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.strings.R
import kotlinx.coroutines.launch

/** Where the detail page leads; `null`: that page is not registered (no control). */
class ItemLinks(
    val item: ((String) -> Unit)?,
    val person: ((Long) -> Unit)?,
    /** «Abspielen» of the title or an episode: the player of stage 3 ([PlayTarget.id]); `null` without it. */
    val play: ((String) -> Unit)?,
    /** «Von vorn». */
    val playFromStart: ((String) -> Unit)?,
    /** The trailer in the app of YouTube on the TV. */
    val trailer: (String) -> Unit,
)

private const val PLAY = "play"
private const val RETRY = "retry"

/**
 * The detail page of a film, a series, a season or an episode (mockup `/item/:i`, `browse.js`):
 * the art layer in full behind the hero, darkened below it; kind, logo or title, facts, tagline,
 * summary, the progress, then the actions: «Abspielen» (only with the player of stage 3), «Von
 * vorn», and round buttons «Merken», «Gesehen», «Empfehlen», «Trailer», «Mehr Infos» (their label
 * shows in focus). A series lists its seasons as chips (the focus chooses) over the episodes of
 * the chosen one; the cast is a row of portraits. Back from a person or an episode finds the
 * element of before.
 */
@Composable
fun ItemScreen(state: ItemUiState, images: ServerImages, links: ItemLinks, events: ItemEvents) {
    val focus = rememberFocusMemory()
    var inHero by remember { mutableStateOf(true) }
    var menu by remember { mutableStateOf(false) }
    val detail = state.detail
    val card = remember(detail) { detail?.let { MediaItem.from(it, images) } }
    Box(Modifier.fillMaxSize()) {
        TvBackdrop(card?.backdrop?.forWidth(screenWidth()), mode = if (inHero) BackdropMode.Hero else BackdropMode.Dim)
        if (detail == null || card == null) {
            LoadableContent(
                state.item,
                onRetry = events::retry,
                modifier = Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.DetailTop),
                retryModifier = Modifier.focusKey(focus, RETRY),
            ) {}
        } else {
            val list = rememberLazyListState()
            val scope = rememberCoroutineScope()
            LazyColumn(state = list, modifier = Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = TvDimens.Browse.DetailTop)) {
                item(key = "hero") {
                    Hero(detail, card, state, links, events, focus, onMore = { menu = true }, onFocus = { has ->
                        if (has && !inHero) scope.launch { list.animateScrollToItem(0) }
                        inHero = has
                    })
                }
                if (detail.kind == ItemKind.SERIES || detail.kind == ItemKind.SEASON) {
                    item(key = "episodes") { Episodes(detail, state, images, links, events, focus) }
                }
                val cast = detail.cast.orEmpty().distinctBy { it.personId }
                if (cast.isNotEmpty()) item(key = "cast") { Cast(detail.cast.orEmpty(), images, links, focus) }
            }
        }
    }
    focus.StartFocus(if (detail == null) RETRY else firstAction(detail, links), state.item::class)
    state.ask?.let { Ask(it, events, focus) }
    if (menu && detail != null) MoreMenu(detail, onDismiss = { menu = false; focus.restore() })
}

/** The first round button when there is no «Abspielen» (2b, or a title without a file). */
private fun firstAction(detail: ItemDetail, links: ItemLinks): String = when {
    links.play != null && playTarget(detail) != null -> PLAY
    listable(detail.kind) -> "watch"
    hasCheck(detail.kind) -> "seen"
    else -> "more"
}

@Composable
private fun Hero(
    detail: ItemDetail,
    card: MediaItem,
    state: ItemUiState,
    links: ItemLinks,
    events: ItemEvents,
    focus: FocusMemory,
    onMore: () -> Unit,
    onFocus: (Boolean) -> Unit,
) {
    val c = SlouchcatTheme.colors
    val mark = detail.watchedMark
    val target = playTarget(detail)
    Column(
        Modifier
            .fillMaxWidth()
            .heightIn(min = TvDimens.Browse.HeroHeight)
            .padding(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.Browse.DetailTop, bottom = TvDimens.Browse.DetailBottom)
            .onFocusChanged { onFocus(it.hasFocus) },
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(bottom = TvDimens.Browse.KindGap)) {
            kindName(detail.kind)?.let { KindLine(if (detail.kind == ItemKind.EPISODE || detail.kind == ItemKind.SEASON) listOfNotNull(detail.seriesRef?.title ?: detail.seriesTitle, it).joinToString(stringResource(R.string.format_separator)) else it) }
            if (mark == WatchedMark.Full) TvBadge(stringResource(R.string.watched_state_full), tone = BadgeTone.Ok, icon = SlouchcatIcon.Check.vector)
            if (mark == WatchedMark.Half) TvBadge(stringResource(R.string.watched_state_half), tone = BadgeTone.Plain, icon = SlouchcatIcon.Check.vector)
        }
        TitleOrLogo(card, TvDimens.Browse.DetailLogoHeight, TvDimens.Browse.DetailLogoWidth)
        MetaLine(facts(detail), Modifier.padding(top = TvDimens.Browse.MetaTop), rating = detail.rating)
        detail.tagline?.takeIf { it.isNotBlank() }?.let {
            Text(it, style = TvType.largeStrong, color = c.tx, modifier = Modifier.padding(top = TvDimens.l), maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        detail.overview?.takeIf { it.isNotBlank() }?.let {
            Text(it, style = TvType.body, color = c.tx2, maxLines = 3, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.s).widthIn(max = TvDimens.Browse.DetailOverview))
        }
        target?.resume?.let { resume ->
            Row(Modifier.padding(top = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
                ProgressBar(share(resume), TvDimens.Browse.ProgressWidth)
                val left = stringResource(R.string.app_item_left, runtimeText(com.slouchcat.core.media.minutesLeft(resume)))
                val episode = target.episode
                Text(if (episode != null) "${episodeLabel(episode)} · $left" else left, style = TvType.small, color = c.tx2)
            }
        }
        Row(Modifier.padding(top = TvDimens.Browse.DetailActionsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
            val play = links.play
            if (play != null && target != null) {
                TvButton(playLabel(target), onClick = { play(target.id) }, icon = SlouchcatIcon.Play.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, PLAY))
                val again = links.playFromStart
                if (again != null && target.resume != null) {
                    IconAction(SlouchcatIcon.Rew.vector, stringResource(R.string.app_item_fromStart), onClick = { again(target.id) }, modifier = Modifier.focusKey(focus, "again"))
                }
            }
            if (listable(detail.kind)) {
                IconAction(
                    if (detail.inWatchlist) SlouchcatIcon.BookmarkOn.vector else SlouchcatIcon.Bookmark.vector,
                    stringResource(if (detail.inWatchlist) R.string.item_watchlist_added else R.string.item_watchlist_add),
                    onClick = events::onWatchlist,
                    pressed = detail.inWatchlist,
                    modifier = Modifier.focusKey(focus, "watch"),
                )
            }
            if (hasCheck(detail.kind)) {
                IconAction(
                    SlouchcatIcon.Check.vector,
                    stringResource(
                        when (mark) {
                            WatchedMark.Full -> R.string.watched_state_full
                            WatchedMark.Half -> R.string.watched_state_half
                            WatchedMark.None -> R.string.watched_hint_set
                        },
                    ),
                    onClick = events::onWatched,
                    pressed = mark == WatchedMark.Full,
                    modifier = Modifier.focusKey(focus, "seen"),
                )
            }
            if (detail.canRecommend) {
                IconAction(
                    SlouchcatIcon.Heart.vector,
                    stringResource(if (detail.recommendedByMe) R.string.item_recommend_done else R.string.item_recommend_add),
                    onClick = events::onRecommendAsk,
                    pressed = detail.recommendedByMe,
                    modifier = Modifier.focusKey(focus, "recommend"),
                )
            }
            state.trailer?.let { youtube ->
                IconAction(SlouchcatIcon.Film.vector, stringResource(R.string.item_trailer), onClick = { links.trailer(youtube) }, modifier = Modifier.focusKey(focus, "trailer"))
            }
            val series = detail.seriesRef?.id ?: detail.series?.id
            val openSeries = links.item
            if (series != null && openSeries != null && detail.kind != ItemKind.SERIES) {
                IconAction(SlouchcatIcon.Tv.vector, stringResource(R.string.item_seriesLink), onClick = { openSeries(series) }, modifier = Modifier.focusKey(focus, "series"))
            }
            IconAction(SlouchcatIcon.Info.vector, stringResource(R.string.tv_item_more), onClick = onMore, modifier = Modifier.focusKey(focus, "more"))
        }
    }
}

/** The facts of the hero: year, then seasons or the length, then up to three genres. */
@Composable
private fun facts(detail: ItemDetail): List<String> = buildList {
    detail.year?.let { add(it.toString()) }
    when (detail.kind) {
        ItemKind.SERIES -> detail.seasons?.size?.takeIf { it > 0 }?.let { add(pluralCount(R.plurals.app_item_seasons, it)) }
        ItemKind.SEASON -> detail.episodes?.size?.takeIf { it > 0 }?.let { add(pluralCount(R.plurals.app_item_episodes, it)) }
        else -> detail.runtimeMinutes?.takeIf { it > 0 }?.let { add(runtimeText(it)) }
    }
    detail.genres.take(GENRES).takeIf { it.isNotEmpty() }?.let { add(it.joinToString(stringResource(R.string.format_separator))) }
}

/** «S1 · F3». */
@Composable
private fun episodeLabel(episode: ItemRef): String =
    stringResource(R.string.home_continue_episode, (episode.seasonNumber ?: 1).toString(), (episode.episodeNumber ?: 0).toString())

/** The words of the big button (web `playTarget`, mockup `cta`). */
@Composable
private fun playLabel(target: PlayTarget): String = when (target.kind) {
    PlayTarget.Kind.Play -> stringResource(R.string.app_item_play)
    PlayTarget.Kind.Resume -> stringResource(R.string.app_item_resume)
    PlayTarget.Kind.Again -> stringResource(R.string.app_item_again)
    PlayTarget.Kind.Episode -> stringResource(R.string.app_item_playEpisode, target.episode?.let { episodeLabel(it) }.orEmpty())
    PlayTarget.Kind.EpisodeResume -> stringResource(R.string.app_item_resumeEpisode, target.episode?.let { episodeLabel(it) }.orEmpty())
}

/** `.br-eps`: the seasons as chips, the episodes of the chosen one, the summary of the focused episode. */
@Composable
private fun Episodes(detail: ItemDetail, state: ItemUiState, images: ServerImages, links: ItemLinks, events: ItemEvents, focus: FocusMemory) {
    val c = SlouchcatTheme.colors
    val seasons = detail.seasons.orEmpty()
    var focused by remember { mutableStateOf<ItemRef?>(null) }
    Column(Modifier.fillMaxWidth()) {
        Text(
            stringResource(R.string.item_episodes_title),
            style = TvType.largeStrong,
            modifier = Modifier.padding(start = LocalContentStart.current, top = TvDimens.l),
        )
        if (detail.kind == ItemKind.SERIES && seasons.isNotEmpty()) {
            TvCardRow(title = null, items = seasons, key = { it.id }, focus = focus, rowKey = "season") { season, modifier ->
                ControlChip(
                    season.title,
                    SlouchcatIcon.List.vector,
                    onClick = { events.onSeason(season.id) },
                    on = season.id == state.seasonId,
                    modifier = modifier.onFocusChanged { if (it.isFocused && season.id != state.seasonId) events.onSeason(season.id) },
                )
            }
        }
        when (val episodes = state.episodes) {
            null -> Unit
            is Loadable.Ready -> {
                val list = episodes.value.episodes.orEmpty()
                val cards = remember(list) { list.map { MediaItem.from(it, images) } }
                TvCardRow(
                    title = null,
                    items = cards,
                    key = { it.id },
                    focus = focus,
                    rowKey = "ep",
                ) { episode, modifier ->
                    val ref = list.first { it.id == episode.id }
                    val number = episode.episodeNumber
                    val name = if (number != null) stringResource(R.string.app_item_episodeTitle, number.toString(), episode.title) else episode.title
                    val minutes = episode.runtimeMinutes?.let { runtimeText(it) }
                    val seen = if (ref.watchedMark == WatchedMark.Full) stringResource(R.string.app_item_seen) else null
                    val open = links.play?.let { play -> { play(episode.id) } } ?: links.item?.let { show -> { show(episode.id) } }
                    MediaCard(
                        episode,
                        open,
                        modifier.onFocusChanged { if (it.isFocused) focused = ref },
                        shape = CardShape.Wide,
                        caption = name,
                        subcaption = listOfNotNull(minutes, seen).joinToString(stringResource(R.string.format_separator)).ifEmpty { null },
                        progress = episode.progress,
                    )
                }
                val shown = focused?.takeIf { f -> list.any { it.id == f.id } } ?: list.firstOrNull()
                Column(Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX).heightIn(min = TvDimens.Browse.EpisodeInfo).widthIn(max = TvDimens.Browse.DetailOverview + LocalContentStart.current)) {
                    if (shown != null) {
                        Text(listOfNotNull(episodeLabel(shown), shown.title).joinToString(stringResource(R.string.format_separator)), style = TvType.largeStrong, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        shown.overview?.let { Text(it, style = TvType.body, color = c.tx2, maxLines = 2, overflow = TextOverflow.Ellipsis) }
                    }
                }
            }
            else -> LoadableContent(episodes, onRetry = events::retry, modifier = Modifier.padding(start = LocalContentStart.current)) {}
        }
    }
}

/** «Besetzung und Team»: portraits with name and role; OK opens the person. */
@Composable
private fun Cast(cast: List<CastMember>, images: ServerImages, links: ItemLinks, focus: FocusMemory) {
    val people = cast.groupBy { it.personId }.values.toList()
    val roles = people.associate { group -> group.first().personId to group.map { roleOf(it) }.distinct().joinToString(", ") }
    TvCardRow(
        title = stringResource(R.string.app_item_cast),
        items = people.map { it.first() },
        key = { it.personId },
        focus = focus,
        rowKey = "cast",
        info = { CardInfo(it.name, roles[it.personId]) },
    ) { person, modifier ->
        TvCard(
            title = listOfNotNull(person.name, roles[person.personId]).joinToString(", "),
            image = images.single(person.image).forWidth(TvDimens.Browse.CastWidth),
            onClick = { links.person?.invoke(person.personId) },
            modifier = modifier,
            caption = person.name,
            subcaption = roles[person.personId],
            width = TvDimens.Browse.CastWidth,
        )
    }
}

@Composable
private fun roleOf(member: CastMember): String = when (member.role) {
    CastMember.Role.DIRECTOR -> stringResource(R.string.item_cast_role_director)
    CastMember.Role.WRITER -> stringResource(R.string.item_cast_role_writer)
    CastMember.Role.CREATOR -> stringResource(R.string.item_cast_role_creator)
    else -> member.character?.takeIf { it.isNotBlank() }?.let { stringResource(R.string.person_as, it) } ?: stringResource(R.string.person_department_acting)
}

/** The questions before a change: clearing a check mark of a group, recommending. */
@Composable
private fun Ask(ask: ItemAsk, events: ItemEvents, focus: FocusMemory) {
    val cancel = remember { FocusRequester() }
    val close = {
        events.onDismissAsk()
        focus.restore()
    }
    when (ask) {
        is ItemAsk.Watched -> TvDialog(
            stringResource(if (ask.half) R.string.watched_ask_halfTitle else R.string.watched_ask_removeTitle, ask.title),
            onDismiss = close,
            initialFocus = cancel,
        ) {
            val body = when {
                ask.half -> pluralCount(R.plurals.watched_ask_halfBody, ask.count)
                ask.count > 0 -> pluralCount(R.plurals.watched_ask_removeBody, ask.count)
                else -> stringResource(R.string.watched_ask_removeBodyAll)
            }
            Text(body, style = TvType.large, color = SlouchcatTheme.colors.tx2)
            TvActions(vertical = true) {
                if (ask.half) TvButton(stringResource(R.string.watched_ask_markAll), onClick = { events.onAskChoice(true); focus.restore() }, icon = SlouchcatIcon.Check.vector)
                TvButton(stringResource(R.string.watched_ask_removeAll), onClick = { events.onAskChoice(false); focus.restore() }, icon = SlouchcatIcon.Close.vector)
                TvButton(stringResource(R.string.common_cancel), onClick = close, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(cancel))
            }
        }
        ItemAsk.Recommend -> TvDialog(stringResource(R.string.app_item_recommendTitle), onDismiss = close, initialFocus = cancel) {
            Text(stringResource(R.string.item_recommend_hint), style = TvType.large, color = SlouchcatTheme.colors.tx2)
            TvActions {
                TvButton(stringResource(R.string.app_item_recommendConfirm), onClick = { events.onRecommendConfirm(); focus.restore() }, icon = SlouchcatIcon.Heart.vector)
                TvButton(stringResource(R.string.common_cancel), onClick = close, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(cancel))
            }
        }
    }
}

/** «Mehr Infos» (mockup `brMore`): the whole summary, the facts, the file; Up and Down scroll. */
@Composable
private fun MoreMenu(detail: ItemDetail, onDismiss: () -> Unit) {
    val close = remember { FocusRequester() }
    TvSideMenu(detail.title, onDismiss = onDismiss, initialFocus = close) {
        TvButton(stringResource(R.string.common_close), onClick = onDismiss, icon = SlouchcatIcon.Close.vector, small = true, kind = TvButtonKind.Ghost, modifier = Modifier.focusRequester(close))
        detail.tagline?.takeIf { it.isNotBlank() }?.let { Text(it, style = TvType.largeStrong, modifier = Modifier.padding(top = TvDimens.l)) }
        ReadBlock(Modifier.padding(top = TvDimens.m)) {
            Text(detail.overview?.takeIf { it.isNotBlank() } ?: stringResource(R.string.app_item_noOverview), style = TvType.body, color = SlouchcatTheme.colors.tx2)
        }
        val rows = buildList<Pair<String, @Composable androidx.compose.foundation.layout.RowScope.() -> Unit>> {
            detail.year?.let { year -> add(stringResource(R.string.library_filters_year) to { DetailValue(year.toString()) }) }
            detail.rating?.let { age -> add(stringResource(R.string.library_filters_rating) to { DetailValue(stringResource(R.string.rating_age, age.toString())) }) }
            detail.runtimeMinutes?.takeIf { it > 0 }?.let { min -> add(stringResource(R.string.library_sort_runtime) to { DetailValue(runtimeText(min)) }) }
            if (detail.genres.isNotEmpty()) add(stringResource(R.string.library_filters_genre) to { DetailValue(detail.genres.joinToString(", ")) })
            detail.mediaFiles.firstOrNull()?.let { file ->
                val video = listOfNotNull(file.videoCodec?.uppercase(java.util.Locale.ROOT), file.height?.let { stringResource(R.string.format_height, it.toString()) }).joinToString(stringResource(R.string.format_separator))
                if (video.isNotEmpty()) add(stringResource(R.string.item_file_video) to { DetailValue(video) })
                file.audioCodec?.let { audio -> add(stringResource(R.string.item_file_audio) to { DetailValue(audio.uppercase(java.util.Locale.ROOT)) }) }
                add(stringResource(R.string.item_file_container) to { DetailValue(file.container.uppercase(java.util.Locale.ROOT)) })
            }
        }
        if (rows.isNotEmpty()) ReadBlock(Modifier.padding(top = TvDimens.l)) { TvDetailRows(rows, keyWidth = TvDimens.Settings.Key) }
    }
}

/**
 * A block of text in a menu that takes the focus (code review m-6): Down moves through the blocks
 * and the menu scrolls them into view; a thin frame shows where the focus is.
 */
@Composable
private fun ReadBlock(modifier: Modifier, content: @Composable () -> Unit) {
    val c = SlouchcatTheme.colors
    var focused by remember { mutableStateOf(false) }
    Box(
        modifier
            .fillMaxWidth()
            .onFocusChanged { focused = it.isFocused }
            .focusable()
            .border(androidx.compose.foundation.BorderStroke(TvDimens.borderThick, if (focused) c.tx else androidx.compose.ui.graphics.Color.Transparent), androidx.compose.foundation.shape.RoundedCornerShape(TvDimens.radiusField))
            .padding(TvDimens.s),
    ) { content() }
}

private const val GENRES = 3
