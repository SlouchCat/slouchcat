package com.slouchcat.app.tv.audio

import androidx.activity.compose.BackHandler
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.focusProperties
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.PageHead
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.search.TYPING_START
import com.slouchcat.app.tv.search.TvTyping
import com.slouchcat.app.tv.search.hardwareTyping
import com.slouchcat.app.tv.search.rememberTypingMode
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.CardShape
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.OnRemoteKey
import com.slouchcat.app.tv.ui.RemoteKey
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvCard
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMenuItem
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvRemoteKeys
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.api.generated.models.RadioStationState
import com.slouchcat.core.browse.radio.RadioUiState
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.player.RadioError
import com.slouchcat.core.player.RadioState
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl
import java.util.Locale

/** What the radio page can be asked to do. */
interface RadioEvents {
    fun onRetry()
    fun onNearEnd()
    fun onSearch(text: String)
    fun onCountry(code: String?)
    fun onFavorite(station: RadioStation)

    /** OK on a station: plays it, or pauses and plays it again when it is the one chosen. */
    fun onStation(station: RadioStation)

    /** The card «Jetzt läuft»: the station's own page. */
    fun onOpenPlaying()
}

/** Test tags and focus keys of the radio page. */
object RadioKeys {
    const val NOW = "radio-now"
    const val SEARCH = "radio-search"
    const val COUNTRY = "radio-country"

    /** No element: the start focus waits for the lists. */
    const val WAIT = "radio-wait"
    const val CLEAR = "radio-clear"
    const val RETRY = "radio-retry"
    const val SEARCH_APPLY = "radio-search-apply"

    /** The star of a station of «Meine Sender» on its tile (no control). */
    const val STAR = "radio-star"
    fun station(row: String, id: Long) = "$row-$id"
    fun heart(row: String, id: Long) = "heart-$row-$id"
    fun country(code: String?) = "country-${code ?: "all"}"
}

/**
 * «Webradio» (concept 16, concept 19 part B; mockup `/radio`, photo TV 31 as meant: stations as
 * tiles like the web app and the phone): the head with the count, the hint «OK startet einen Sender,
 * nochmals OK hält ihn an», the card «Jetzt läuft» while a station plays (OK opens its page), the
 * search with the keyboard of the TV's search and the country (a side menu of the countries of the
 * offer with their counts, the profile's own chosen first, as the phone's sheet), then «Meine
 * Sender» and the stations as rows of tiles: logo, «Läuft», name, country and genres, the heart below.
 */
