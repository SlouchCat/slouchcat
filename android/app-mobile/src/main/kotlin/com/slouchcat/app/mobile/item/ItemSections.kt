package com.slouchcat.app.mobile.item

import android.text.format.Formatter
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
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.ClampedText
import com.slouchcat.app.mobile.media.MediaCard
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.MediaRow
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.core.api.generated.models.CastMember
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.MediaFileInfo
import com.slouchcat.core.browse.item.ItemAction
import com.slouchcat.core.browse.item.ItemEvents
import com.slouchcat.core.browse.item.ItemUiState
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SlouchcatButton
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
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.isAudio
import com.slouchcat.core.media.minutesLeft
import com.slouchcat.core.media.share
import com.slouchcat.core.media.started
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.strings.R

/**
 * The body of the detail page under the head: summary, cast, episodes and file (`.dt-main`). On
 * the phone with the page's side margin; from 700 dp in the right column without one.
 */
@Composable
fun Main(item: ItemDetail, state: ItemUiState, images: ServerImages, links: ItemLinks, events: ItemEvents) {
    val wide = isWideLayout()
    val pad = if (wide) Modifier else Modifier.padding(horizontal = gutter())
    Column(Modifier.fillMaxWidth()) {
        links.audioSections?.let { sections -> Column(pad) { sections(item) } }
        Overview(item, state.overviewOpen, events::onToggleOverview, pad.then(Modifier.widthIn(max = 720.dp)))
        Cast(item.cast.orEmpty(), images, links, wide, pad)
        if (item.kind == ItemKind.SERIES || item.kind == ItemKind.SEASON) Episodes(item, state, images, links, events, pad)
        Tech(item, state, events, pad.then(Modifier.widthIn(max = 860.dp)))
    }
}

/** Collections and extras as rows over the whole width (web `CollectionShelf`, `Extras`). */
@Composable
fun Rows(item: ItemDetail, images: ServerImages, links: ItemLinks) {
    item.collections.filter { !it.members.isNullOrEmpty() || it.cover != null }.forEach { collection ->
        val members = (collection.members ?: listOfNotNull(collection.cover)).distinctBy { it.id }
        Column(Modifier.padding(top = 12.dp)) {
            MediaRow(collection.name, onShowAll = links.collection(collection.id)) {
                items(members, key = { it.id }) { member ->
                    val card = remember(member, images) { MediaItem.from(member, images) }
                    MediaCard(card, onClick = if (member.id == item.id) null else links.item(member.id), caption = member.year?.toString())
                }
            }
        }
    }
    val extras = item.extras.orEmpty()
    if (extras.isNotEmpty()) {
        Column(Modifier.padding(top = 12.dp)) {
            MediaRow(stringResource(R.string.item_extras), onShowAll = null) {
                items(extras, key = { it.id }) { extra ->
                    val card = remember(extra, images) { MediaItem.from(extra, images) }
                    val play = links.play
                    MediaCard(card, onClick = play?.let { { it(extra.id) } }, caption = extra.runtimeMinutes?.let { runtimeText(it) })
                }
            }
        }
    }
}

/** The summary in four lines with «Mehr anzeigen» when it is longer (`.dt-ov`), closed from the start. */
@Composable
private fun Overview(item: ItemDetail, open: Boolean, onToggle: () -> Unit, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val text = item.overview?.takeIf { it.isNotBlank() }
    Column(modifier.fillMaxWidth().padding(top = 18.dp)) {
        if (text == null) {
            Text(
                if (isAudio(item.kind)) listOfNotNull(item.author, item.runtimeMinutes?.let { runtimeText(it) }).joinToString(stringResource(R.string.format_separator))
                else stringResource(R.string.app_item_noOverview),
                style = SlouchcatType.bodySmall,
                color = c.tx3,
            )
            return@Column
        }
        ClampedText(
            text,
            open = open,
            onToggle = onToggle,
            more = stringResource(R.string.app_item_more),
            less = stringResource(R.string.app_item_less),
            style = SlouchcatType.body.copy(fontSize = if (isWideLayout()) 16.5.sp else 15.5.sp, lineHeight = if (isWideLayout()) 25.sp else 24.sp),
            lines = 4,
        )
    }
}

