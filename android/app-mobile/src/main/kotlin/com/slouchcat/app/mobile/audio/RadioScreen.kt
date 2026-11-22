package com.slouchcat.app.mobile.audio

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.PagingFooter
import com.slouchcat.app.mobile.media.pluralCount
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.api.generated.models.RadioStationState
import com.slouchcat.core.browse.radio.RadioUiState
import com.slouchcat.core.design.BarNavigation
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SectionTitle
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatChip
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTextField
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.player.RadioState
import com.slouchcat.core.strings.R
import java.util.Locale

/** What the radio page can be asked to do. */
interface RadioEvents {
    fun onBack()
    fun onRetry()
    fun onNearEnd()
    fun onSearch(text: String)
    fun onCountry(code: String?)
    fun onCountrySheet(open: Boolean)
    fun onFavoritesOnly()
    fun onFavorite(station: RadioStation)

    /** A tap on a station: plays it, or play and pause when it is the one chosen. */
    fun onStation(station: RadioStation)
}

/**
 * «Webradio» (concept 16, concept 19 part B; mockup `/radio`, photo handy 39; web `RadioPage`): the
 * search, the country (the profile's own when the offer has it) and «Meine Sender» as chips; while
 * nothing filters, «Meine Sender» as a row above the list; then the stations as tiles with logo,
 * «Live» or «Läuft», heart, name, country and genres. What plays comes from [playing] (the app's
 * radio); the title it plays shows in the small player and in the notification.
 */
@Composable
fun RadioScreen(
    ui: RadioUiState,
    stations: Paged<RadioStation>,
    playing: RadioState?,
    logo: (RadioStation) -> ImageSet,
    events: RadioEvents,
    snackbar: SnackbarHostState = SnackbarHostState(),
) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    Scaffold(
        containerColor = c.bg,
        topBar = { TopBar(stringResource(R.string.app_radio_title), navigation = BarNavigation.Back, navigationLabel = stringResource(R.string.common_back), onNavigation = events::onBack) },
        snackbarHost = { SlouchcatSnackbarHost(snackbar) },
    ) { padding ->
        LazyVerticalGrid(
            columns = GridCells.Adaptive(if (wide) 190.dp else 150.dp),
            contentPadding = PaddingValues(start = gutter(), end = gutter(), top = padding.calculateTopPadding(), bottom = padding.calculateBottomPadding() + 24.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.fillMaxSize(),
        ) {
            item(span = { GridItemSpan(maxLineSpan) }) { Filters(ui, events) }
            if (!ui.filter.favorites && ui.filter.text.isBlank()) {
                item(span = { GridItemSpan(maxLineSpan) }) { Favorites(ui, playing, logo, events) }
            }
            item(span = { GridItemSpan(maxLineSpan) }) {
                val heading = stringResource(if (ui.filter.unfiltered) R.string.radio_page_allStations else R.string.radio_page_results)
                Row(Modifier.fillMaxWidth().padding(top = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    SectionTitle(heading, inset = false, modifier = Modifier.weight(1f))
                    if (stations.items is Loadable.Ready) Text(pluralCount(R.plurals.radio_page_count, stations.total), style = SlouchcatType.caption, color = c.tx3)
                }
            }
            when (val items = stations.items) {
                is Loadable.Ready -> {
                    items(items.value, key = { it.id }) { station ->
                        StationTile(station, playing, logo(station), events)
                    }
                    item(span = { GridItemSpan(maxLineSpan) }) {
                        LaunchedEffect(items.value.size) { events.onNearEnd() }
                        PagingFooter(stations.more, onRetry = events::onNearEnd)
                    }
                }
                Loadable.Empty -> item(span = { GridItemSpan(maxLineSpan) }) {
                    EmptyState(stringResource(R.string.radio_page_empty_title), body = stringResource(R.string.radio_page_empty_body), icon = SlouchcatIcon.Radio)
                }
                else -> item(span = { GridItemSpan(maxLineSpan) }) {
                    LoadableContent(items, onRetry = events::onRetry) {}
                }
            }
        }
    }
    if (ui.countrySheet) CountrySheet(ui, events)
}

@Composable
private fun Filters(ui: RadioUiState, events: RadioEvents) {
    var text by rememberSaveable { mutableStateOf(ui.filter.text) }
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        SlouchcatTextField(
            value = text,
            onValueChange = { text = it },
            label = stringResource(R.string.radio_page_searchLabel),
            placeholder = stringResource(R.string.radio_page_searchPlaceholder),
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
            keyboardActions = KeyboardActions(onSearch = { events.onSearch(text) }),
        )
        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            item {
                val country = ui.filter.country
                SlouchcatChip(
                    when {
                        country != null && country == ui.myCountry -> stringResource(R.string.radio_page_quick_myCountry, countryName(country))
                        country != null -> countryName(country)
                        else -> stringResource(R.string.app_radio_allCountries)
                    },
                    selected = country != null,
                    onClick = { events.onCountrySheet(true) },
                    icon = SlouchcatIcon.Globe,
                    trailingIcon = SlouchcatIcon.Down,
                    modifier = Modifier.heightIn(min = 48.dp),
                )
            }
            item {
                SlouchcatChip(
                    stringResource(R.string.radio_page_quick_favorites),
                    selected = ui.filter.favorites,
                    onClick = events::onFavoritesOnly,
                    icon = SlouchcatIcon.Heart,
                    modifier = Modifier.heightIn(min = 48.dp),
                )
            }
        }
    }
}

