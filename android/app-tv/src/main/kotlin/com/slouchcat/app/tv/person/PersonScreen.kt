package com.slouchcat.app.tv.person

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.KindLine
import com.slouchcat.app.tv.browse.MediaCard
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.info
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.browse.screenWidth
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.BackdropMode
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvBackdrop
import com.slouchcat.app.tv.ui.TvBadge
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.PersonCreditsInner
import com.slouchcat.core.browse.person.PersonUiState
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

private const val ROW = "known"
private const val RETRY = "retry"

/**
 * A person (mockup `/person/:id`): the portrait on the left; «Person», the name, what they do, how
 * many titles of the library, and «Bekannt aus» as a row of the titles the profile sees (each
 * with the role as caption). The art layer shows the first title, darkened.
 */
@Composable
fun PersonScreen(state: PersonUiState, images: ServerImages, onItem: ((String) -> Unit)?, onRetry: () -> Unit) {
    val focus = rememberFocusMemory()
    val c = SlouchcatTheme.colors
    val page = (state.person as? Loadable.Ready)?.value
    val titles = page?.titles.orEmpty().map { MediaItem.from(it.item, images) to it.roles }
    Box(Modifier.fillMaxSize()) {
        TvBackdrop(titles.firstOrNull()?.first?.backdrop?.forWidth(screenWidth()), mode = BackdropMode.Dim)
        if (page == null) {
            LoadableContent(state.person, onRetry, Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.DetailTop), retryModifier = Modifier.focusKey(focus, RETRY)) {}
        } else {
            Row(
                Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.PersonTop, end = TvDimens.safeX),
                horizontalArrangement = Arrangement.spacedBy(TvDimens.Browse.PersonGap),
            ) {
                Box(Modifier.width(TvDimens.Browse.PersonImage).aspectRatio(PORTRAIT).clip(RoundedCornerShape(TvDimens.radiusPanel)).background(c.s2)) {
                    TvImage(images.single(page.person.image).forWidth(TvDimens.Browse.PersonImage), Modifier.fillMaxSize())
                }
                Column(Modifier.weight(1f)) {
                    KindLine(stringResource(R.string.tv_person_kind))
                    Text(page.person.name, style = TvType.xl, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.s).semantics { heading() })
                    val jobs = page.titles.flatMap { t -> t.roles.map { jobOf(it) } }.distinct().joinToString(" · ")
                    if (jobs.isNotEmpty()) Text(jobs, style = TvType.large, color = c.tx2, modifier = Modifier.padding(vertical = TvDimens.m))
                    TvBadge(pluralCount(R.plurals.app_person_inLibrary, page.titles.size))
                    Text(stringResource(R.string.app_person_knownFrom), style = TvType.h2, modifier = Modifier.padding(top = TvDimens.xl).semantics { heading() })
                    CompositionLocalProvider(LocalContentStart provides 0.dp) {
                        val roles = titles.associate { (item, credits) -> item.id to roleLine(credits) }
                        val infos = titles.associate { (item, _) -> item.id to item.info().let { i -> roles[item.id]?.let { r -> i.copy(meta = listOfNotNull(r, i.meta).joinToString(" · ")) } ?: i } }
                        TvCardRow(
                            title = null,
                            items = titles.map { it.first },
                            key = { it.id },
                            focus = focus,
                            rowKey = ROW,
                            info = { infos[it.id] },
                        ) { item, modifier ->
                            MediaCard(item, onItem?.let { open -> { open(item.id) } }, modifier, subcaption = roles[item.id])
                        }
                    }
                }
            }
        }
    }
    focus.StartFocus(if (page == null) RETRY else titles.firstOrNull()?.let { "$ROW-${it.first.id}" } ?: RETRY, state.person::class)
}

/** «Schauspiel», «Regie» … of a credit. */
@Composable
private fun jobOf(credit: PersonCreditsInner): String = stringResource(
    when (credit.role) {
        PersonCreditsInner.Role.DIRECTOR -> R.string.item_cast_role_director
        PersonCreditsInner.Role.WRITER -> R.string.item_cast_role_writer
        PersonCreditsInner.Role.CREATOR -> R.string.item_cast_role_creator
        else -> R.string.person_department_acting
    },
)

/** «als Peter Parker, Regie»: the roles of one title. */
@Composable
private fun roleLine(credits: List<PersonCreditsInner>): String? = credits.map { credit ->
    val character = credit.character
    if (credit.role == PersonCreditsInner.Role.CAST && !character.isNullOrBlank()) stringResource(R.string.person_as, character) else jobOf(credit)
}.distinct().joinToString(", ").ifEmpty { null }

private const val PORTRAIT = 2f / 3f