/** «als Peter Parker», «Regie» (web `creditCaption`). */
@Composable
fun creditCaption(role: String, character: String?): String? = when {
    !character.isNullOrBlank() -> stringResource(R.string.person_as, character)
    role == "director" -> stringResource(R.string.item_cast_role_director)
    role == "writer" -> stringResource(R.string.item_cast_role_writer)
    role == "creator" -> stringResource(R.string.item_cast_role_creator)
    else -> null
}

/** «Besetzung und Team» as a row of portraits (`.cast`); a card leads to the person. */
@Composable
private fun Cast(cast: List<CastMember>, images: ServerImages, links: ItemLinks, wide: Boolean, modifier: Modifier) {
    if (cast.isEmpty()) return
    Section(stringResource(R.string.app_item_cast), modifier) {
        LazyRow(
            contentPadding = PaddingValues(horizontal = if (wide) 0.dp else gutter()),
            horizontalArrangement = Arrangement.spacedBy(14.dp),
            modifier = Modifier.fillMaxWidth().layoutEdgeToEdge(wide),
        ) {
            items(cast, key = { "${it.personId}-${it.role.value}-${it.character}" }) { member ->
                CastCard(member, images, links.person(member.personId), if (wide) 112.dp else 96.dp)
            }
        }
    }
}

/** On the phone the row runs to the screen's edges, past the section's margin. */
@Composable
private fun Modifier.layoutEdgeToEdge(wide: Boolean): Modifier {
    if (wide) return this
    val margin = gutter()
    return this.then(
        Modifier.layout { measurable, constraints ->
            val extra = (margin * 2).roundToPx()
            val placeable = measurable.measure(constraints.copy(maxWidth = constraints.maxWidth + extra, minWidth = constraints.minWidth + extra))
            layout(constraints.maxWidth, placeable.height) { placeable.place(-margin.roundToPx(), 0) }
        },
    )
}

@Composable
private fun CastCard(member: CastMember, images: ServerImages, onClick: (() -> Unit)?, width: Dp) {
    val c = SlouchcatTheme.colors
    val role = creditCaption(member.role.value, member.character)
    val said = listOfNotNull(member.name, role).joinToString(stringResource(R.string.format_listSeparator))
    val portrait = remember(member.image, images) { images.single(member.image) }
    Column(
        Modifier
            .width(width)
            .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClick = onClick) else Modifier)
            .clearAndSetSemantics { contentDescription = said },
    ) {
        val shape = RoundedCornerShape(12.dp)
        Box(Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
            MediaImage(portrait, member.name, Modifier.fillMaxSize())
        }
        Text(member.name, style = SlouchcatType.label.copy(fontSize = 13.5.sp, lineHeight = 17.sp), color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 8.dp))
        role?.let { Text(it, style = SlouchcatType.caption.copy(fontSize = 12.5.sp, lineHeight = 16.sp), color = c.tx3, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 2.dp)) }
    }
}

/**
 * «Folgen» of a series (mockup `episodes`): the seasons as chips, «Staffel als gesehen markieren»,
 * the episodes with still, number, title, duration, summary, progress and check mark. A row opens
 * the episode's page; its play button shows once the player is there (stage 5).
 */