@Composable
fun RadioScreen(ui: RadioUiState, stations: Paged<RadioStation>, playing: RadioState?, server: HttpUrl?, events: RadioEvents) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    var searching by rememberSaveable { mutableStateOf(false) }
    var countries by rememberSaveable { mutableStateOf(false) }
    // Few rows, each a lazy row: a plain column keeps every row composed (the card «Jetzt läuft»
    // appears above the focused row, and Down always finds the next row).
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(top = TvDimens.safeY, bottom = TvDimens.safeY)) {
        Column(Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX)) {
            val count = (stations.items as? Loadable.Ready)?.let { pluralCount(R.plurals.radio_page_count, stations.total) }
            PageHead(SlouchcatIcon.Radio.vector, stringResource(R.string.app_radio_title), count)
            Text(stringResource(R.string.tv_radio_hint), style = TvType.large, color = c.tx2, modifier = Modifier.padding(top = TvDimens.s))
        }
        if (playing?.station != null) {
            NowCard(playing, server, events, Modifier.padding(start = LocalContentStart.current, top = TvDimens.l).focusKey(focus, RadioKeys.NOW).testTag(RadioKeys.NOW))
        }
        run {
            PageRow(Modifier.padding(top = TvDimens.l)) {
                val text = ui.filter.text
                ControlChip(
                    if (text.isBlank()) stringResource(R.string.radio_page_searchLabel) else stringResource(R.string.radio_page_filters_search, text),
                    SlouchcatIcon.Search.vector,
                    onClick = { searching = true },
                    on = text.isNotBlank(),
                    modifier = Modifier.focusKey(focus, RadioKeys.SEARCH).testTag(RadioKeys.SEARCH),
                )
                if (ui.countries.isNotEmpty()) {
                    ControlChip(
                        ui.filter.country?.let { countryLabel(it, ui.myCountry) } ?: stringResource(R.string.app_radio_allCountries),
                        SlouchcatIcon.Globe.vector,
                        onClick = { countries = true },
                        on = ui.filter.country != null,
                        menu = true,
                        modifier = Modifier.focusKey(focus, RadioKeys.COUNTRY).testTag(RadioKeys.COUNTRY),
                    )
                }
                if (text.isNotBlank()) {
                    ControlChip(stringResource(R.string.radio_page_filters_reset), SlouchcatIcon.Close.vector, onClick = { events.onSearch("") }, ghost = true, modifier = Modifier.focusKey(focus, RadioKeys.CLEAR).testTag(RadioKeys.CLEAR))
                }
            }
        }
        val favorites = (ui.favorites as? Loadable.Ready)?.value
        if (!favorites.isNullOrEmpty() && ui.filter.text.isBlank()) {
            StationRow(stringResource(R.string.radio_page_rows_favorites), "fav", favorites, playing, server, focus, events, Modifier.padding(top = TvDimens.l))
        }
        run {
            val heading = stringResource(if (ui.filter.unfiltered) R.string.radio_page_allStations else R.string.radio_page_results)
            when (val items = stations.items) {
                is Loadable.Ready -> StationRow(heading, "st", items.value, playing, server, focus, events, Modifier.padding(top = TvDimens.l), onNearEnd = events::onNearEnd)
                Loadable.Empty -> EmptyState(
                    SlouchcatIcon.Radio.vector,
                    stringResource(if (ui.filter.unfiltered) R.string.radio_page_offerEmpty_title else R.string.radio_page_empty_title),
                    stringResource(if (ui.filter.unfiltered) R.string.radio_page_offerEmpty_body else R.string.radio_page_empty_body),
                    modifier = Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX),
                )
                else -> LoadableContent(
                    items,
                    onRetry = events::onRetry,
                    modifier = Modifier.padding(start = LocalContentStart.current, top = TvDimens.l),
                    retryModifier = Modifier.focusKey(focus, RadioKeys.RETRY).testTag(RadioKeys.RETRY),
                ) {}
            }
        }
    }
    val first = (stations.items as? Loadable.Ready)?.value?.firstOrNull()
    val favorite = (ui.favorites as? Loadable.Ready)?.value?.firstOrNull()?.takeIf { ui.filter.text.isBlank() }
    focus.StartFocus(
        when {
            playing?.station != null -> RadioKeys.NOW
            // Nothing yet while a list loads: the focus goes to the station once both are there.
            ui.favorites is Loadable.Loading || stations.items is Loadable.Loading -> RadioKeys.WAIT
            favorite != null -> RadioKeys.station("fav", favorite.id)
            first != null -> RadioKeys.station("st", first.id)
            stations.items is Loadable.Failed -> RadioKeys.RETRY
            else -> RadioKeys.SEARCH
        },
        stations.items::class,
        ui.favorites::class,
    )
    if (countries) {
        CountryMenu(
            ui,
            onPick = { code ->
                countries = false
                events.onCountry(code)
                focus.restore()
            },
            onDismiss = {
                countries = false
                focus.restore()
            },
        )
    }
    if (searching) {
        SearchMenu(
            initial = ui.filter.text,
            onApply = { text ->
                searching = false
                events.onSearch(text)
                focus.restore()
            },
            onDismiss = {
                searching = false
                focus.restore()
            },
        )
    }
}

