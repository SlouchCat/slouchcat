package com.slouchcat.app.mobile.wishes

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishFailReason
import com.slouchcat.core.api.generated.models.WishRequestReason
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.api.generated.models.WishStateDetail
import com.slouchcat.core.browse.wishes.WishText
import com.slouchcat.core.browse.wishes.cardMarkText
import com.slouchcat.core.browse.wishes.failRes
import com.slouchcat.core.browse.wishes.mediaKindRes
import com.slouchcat.core.browse.wishes.reasonRes
import com.slouchcat.core.browse.wishes.wishDetailRes
import com.slouchcat.core.browse.wishes.wishErrorRes
import com.slouchcat.core.browse.wishes.wishStateRes
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.strings.R

/*
 * What every page of the wishes says about a wish (web `wishes/state.tsx`): the state in words,
 * a failure, a reason, a refused request, the line under a title, and the card of a title outside
 * the library. Only the server's state is put into words (concept 25, 5.3).
 */

@Composable
fun wishStateText(state: WishState, detail: WishStateDetail? = null): String {
    val text = stringResource(wishStateRes(state))
    val more = wishDetailRes(detail)?.let { stringResource(it) }
    return if (more != null) text + stringResource(R.string.format_separator) + more else text
}

@Composable
fun failText(reason: WishFailReason): String? = failRes(reason)?.let { stringResource(it) }

@Composable
fun reasonText(reason: WishRequestReason?): String = stringResource(reasonRes(reason))

/** A refused change of the wishes in the viewer's words (web `wishProblemMessage`). */
@Composable
fun wishErrorText(code: String?): String = stringResource(wishErrorRes(code))

@Composable
fun mediaKind(media: ExternalMedia): String = stringResource(mediaKindRes(media))

/** A text of the shared words about wishes ([WishText]). */
@Composable
fun WishText.text(): String = stringResource(id, *args.toTypedArray())

/** «2024 · Film»; the kind alone without a year. */
@Composable
fun externalCaption(year: Int?, media: ExternalMedia): String =
    if (year != null) stringResource(R.string.app_wishes_caption, year.toString(), mediaKind(media)) else mediaKind(media)

/**
 * A title outside the library as a poster card (mockup `/wishes` rows): poster, title, «2024 ·
 * Film» and, below, «In der Bibliothek», the own wish's state or «Bereits angefordert». The whole
 * card is one control for the screen reader.
 */
@Composable
fun ExternalPosterCard(card: ExternalCard, onClick: (() -> Unit)?, modifier: Modifier = Modifier, width: Dp? = 124.dp) {
    val c = SlouchcatTheme.colors
    val caption = externalCaption(card.year, card.media)
    val mark = cardMark(card)
    val label = listOfNotNull(card.title, caption, mark).joinToString(", ")
    Column(
        modifier
            .then(if (width != null) Modifier.width(width) else Modifier.fillMaxWidth())
            .clip(RoundedCornerShape(10.dp))
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .clearAndSetSemantics {
                contentDescription = label
                if (onClick != null) role = Role.Button
            },
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        MediaImage(card.posters, card.title, Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(10.dp)))
        Text(card.title, style = SlouchcatType.label, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
        Text(caption, style = SlouchcatType.caption, color = c.tx2, maxLines = 1)
        if (mark != null) StateTag(mark)
    }
}

/** What a tile says next to its title: in the library, the own wish, requested by someone. */
@Composable
fun cardMark(card: ExternalCard): String? = cardMarkText(card)?.text()

/** `.chip` with a frame: a state, «Nicht in der Bibliothek». */
@Composable
fun StateTag(text: String, modifier: Modifier = Modifier, strong: Boolean = false) {
    val c = SlouchcatTheme.colors
    Box(
        modifier
            .heightIn(min = 24.dp)
            .clip(RoundedCornerShape(6.dp))
            .background(if (strong) c.s3 else c.s1)
            .border(1.dp, c.frame, RoundedCornerShape(6.dp))
            .padding(horizontal = 8.dp, vertical = 2.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        Text(text, style = SlouchcatType.label, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
    }
}

/** A row of a list of titles outside the library (the search's «Noch nicht in der Bibliothek»). */
@Composable
fun ExternalRow(card: ExternalCard, onClick: (() -> Unit)?, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val caption = externalCaption(card.year, card.media)
    val mark = cardMark(card)
    Row(
        modifier
            .fillMaxWidth()
            .heightIn(min = 72.dp)
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .semantics(mergeDescendants = true) {}
            .padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        MediaImage(card.posters, card.title, Modifier.width(48.dp).aspectRatio(2f / 3f).clip(RoundedCornerShape(6.dp)))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(card.title, style = SlouchcatType.body, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Text(listOfNotNull(caption, mark).joinToString(stringResource(R.string.format_separator)), style = SlouchcatType.label, color = c.tx2)
        }
    }
}