@Composable
private fun Episodes(item: ItemDetail, state: ItemUiState, images: ServerImages, links: ItemLinks, events: ItemEvents, modifier: Modifier) {
    val seasons = item.seasons.orEmpty()
    val episodes = state.episodes ?: return
    val season = seasons.firstOrNull { it.id == state.seasonId }
    val count = (episodes as? Loadable.Ready)?.value?.episodes?.size ?: season?.childCount
    Section(stringResource(R.string.item_episodes_title), modifier, end = count?.let { pluralStringResource(R.plurals.app_item_episodes, it, it) }) {
        if (item.kind == ItemKind.SERIES && seasons.isNotEmpty()) {
            val label = stringResource(R.string.app_item_seasonsLabel)
            LazyRow(
                contentPadding = PaddingValues(horizontal = if (isWideLayout()) 0.dp else gutter()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.fillMaxWidth().layoutEdgeToEdge(isWideLayout()).semantics { contentDescription = label },
            ) {
                items(seasons, key = { it.id }) { s ->
                    SlouchcatChip(s.title, selected = s.id == state.seasonId, onClick = { events.onSeason(s.id) }, modifier = Modifier.heightIn(min = 48.dp))
                }
            }
            season?.let { s ->
                val mark = state.seasonMark(s)
                SlouchcatButton(
                    stringResource(if (mark == WatchedMark.Full) R.string.app_item_seasonUnwatched else R.string.app_item_seasonWatched),
                    onClick = events::onSeasonWatched,
                    kind = ButtonKind.Ghost,
                    small = true,
                    busy = ItemAction.SeasonWatched in state.busy,
                    leadingIcon = SlouchcatIcon.Check,
                    modifier = Modifier.padding(top = 8.dp),
                )
            }
        }
        LoadableContent(episodes, onRetry = { state.seasonId?.let(events::onSeason) ?: events.retry() }) { detail ->
            val nextUp = item.nextUp?.episode?.id
            Column(Modifier.padding(top = 8.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                detail.episodes.orEmpty().forEach { episode ->
                    EpisodeRow(episode, images, links.item(episode.id), links.play, current = episode.id == nextUp)
                }
            }
        }
    }
}

@Composable
private fun EpisodeRow(episode: ItemRef, images: ServerImages, onOpen: (() -> Unit)?, play: ((String) -> Unit)?, current: Boolean) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    val title = when {
        episode.episodeNumber != null && episode.title.isNotBlank() -> stringResource(R.string.app_item_episodeTitle, episode.episodeNumber.toString(), episode.title)
        episode.title.isNotBlank() -> episode.title
        else -> stringResource(R.string.app_item_episodeUntitled, (episode.episodeNumber ?: 0).toString())
    }
    val started = episode.progress.started()
    val seen = episode.watchedMark == WatchedMark.Full
    val meta = listOfNotNull(
        episode.runtimeMinutes?.let { runtimeText(it) },
        started?.let { stringResource(R.string.app_item_left, runtimeText(minutesLeft(it))) },
        if (seen) stringResource(R.string.app_item_seen) else null,
    ).joinToString(stringResource(R.string.format_separator))
    val still = remember(episode, images) { images.backdrops(episode.images).or(images.posters(episode.images)) }
    val open = stringResource(R.string.app_item_episodeOpen, title)
    Column(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(14.dp))
            .background(if (current) c.s1 else Color.Transparent)
            .then(if (onOpen != null) Modifier.clickable(role = Role.Button, onClickLabel = open, onClick = onOpen) else Modifier)
            .padding(8.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(14.dp), verticalAlignment = Alignment.CenterVertically) {
            val shape = RoundedCornerShape(10.dp)
            Box(Modifier.width(if (wide) 224.dp else 136.dp).aspectRatio(16f / 9f).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
                MediaImage(still, title, Modifier.fillMaxSize())
                if (seen) {
                    Box(Modifier.align(Alignment.TopEnd).padding(6.dp).size(22.dp).clip(CircleShape).background(c.ok), contentAlignment = Alignment.Center) {
                        SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 14.dp, tint = c.bg)
                    }
                }
                started?.let { ProgressBar(share(it), Modifier.align(Alignment.BottomStart)) }
            }
            Column(Modifier.weight(1f)) {
                Text(title, style = SlouchcatType.rowTitle.copy(fontSize = 15.sp, lineHeight = 19.5.sp), color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
                if (meta.isNotEmpty()) Text(meta, style = SlouchcatType.caption, color = c.tx3, modifier = Modifier.padding(top = 3.dp))
                if (wide) episode.overview?.takeIf { it.isNotBlank() }?.let { Text(it, style = SlouchcatType.caption.copy(fontSize = 14.sp, lineHeight = 20.sp), color = c.tx3, maxLines = 3, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 6.dp)) }
            }
            if (play != null) {
                SlouchcatIconButton(SlouchcatIcon.Play, contentDescription = stringResource(R.string.item_episodes_playTitle, title), onClick = { play(episode.id) }, tint = c.tx2)
            }
        }
        if (!wide) episode.overview?.takeIf { it.isNotBlank() }?.let { Text(it, style = SlouchcatType.caption.copy(fontSize = 13.5.sp, lineHeight = 19.5.sp), color = c.tx3, maxLines = 2, overflow = TextOverflow.Ellipsis) }
    }
}