/** A titled row of station tiles; [onNearEnd] when the focus nears its end (the next page). */
@Composable
private fun StationRow(
    title: String,
    rowKey: String,
    stations: List<RadioStation>,
    playing: RadioState?,
    server: HttpUrl?,
    focus: FocusMemory,
    events: RadioEvents,
    modifier: Modifier,
    onNearEnd: (() -> Unit)? = null,
) {
    val c = SlouchcatTheme.colors
    var last by rememberSaveable { mutableStateOf<String?>(null) }
    var hasFocus by remember { mutableStateOf(false) }
    Column(modifier.fillMaxWidth().onFocusChanged { hasFocus = it.hasFocus }) {
        Text(
            title,
            style = TvType.largeStrong,
            color = if (hasFocus) c.tx else c.tx2,
            modifier = Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX).semantics { heading() },
        )
        LazyRow(
            contentPadding = PaddingValues(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.Card.RowTop, bottom = TvDimens.Card.RowBottom),
            horizontalArrangement = Arrangement.spacedBy(TvDimens.gap),
            // Into the row with an arrow: the tile it had last, else the first (the row remembers its
            // tile). Not for a focus asked for by name (Back to the page, a heart): that one stays.
            modifier = Modifier.fillMaxWidth().focusProperties {
                onEnter = {
                    if (requestedFocusDirection != FocusDirection.Enter) {
                        val key = last ?: stations.firstOrNull()?.let { RadioKeys.station(rowKey, it.id) }
                        key?.let { runCatching { focus.requester(it).requestFocus() } }
                    }
                }
            },
        ) {
            itemsIndexed(stations, key = { _, station -> station.id }) { index, station ->
                val name = RadioKeys.station(rowKey, station.id)
                StationTile(
                    station,
                    playing,
                    server,
                    events,
                    Modifier
                        .focusKey(focus, name)
                        .onFocusChanged {
                            if (it.isFocused) {
                                last = name
                                if (onNearEnd != null && index >= stations.size - NEAR_END) onNearEnd()
                            }
                        },
                    heartModifier = Modifier.focusKey(focus, RadioKeys.heart(rowKey, station.id)).testTag(RadioKeys.heart(rowKey, station.id)),
                )
            }
        }
    }
}

/** `.md-st`: the logo (or the name) with «Läuft», name, country and genres, the heart below. */
@Composable
private fun StationTile(station: RadioStation, playing: RadioState?, server: HttpUrl?, events: RadioEvents, modifier: Modifier, heartModifier: Modifier) {
    val c = SlouchcatTheme.colors
    val chosen = playing?.station?.id == station.id && playing.error == null
    val running = chosen && playing!!.wantsToPlay
    val action = when {
        running -> stringResource(R.string.radio_page_pause, station.name)
        chosen -> stringResource(R.string.radio_page_resume, station.name)
        else -> stringResource(R.string.radio_page_play, station.name)
    }
    Column(Modifier.width(A.Station), verticalArrangement = Arrangement.spacedBy(TvDimens.xs)) {
        TvCard(
            title = station.name,
            image = stationLogo(server, station),
            onClick = { events.onStation(station) },
            modifier = modifier.semantics { contentDescription = action },
            shape = CardShape.Square,
            tag = when {
                running -> stringResource(R.string.radio_page_playing)
                chosen -> stringResource(R.string.radio_page_paused)
                else -> null
            },
            width = A.Station,
            // A station of «Meine Sender»: the star at the top right (mockup photo 31), inside the
            // card so the focused tile shows it too (verifier 4a); no control.
            badge = if (station.favorite) {
                {
                    Box(
                        Modifier.align(Alignment.TopEnd).padding(TvDimens.s).size(A.StationStar).clip(CircleShape)
                            .background(SlouchcatTokens.Color.Scrim.Solid).testTag(RadioKeys.STAR),
                        contentAlignment = Alignment.Center,
                    ) {
                        TvIcon(SlouchcatIcon.Star, TvDimens.Button.Icon, tint = c.accent)
                    }
                }
            } else {
                null
            },
        )
        Text(station.name, style = TvType.bodyStrong, color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.xs))
        val line = listOfNotNull(station.country?.let { countryName(it) }, station.genres.take(GENRES).joinToString(stringResource(R.string.format_listSeparator)).ifBlank { null })
            .joinToString(stringResource(R.string.format_separator))
        if (line.isNotEmpty()) Text(line, style = TvType.small, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (station.state == RadioStationState.BROKEN) Text(stringResource(R.string.radio_page_broken), style = TvType.small, color = c.warn)
        val heart = stringResource(if (station.favorite) R.string.app_radio_favoriteOn else R.string.app_radio_favoriteOff, station.name)
        TvButton(
            stringResource(if (station.favorite) R.string.tv_radio_favorite else R.string.tv_radio_addFavorite),
            onClick = { events.onFavorite(station) },
            icon = if (station.favorite) SlouchcatIcon.Star.vector else SlouchcatIcon.Heart.vector,
            kind = TvButtonKind.Ghost,
            small = true,
            modifier = heartModifier
                .padding(top = TvDimens.xs)
                .semantics { contentDescription = heart },
        )
    }
}