/** «Meine Sender» as a row (web `rows.favorites`), or the hint how a station gets there. */
@Composable
private fun Favorites(ui: RadioUiState, playing: RadioState?, logo: (RadioStation) -> ImageSet, events: RadioEvents) {
    Column(Modifier.fillMaxWidth().padding(top = 8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        SectionTitle(stringResource(R.string.radio_page_rows_favorites), inset = false)
        when (val favorites = ui.favorites) {
            is Loadable.Ready -> LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                items(favorites.value, key = { it.id }) { station ->
                    StationTile(station, playing, logo(station), events, Modifier.width(150.dp))
                }
            }
            Loadable.Empty -> Note(stringResource(R.string.radio_page_rows_noFavorites))
            else -> LoadableContent(favorites, onRetry = events::onRetry) {}
        }
    }
}

/** A station (`.md-st`): logo with heart and «Live» or «Läuft», name, country and genres. */
@Composable
private fun StationTile(station: RadioStation, playing: RadioState?, logo: ImageSet, events: RadioEvents, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(16.dp)
    val chosen = playing?.station?.id == station.id && playing.error == null
    val running = chosen && playing!!.wantsToPlay
    val action = when {
        running -> stringResource(R.string.radio_page_pause, station.name)
        chosen -> stringResource(R.string.radio_page_resume, station.name)
        else -> stringResource(R.string.radio_page_play, station.name)
    }
    val note = when (station.state) {
        RadioStationState.BROKEN -> stringResource(R.string.radio_page_broken)
        else -> null
    }
    Column(
        modifier
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, if (chosen) c.accent else c.line), shape)
            .clickable(role = Role.Button, onClickLabel = action) { events.onStation(station) }
            .padding(10.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.fillMaxWidth().aspectRatio(1f).clip(RoundedCornerShape(12.dp))) {
            MediaImage(logo, station.name, Modifier.fillMaxSize())
            val heart = stringResource(if (station.favorite) R.string.app_radio_favoriteOn else R.string.app_radio_favoriteOff, station.name)
            Box(Modifier.align(Alignment.TopEnd).padding(4.dp).clip(CircleShape).background(c.scrim)) {
                SlouchcatIconButton(SlouchcatIcon.Heart, heart, onClick = { events.onFavorite(station) }, tint = if (station.favorite) c.accent else c.tx)
            }
            Row(
                Modifier.align(Alignment.BottomStart).padding(6.dp).clip(CircleShape).background(c.scrim).padding(horizontal = 10.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                SlouchcatIconImage(if (running) SlouchcatIcon.Pause else SlouchcatIcon.Play, contentDescription = null, size = 14.dp, tint = c.tx)
                Text(stringResource(if (running) R.string.radio_page_playing else R.string.radio_player_live), style = SlouchcatType.label, color = c.tx)
            }
        }
        Text(station.name, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
        val line = listOfNotNull(station.country?.let { countryName(it) }, station.genres.take(2).joinToString(stringResource(R.string.format_listSeparator)).ifBlank { null })
            .joinToString(stringResource(R.string.format_separator))
        if (line.isNotEmpty()) Text(line, style = SlouchcatType.caption, color = c.tx3, maxLines = 1, overflow = TextOverflow.Ellipsis)
        note?.let { Text(it, style = SlouchcatType.caption, color = c.warn) }
    }
}

/** «Land»: every country of the offer with its count, «Alle Länder» first. */
@Composable
private fun CountrySheet(ui: RadioUiState, events: RadioEvents) {
    val title = stringResource(R.string.app_radio_countryTitle)
    SlouchcatSheet(title, onDismiss = { events.onCountrySheet(false) }) {
        SheetTitle(title)
        Column(Modifier.fillMaxWidth().semantics { contentDescription = title }) {
            Choice(stringResource(R.string.app_radio_allCountries), selected = ui.filter.country == null) { events.onCountry(null) }
            ui.countries.forEach { facet ->
                Choice(countryName(facet.`value`), selected = ui.filter.country == facet.`value`, detail = pluralCount(R.plurals.radio_page_count, facet.count)) {
                    events.onCountry(facet.`value`)
                }
            }
        }
    }
}

/** The name of a country code in the language of the app («Schweiz»); the code when Android does not know it. */
@Composable
fun countryName(code: String): String {
    val locale = LocalConfiguration.current.locales[0]
    return runCatching { Locale.Builder().setRegion(code).build().getDisplayCountry(locale) }.getOrNull()?.ifBlank { null } ?: code
}
