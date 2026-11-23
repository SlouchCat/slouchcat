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
import com.slouchcat.core.browse.wishes.WishText
import com.slouchcat.core.browse.wishes.cardMarkText
import com.slouchcat.core.browse.wishes.failRes
import com.slouchcat.core.browse.wishes.mediaKindRes
import com.slouchcat.core.browse.wishes.progressTexts
import com.slouchcat.core.browse.wishes.quotaText
import com.slouchcat.core.browse.wishes.reasonRes
import com.slouchcat.core.browse.wishes.wishDetailRes
import com.slouchcat.core.browse.wishes.wishStateRes
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.strings.R

/*
 * The words about a wish on the TV: the shared keys of `WishWords` (`:core:browse`, the same as the
 * phone app's) drawn with stringResource; nothing of their own.
 */

@Composable
fun WishText.text(): String = stringResource(id, *args.toTypedArray())

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

@Composable
fun mediaKind(media: ExternalMedia): String = stringResource(mediaKindRes(media))

/** «2024 · Film»; the kind alone without a year. */
@Composable
fun externalCaption(year: Int?, media: ExternalMedia): String =
    if (year != null) stringResource(R.string.app_wishes_caption, year.toString(), mediaKind(media)) else mediaKind(media)

@Composable
fun cardMark(card: ExternalCard): String? = cardMarkText(card)?.text()

@Composable
fun quotaLine(media: ExternalMedia, quota: WishQuota): String = quotaText(media, quota).text()

@Composable
fun progressText(progress: WishProgress): String? =
    progressTexts(progress).map { it.text() }.takeIf { it.isNotEmpty() }?.joinToString(stringResource(R.string.format_separator))