/** «Jetzt läuft»: the station that plays with its current title; OK opens its page. */
@Composable
private fun NowCard(playing: RadioState, server: HttpUrl?, events: RadioEvents, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val station = playing.station ?: return
    val shape = RoundedCornerShape(TvDimens.radiusPanel)
    val label = stringResource(R.string.radio_player_open, station.name)
    Surface(
        onClick = events::onOpenPlaying,
        modifier = modifier.width(A.Playing).semantics(mergeDescendants = true) { contentDescription = label },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = c.s1,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
        ),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, c.accent), shape = shape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Row(Modifier.padding(TvDimens.m), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            Box(Modifier.size(A.NowHeight * 2).clip(RoundedCornerShape(TvDimens.radiusCard)).background(c.s2), contentAlignment = Alignment.Center) {
                val image = stationImage(server, playing.now?.cover?.url ?: station.logo?.url)
                if (image != null) TvImage(image, Modifier.fillMaxSize()) else TvIcon(SlouchcatIcon.Radio, TvDimens.Browse.Icon)
            }
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.xs)) {
                    TvIcon(AudioIcons.Bars, TvDimens.Button.Icon, tint = c.accent)
                    Text(stringResource(if (playing.wantsToPlay) R.string.radio_player_nowPlaying else R.string.radio_player_paused), style = TvType.small)
                }
                Text(station.name, style = TvType.largeStrong, maxLines = 1, overflow = TextOverflow.Ellipsis)
                playing.now?.let { nowLine(it.artist, it.title, it.text) }?.let { Text(it, style = TvType.body, maxLines = 1, overflow = TextOverflow.Ellipsis) }
            }
        }
    }
}

/** «Mein Land: Schweiz» for the profile's own country, else its name. */
@Composable
private fun countryLabel(code: String, mine: String?): String =
    if (code == mine) stringResource(R.string.radio_page_quick_myCountry, countryName(code)) else countryName(code)

/** «Länder»: every country of the offer with its count, «Alle Länder» first, the chosen one focused. */
@Composable
private fun CountryMenu(ui: RadioUiState, onPick: (String?) -> Unit, onDismiss: () -> Unit) {
    val initial = remember { FocusRequester() }
    val codes = listOf<String?>(null) + ui.countries.map { it.`value` }
    val counts = ui.countries.associate { it.`value` to it.count }
    TvSideMenu(stringResource(R.string.tv_radio_countries), onDismiss = onDismiss, initialFocus = initial) {
        Column(verticalArrangement = Arrangement.spacedBy(TvDimens.Menu.ListGap)) {
            for (code in codes) {
                TvMenuItem(
                    code?.let { countryLabel(it, ui.myCountry) } ?: stringResource(R.string.app_radio_allCountries),
                    onClick = { onPick(code) },
                    icon = SlouchcatIcon.Globe.vector,
                    subtitle = code?.let { counts[it] }?.let { pluralCount(R.plurals.radio_page_count, it) },
                    chevron = false,
                    selected = code == ui.filter.country,
                    modifier = Modifier.testTag(RadioKeys.country(code)).then(if (code == ui.filter.country) Modifier.focusRequester(initial) else Modifier),
                )
            }
        }
    }
}