/** «4K · 3840 × 2160» (web `formatResolution`). */
@Composable
private fun resolution(file: MediaFileInfo): String? {
    val w = file.width ?: return null
    val h = file.height ?: return null
    val size = stringResource(R.string.format_resolution, w.toString(), h.toString())
    val tier = when {
        h >= 2000 || w >= 3600 -> R.string.format_uhd
        h >= 1000 || w >= 1800 -> R.string.format_fhd
        h >= 700 || w >= 1200 -> R.string.format_hd
        else -> R.string.format_sd
    }
    return stringResource(R.string.format_resolutionWithClass, size, stringResource(tier))
}

/**
 * «Datei und Technik», closed from the start (`.dt-fold`): codec, resolution, sound, size,
 * container and the versions of the file; with several versions a chip per version.
 */
@Composable
private fun Tech(item: ItemDetail, state: ItemUiState, events: ItemEvents, modifier: Modifier) {
    if (item.kind == ItemKind.SERIES || item.kind == ItemKind.SEASON || isAudio(item.kind) || item.mediaFiles.isEmpty()) return
    val c = SlouchcatTheme.colors
    val context = LocalContext.current
    val file = item.mediaFiles.firstOrNull { it.id == state.versionId } ?: item.mediaFiles.first()
    val size = Formatter.formatShortFileSize(context, file.sizeBytes)
    val res = resolution(file)
    val summary = listOfNotNull(file.videoCodec?.uppercase(), res?.substringBefore(stringResource(R.string.format_separator).trim())?.trim(), size)
        .joinToString(stringResource(R.string.format_separator))
    val shape = RoundedCornerShape(14.dp)
    val open = state.techOpen
    Column(modifier.fillMaxWidth().padding(top = 18.dp).clip(shape).background(c.s1).border(BorderStroke(1.dp, c.frame), shape)) {
        Row(
            Modifier.fillMaxWidth().clickable(role = Role.Button, onClick = events::onToggleTech).heightIn(min = 64.dp).padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Box(Modifier.size(40.dp).clip(RoundedCornerShape(12.dp)).background(c.s2), contentAlignment = Alignment.Center) {
                SlouchcatIconImage(SlouchcatIcon.Info, contentDescription = null, size = 20.dp, tint = c.tx2)
            }
            Column(Modifier.weight(1f)) {
                Text(stringResource(R.string.app_item_tech), style = SlouchcatType.rowTitle, color = c.tx)
                Text(summary, style = SlouchcatType.caption, color = c.tx3)
            }
            SlouchcatIconImage(if (open) SlouchcatIcon.Up else SlouchcatIcon.Down, contentDescription = null, size = 20.dp, tint = c.tx2)
        }
        if (!open) return@Column
        Column(Modifier.fillMaxWidth().background(c.s1).padding(start = 16.dp, end = 16.dp, bottom = 10.dp)) {
            if (item.mediaFiles.size > 1) {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(vertical = 8.dp)) {
                    items(item.mediaFiles, key = { it.id }) { f ->
                        val index = item.mediaFiles.indexOf(f) + 1
                        SlouchcatChip(f.versionLabel ?: stringResource(R.string.app_item_version, index.toString()), selected = f.id == file.id, onClick = { events.onVersion(f.id) }, modifier = Modifier.heightIn(min = 48.dp))
                    }
                }
            }
            if (!file.available) Note(stringResource(R.string.item_file_unavailable), warning = true, modifier = Modifier.padding(vertical = 8.dp))
            val rows = listOfNotNull(
                file.videoCodec?.let { stringResource(R.string.item_file_video) to it.uppercase() },
                res?.let { stringResource(R.string.item_file_resolution) to it },
                file.audioCodec?.let { stringResource(R.string.app_item_techAudio) to it.uppercase() },
                stringResource(R.string.item_file_size) to size,
                stringResource(R.string.item_file_container) to file.container.uppercase(),
                stringResource(R.string.app_item_techVersions) to pluralStringResource(R.plurals.app_item_versionCount, item.mediaFiles.size, item.mediaFiles.size),
            )
            rows.forEachIndexed { i, (k, v) ->
                Row(Modifier.fillMaxWidth().padding(vertical = 10.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                    Text(k, style = SlouchcatType.bodySmall.copy(fontSize = 14.sp), color = c.tx3, modifier = Modifier.weight(1f))
                    Text(v, style = SlouchcatType.label.copy(fontSize = 14.sp), color = c.tx)
                }
                if (i < rows.lastIndex) Box(Modifier.fillMaxWidth().height(1.dp).background(c.line))
            }
        }
    }
}
