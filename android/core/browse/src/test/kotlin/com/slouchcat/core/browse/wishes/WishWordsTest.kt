package com.slouchcat.core.browse.wishes

import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishFailReason
import com.slouchcat.core.api.generated.models.WishProgress
import com.slouchcat.core.api.generated.models.WishQuota
import com.slouchcat.core.api.generated.models.WishRequestReason
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.browse.testing.AT
import com.slouchcat.core.browse.testing.externalRef
import com.slouchcat.core.browse.testing.quota
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Test

/** The words about wishes both apps share (code review m-1 of K26 stage 5): every value has its key. */
class WishWordsTest {
    @Test
    fun everyStateReasonAndFailureHasAText() {
        // Every state of the contract: a key of its own, the unknown one the text of «withdrawn».
        val states = WishState.entries.filter { it != WishState.UNKNOWN_DEFAULT_OPEN_API }.map(::wishStateRes)
        assertEquals(states.size, states.toSet().size)
        assertEquals(R.string.wishes_state_withdrawn, wishStateRes(WishState.UNKNOWN_DEFAULT_OPEN_API))
        // Every known reason of a request: its own text; none falls back to the general one.
        WishRequestReason.entries.filter { it != WishRequestReason.UNKNOWN_DEFAULT_OPEN_API }
            .forEach { assertNotEquals(it.value, R.string.titles_cannotRequest, reasonRes(it)) }
        assertEquals(R.string.titles_cannotRequest, reasonRes(null))
        WishFailReason.entries.filter { it != WishFailReason.UNKNOWN_DEFAULT_OPEN_API }.forEach { assertNotEquals(it.value, null, failRes(it)) }
    }

    @Test
    fun problemCodesMapAndUnknownOnesAreGeneral() {
        assertEquals(R.string.errors_code_quota_exceeded, wishErrorRes("quota_exceeded"))
        assertEquals(R.string.errors_code_wish_not_withdrawable, wishErrorRes("wish_not_withdrawable"))
        assertEquals(R.string.app_state_notAllowed, wishErrorRes("client_not_allowed"))
        assertEquals(R.string.app_state_offline, wishErrorRes(WishTitleViewModel.OFFLINE))
        assertEquals(R.string.app_wishes_failed, wishErrorRes("something_new"))
        assertEquals(R.string.app_wishes_failed, wishErrorRes(null))
    }

    @Test
    fun theLimitTheProgressAndTheMarkOfATile() {
        assertEquals(WishText(R.string.wishes_quota_movie, listOf("1", "5", "7")), quotaText(ExternalMedia.MOVIE, quota()))
        assertEquals(WishText(R.string.wishes_quota_tv, listOf("1", "5", "7")), quotaText(ExternalMedia.SHOW, quota()))
        assertEquals(WishText(R.string.wishes_quota_tvNone), quotaText(ExternalMedia.SHOW, WishQuota(unlimited = true)))
        assertEquals(
            listOf(WishText(R.string.wishes_progress_download), WishText(R.string.wishes_progress_percent, listOf("64"))),
            progressTexts(WishProgress(WishProgress.Stage.DOWNLOAD, AT, percent = 64)),
        )
        val images = ServerImages("https://example.test/".toHttpUrl())
        assertNull(cardMarkText(ExternalCard.from(externalRef(1), images)))
        assertEquals(WishText(R.string.wishes_discover_inLibrary), cardMarkText(ExternalCard.from(externalRef(1), images, itemId = "x")))
        assertEquals(WishText(R.string.wishes_state_pending), cardMarkText(ExternalCard.from(externalRef(1, wish = WishState.PENDING), images)))
        assertEquals(WishText(R.string.wishes_discover_requested), cardMarkText(ExternalCard.from(externalRef(1), images, requested = true)))
    }
}