/** The search for stations: the keyboard of the TV's search, «Anzeigen» searches. */
@Composable
private fun SearchMenu(initial: String, onApply: (String) -> Unit, onDismiss: () -> Unit) {
    var text by rememberSaveable { mutableStateOf(initial) }
    val keys = rememberFocusMemory()
    val mode = rememberTypingMode()
    TvSideMenu(stringResource(R.string.radio_page_searchLabel), onDismiss = onDismiss, initialFocus = keys.requester(TYPING_START)) {
        TvTyping(
            text,
            onText = { text = it },
            onClear = { text = "" },
            focus = keys,
            mode = mode,
            placeholder = stringResource(R.string.radio_page_searchPlaceholder),
            modifier = Modifier.fillMaxWidth().hardwareTyping(mode, { text }) { text = it },
            hint = null,
        )
        TvButton(
            stringResource(R.string.tv_library_show),
            onClick = { onApply(text) },
            icon = SlouchcatIcon.Search.vector,
            kind = TvButtonKind.Primary,
            modifier = Modifier.padding(top = TvDimens.l).testTag(RadioKeys.SEARCH_APPLY),
        )
    }
}

/** What the page of the station that plays can be asked to do. */
interface RadioListenEvents {
    fun onToggle()

    /** The Play and Pause keys of a remote with keys of their own (review m-1). */
    fun onPlay()
    fun onPause()
    fun onStop()
    fun onRetry()
    fun onFavorite()
    fun onBack()
}

/** Test tags of the station's page. */
object RadioListenKeys {
    const val PLAY = "radio-listen-play"
    const val HEART = "radio-listen-heart"
    const val STOP = "radio-listen-stop"
    const val CALM = "radio-listen-calm"
}

/**
 * The station that plays (stage 4, the audio player of a station): a page without the rail, like
 * the audio player of a title: the logo or the cover of the current title, «Webradio · Live», the
 * station, what it plays («Jetzt läuft»), play or pause, the heart and «Radio beenden». It turns
 * calm after [calmAfterMs] without a key (burn-in); Back returns, the station plays on.
 */
