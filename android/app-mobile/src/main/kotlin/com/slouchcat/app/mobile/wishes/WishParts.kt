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
    val text = stringResource(
        when (state) {
            WishState.QUEUED -> R.string.wishes_state_queued
            WishState.PENDING -> R.string.wishes_state_pending
            WishState.PROCESSING -> R.string.wishes_state_processing
            WishState.PARTIAL -> R.string.wishes_state_partial
            WishState.AVAILABLE -> R.string.wishes_state_available
            WishState.DECLINED -> R.string.wishes_state_declined
            WishState.FAILED -> R.string.wishes_state_failed
            WishState.WITHDRAWN, WishState.UNKNOWN_DEFAULT_OPEN_API -> R.string.wishes_state_withdrawn
        },
    )
    val more = when (detail) {
        WishStateDetail.IMPORTING -> stringResource(R.string.wishes_detail_importing)
        WishStateDetail.NOT_FOUND_IN_LIBRARY -> stringResource(R.string.wishes_detail_not_found_in_library)
        else -> null
    }
    return if (more != null) text + stringResource(R.string.format_separator) + more else text
}

@Composable
fun failText(reason: WishFailReason): String? = when (reason) {
    WishFailReason.QUOTA -> R.string.wishes_fail_quota
    WishFailReason.BLOCKLISTED -> R.string.wishes_fail_blocklisted
    WishFailReason.NO_PERMISSION -> R.string.wishes_fail_no_permission
    WishFailReason.ARR_FAILED -> R.string.wishes_fail_arr_failed
    WishFailReason.UNREACHABLE_24H -> R.string.wishes_fail_unreachable_24h
    WishFailReason.SEERR_REQUEST_GONE -> R.string.wishes_fail_seerr_request_gone
    WishFailReason.SEERR_MEDIA_DELETED -> R.string.wishes_fail_seerr_media_deleted
    WishFailReason.SEERR_USER_GONE -> R.string.wishes_fail_seerr_user_gone
    WishFailReason.TVDB_ID_MISSING -> R.string.wishes_fail_tvdb_id_missing
    WishFailReason.ACCOUNT_DELETED -> R.string.wishes_fail_account_deleted
    else -> null
}?.let { stringResource(it) }

@Composable
fun reasonText(reason: WishRequestReason?): String = stringResource(
    when (reason) {
        WishRequestReason.DISABLED -> R.string.wishes_reason_disabled
        WishRequestReason.ACCOUNT_NOT_ALLOWED -> R.string.wishes_reason_account_not_allowed
        WishRequestReason.PROFILE_NOT_ALLOWED -> R.string.wishes_reason_profile_not_allowed
        WishRequestReason.NO_MAPPING -> R.string.wishes_reason_no_mapping
        WishRequestReason.SEERR_USER_GONE -> R.string.wishes_reason_seerr_user_gone
        WishRequestReason.MOVIES_NOT_ALLOWED -> R.string.wishes_reason_movies_not_allowed
        WishRequestReason.SHOWS_NOT_ALLOWED -> R.string.wishes_reason_shows_not_allowed
        WishRequestReason.STATUS_ONLY -> R.string.wishes_reason_status_only
        WishRequestReason.ALREADY_REQUESTED -> R.string.wishes_reason_already_requested
        else -> R.string.titles_cannotRequest
    },
)

/** A refused change of the wishes in the viewer's words (web `wishProblemMessage`). */
@Composable
fun wishErrorText(code: String?): String = stringResource(
    when (code) {
        "tmdb_unreachable" -> R.string.wishes_tmdbUnreachable
        "seerr_unreachable" -> R.string.errors_code_seerr_unreachable
        "seerr_failed" -> R.string.errors_code_seerr_failed
        "wish_not_allowed" -> R.string.errors_code_wish_not_allowed
        "quota_exceeded" -> R.string.errors_code_quota_exceeded
        "already_available" -> R.string.errors_code_already_available
        "wish_not_withdrawable" -> R.string.errors_code_wish_not_withdrawable
        "not_found" -> R.string.app_state_notFound
        "offline" -> R.string.app_state_offline
        else -> R.string.app_wishes_failed
    },
)

@Composable
fun mediaKind(media: ExternalMedia): String = stringResource(if (media == ExternalMedia.SHOW) R.string.wishes_kind_show else R.string.wishes_kind_movie)

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
fun cardMark(card: ExternalCard): String? = when {
    card.itemId != null -> stringResource(R.string.wishes_discover_inLibrary)
    card.wishState != null -> wishStateText(card.wishState!!)
    card.requested -> stringResource(R.string.wishes_discover_requested)
    else -> null
}

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
