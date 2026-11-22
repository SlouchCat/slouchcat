package com.slouchcat.app.tv.wishes

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishFailReason
import com.slouchcat.core.api.generated.models.WishProgress
import com.slouchcat.core.api.generated.models.WishQuota
import com.slouchcat.core.api.generated.models.WishRequestReason
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.api.generated.models.WishStateDetail
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.strings.R

/*
 * What the pages of «Wünsche» on the TV say about a wish (web `wishes/state.tsx`, the same words as
 * the phone app's `WishParts`): the state, a failure, why a profile cannot request, a refused
 * change, the kind, the limit. Only the server's state is put into words (concept 25, 5.3; rule 12).
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

/** Why this profile cannot request the title (the server's reason). */
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

/** A refused change of the wishes in the viewer's words (web `wishProblemMessage`); [code] a problem code. */
fun wishErrorId(code: String?): Int = when (code) {
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
}

@Composable
fun mediaKind(media: ExternalMedia): String = stringResource(if (media == ExternalMedia.SHOW) R.string.wishes_kind_show else R.string.wishes_kind_movie)

/** «2024 · Film»; the kind alone without a year. */
@Composable
fun externalCaption(year: Int?, media: ExternalMedia): String =
    if (year != null) stringResource(R.string.app_wishes_caption, year.toString(), mediaKind(media)) else mediaKind(media)

/** What a tile says next to its title: in the library, the own wish, requested by someone, else «Noch nicht in der Bibliothek». */
@Composable
fun cardMark(card: ExternalCard): String? {
    val state = card.wishState
    return when {
        card.itemId != null -> stringResource(R.string.wishes_discover_inLibrary)
        state != null -> wishStateText(state)
        card.requested -> stringResource(R.string.wishes_discover_requested)
        else -> null
    }
}

/** «Filme: 1 von 5 in 7 Tagen» (web `quotaText`); without a limit «Filme ohne Limit». */
@Composable
fun quotaLine(media: ExternalMedia, quota: WishQuota): String {
    val part = if (media == ExternalMedia.SHOW) quota.tv else quota.movie
    return when {
        quota.unlimited || part == null -> stringResource(if (media == ExternalMedia.SHOW) R.string.wishes_quota_tvNone else R.string.wishes_quota_movieNone)
        media == ExternalMedia.SHOW -> stringResource(R.string.wishes_quota_tv, part.used.toString(), part.limit.toString(), part.days.toString())
        else -> stringResource(R.string.wishes_quota_movie, part.used.toString(), part.limit.toString(), part.days.toString())
    }
}

/** How far Seerr is with a wish on its way: searching, a download with its share, importing, stalled. */
@Composable
fun progressText(progress: WishProgress): String? = when (progress.stage) {
    WishProgress.Stage.SEARCH -> stringResource(R.string.wishes_progress_search)
    WishProgress.Stage.DOWNLOAD -> listOfNotNull(
        stringResource(R.string.wishes_progress_download),
        progress.percent?.let { stringResource(R.string.wishes_progress_percent, it.toString()) },
    ).joinToString(stringResource(R.string.format_separator))
    WishProgress.Stage.IMPORT -> stringResource(R.string.wishes_progress_import)
    WishProgress.Stage.STALLED -> stringResource(R.string.wishes_progress_stalled)
    else -> null
}