@Composable
fun RadioListenScreen(state: RadioState?, server: HttpUrl?, favorite: Boolean, remoteKeys: TvRemoteKeys, events: RadioListenEvents, calmAfterMs: Long = CALM_MS) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    val calm = rememberCalm(calmAfterMs, hold = false)
    OnRemoteKey(remoteKeys) { key ->
        calm.wake()
        when (key) {
            RemoteKey.PlayPause -> events.onToggle()
            RemoteKey.Play -> events.onPlay()
            RemoteKey.Pause -> events.onPause()
            RemoteKey.Stop -> events.onStop()
            else -> Unit
        }
    }
    BackHandler { if (!calm.wake()) events.onBack() }
    val station = state?.station
    val cover = stationImage(server, state?.now?.cover?.url ?: station?.logo?.url)
    val calmShare by animateFloatAsState(if (calm.calm) 1f else 0f, tween(CALM_FADE_MS))
    val song = state?.now?.let { nowLine(it.artist, it.title, it.text) }
    Box(Modifier.fillMaxSize().background(c.bg).wakes(calm)) {
        if (station != null) {
            Row(
                Modifier.fillMaxSize().alpha(1f - calmShare).padding(horizontal = TvDimens.safeX, vertical = TvDimens.safeY),
                horizontalArrangement = Arrangement.spacedBy(A.Gap),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(Modifier.size(A.Cover).clip(RoundedCornerShape(TvDimens.radiusDialog)).background(c.s2), contentAlignment = Alignment.Center) {
                    if (cover != null) TvImage(cover, Modifier.fillMaxSize()) else Text(station.name, style = TvType.h1, maxLines = 3)
                }
                Column(Modifier.widthIn(max = A.Info)) {
                    val live = stringResource(if (state.starting) R.string.radio_player_starting else if (state.wantsToPlay) R.string.radio_player_live else R.string.radio_player_paused)
                    Text(listOf(stringResource(R.string.app_radio_title), live).joinToString(stringResource(R.string.format_separator)).uppercase(), style = TvType.section, color = c.tx2)
                    Text(station.name, style = TvType.xl, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.s).semantics { heading() })
                    Row(Modifier.padding(top = A.TitleGap), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                        TvIcon(SlouchcatIcon.Radio, TvDimens.Browse.Icon, tint = c.accent)
                        Text(
                            song ?: stringResource(if (station.showsTitles) R.string.radio_player_programme else R.string.radio_player_noTitles),
                            style = TvType.large,
                            color = c.tx2,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    state.error?.let { error ->
                        TvNote(radioErrorText(error), tone = NoteTone.Warn, modifier = Modifier.padding(top = TvDimens.l))
                    }
                    Row(Modifier.padding(top = A.ButtonsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
                        val error = state.error
                        if (error != null) {
                            TvButton(stringResource(R.string.radio_player_retry), onClick = events::onRetry, icon = SlouchcatIcon.Refresh.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, RadioListenKeys.PLAY).testTag(RadioListenKeys.PLAY))
                        } else {
                            TvButton(
                                stringResource(if (state.wantsToPlay) R.string.radio_player_pause else R.string.radio_player_play),
                                onClick = events::onToggle,
                                icon = if (state.wantsToPlay) SlouchcatIcon.Pause.vector else SlouchcatIcon.Play.vector,
                                kind = TvButtonKind.Primary,
                                modifier = Modifier.focusKey(focus, RadioListenKeys.PLAY).testTag(RadioListenKeys.PLAY),
                            )
                        }
                        TvButton(
                            stringResource(if (favorite) R.string.tv_radio_favorite else R.string.tv_radio_addFavorite),
                            onClick = events::onFavorite,
                            icon = if (favorite) SlouchcatIcon.Star.vector else SlouchcatIcon.Heart.vector,
                            modifier = Modifier.focusKey(focus, RadioListenKeys.HEART).testTag(RadioListenKeys.HEART),
                        )
                        TvButton(stringResource(R.string.radio_player_stop), onClick = events::onStop, icon = SlouchcatIcon.Close.vector, modifier = Modifier.focusKey(focus, RadioListenKeys.STOP).testTag(RadioListenKeys.STOP))
                    }
                }
            }
            if (calmShare > 0f) {
                CalmScreen(cover, paused = !state.wantsToPlay, title = station.name, line = song, share = null, modifier = Modifier.alpha(calmShare).testTag(RadioListenKeys.CALM))
            }
        }
    }
    focus.StartFocus(RadioListenKeys.PLAY, station?.id)
}

@Composable
private fun radioErrorText(error: RadioError): String = stringResource(
    when (error) {
        RadioError.WatchLimit -> R.string.app_radio_error_watchLimit
        RadioError.Capacity -> R.string.app_radio_error_capacity
        RadioError.Unreachable -> R.string.app_radio_error_unreachable
        RadioError.NotAudio -> R.string.app_radio_error_notAudio
        RadioError.Ended -> R.string.app_radio_error_ended
        RadioError.Network -> R.string.app_radio_error_network
        RadioError.Failed -> R.string.app_radio_error_failed
    },
)

/** The name of a country code in the language of the app («Schweiz»); the code when Android does not know it. */
@Composable
fun countryName(code: String): String {
    val locale = LocalConfiguration.current.locales[0]
    return runCatching { Locale.Builder().setRegion(code).build().getDisplayCountry(locale) }.getOrNull()?.ifBlank { null } ?: code
}

private const val GENRES = 2
private const val NEAR_END = 5
private const val CALM_FADE_MS = 1_200
