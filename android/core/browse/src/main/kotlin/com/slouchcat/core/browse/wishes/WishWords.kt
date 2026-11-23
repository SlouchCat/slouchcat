package com.slouchcat.core.browse.wishes

import androidx.annotation.StringRes
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
 * What both apps say about a wish (web `wishes/state.tsx`): the state, a failure, why a profile
 * cannot request, a refused change, the kind, the limit, the progress — as keys of `core.strings`
 * with their arguments, no Compose (code review m-1 of K26 stage 5). The apps only call
 * `stringResource(text.id, *text.args)`. Only the server's state is put into words (rule 12).
 */

/** A text of `core.strings` with its arguments (all already as text). */
data class WishText(@StringRes val id: Int, val args: List<String> = emptyList())

@StringRes
fun wishStateRes(state: WishState): Int = when (state) {
    WishState.QUEUED -> R.string.wishes_state_queued
    WishState.PENDING -> R.string.wishes_state_pending
    WishState.PROCESSING -> R.string.wishes_state_processing
    WishState.PARTIAL -> R.string.wishes_state_partial
    WishState.AVAILABLE -> R.string.wishes_state_available
    WishState.DECLINED -> R.string.wishes_state_declined
    WishState.FAILED -> R.string.wishes_state_failed
    WishState.WITHDRAWN, WishState.UNKNOWN_DEFAULT_OPEN_API -> R.string.wishes_state_withdrawn
}

/** The addition to a state («wird eingelesen»); `null` without one. */
@StringRes
fun wishDetailRes(detail: WishStateDetail?): Int? = when (detail) {
    WishStateDetail.IMPORTING -> R.string.wishes_detail_importing
    WishStateDetail.NOT_FOUND_IN_LIBRARY -> R.string.wishes_detail_not_found_in_library
    else -> null
}

/** Why Seerr did not take a wish; `null` for a reason without a text of its own. */
@StringRes
fun failRes(reason: WishFailReason): Int? = when (reason) {
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
}

/** Why this profile cannot request the title (the server's reason). */
@StringRes
fun reasonRes(reason: WishRequestReason?): Int = when (reason) {
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
}

/**
 * A refused change of the wishes in the viewer's words (web `wishProblemMessage`); [code] a problem
 * code or one of the page's own ([WishTitleViewModel.OFFLINE] …). A session that may not (a device,
 * not the main profile) gets the text of «nicht erlaubt» (code review L-6).
 */
@StringRes
fun wishErrorRes(code: String?): Int = when (code) {
    "tmdb_unreachable" -> R.string.wishes_tmdbUnreachable
    "seerr_unreachable" -> R.string.errors_code_seerr_unreachable
    "seerr_failed" -> R.string.errors_code_seerr_failed
    "wish_not_allowed" -> R.string.errors_code_wish_not_allowed
    "quota_exceeded" -> R.string.errors_code_quota_exceeded
    "already_available" -> R.string.errors_code_already_available
    "wish_not_withdrawable" -> R.string.errors_code_wish_not_withdrawable
    "client_not_allowed", "primary_profile_required" -> R.string.app_state_notAllowed
    WishTitleViewModel.NOT_FOUND -> R.string.app_state_notFound
    WishTitleViewModel.OFFLINE -> R.string.app_state_offline
    else -> R.string.app_wishes_failed
}

@StringRes
fun mediaKindRes(media: ExternalMedia): Int = if (media == ExternalMedia.SHOW) R.string.wishes_kind_show else R.string.wishes_kind_movie

/** What a tile says next to its title: in the library, the own wish, requested by someone; `null` for none of these. */
fun cardMarkText(card: ExternalCard): WishText? {
    val state = card.wishState
    return when {
        card.itemId != null -> WishText(R.string.wishes_discover_inLibrary)
        state != null -> WishText(wishStateRes(state))
        card.requested -> WishText(R.string.wishes_discover_requested)
        else -> null
    }
}

/** «Filme: 1 von 5 in 7 Tagen» (web `quotaText`); without a limit «Filme ohne Limit». */
fun quotaText(media: ExternalMedia, quota: WishQuota): WishText {
    val part = if (media == ExternalMedia.SHOW) quota.tv else quota.movie
    return when {
        quota.unlimited || part == null -> WishText(if (media == ExternalMedia.SHOW) R.string.wishes_quota_tvNone else R.string.wishes_quota_movieNone)
        else -> WishText(
            if (media == ExternalMedia.SHOW) R.string.wishes_quota_tv else R.string.wishes_quota_movie,
            listOf(part.used.toString(), part.limit.toString(), part.days.toString()),
        )
    }
}

/** How far Seerr is with a wish on its way, in parts the app joins: searching, a download with its share, importing, stalled. */
fun progressTexts(progress: WishProgress): List<WishText> = when (progress.stage) {
    WishProgress.Stage.SEARCH -> listOf(WishText(R.string.wishes_progress_search))
    WishProgress.Stage.DOWNLOAD -> listOfNotNull(
        WishText(R.string.wishes_progress_download),
        progress.percent?.let { WishText(R.string.wishes_progress_percent, listOf(it.toString())) },
    )
    WishProgress.Stage.IMPORT -> listOf(WishText(R.string.wishes_progress_import))
    WishProgress.Stage.STALLED -> listOf(WishText(R.string.wishes_progress_stalled))
    else -> emptyList()
}
