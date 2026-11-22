package com.slouchcat.app.mobile.tvlink

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextReplacement
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.device
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.SessionKind
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** A scanner without camera: the test decides what it reads. */
private class TestScanner(override val asksForCamera: Boolean = false) : CodeScanner {
    @Composable
    override fun Camera(onCode: (String) -> Unit, onClose: () -> Unit, onUnavailable: () -> Unit, modifier: Modifier) {
        Box(modifier)
    }
}

/** «Fernseher verbinden» (rule 8): every step at two widths, the large font, every control labelled, back. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class TvLinkScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val events = mutableListOf<String>()
    private val tv = device("tv", "Living room", type = MyDevice.Type.TV, kind = SessionKind.DEVICE)

    private val actions = TvLinkActions(
        onBack = { events += "back" },
        onScan = { events += "scan" },
        onScanned = { events += "scanned" },
        onScanClosed = { events += "closed" },
        onScannerUnavailable = { events += "unavailable" },
        onConfirm = { events += "confirm" },
        onBackToIntro = { events += "intro" },
        onSwitchProfile = { events += "profiles" },
        onDevices = { events += "devices" },
        onFinish = { events += "finish" },
        onEditRelay = { events += "relay:$it" },
        onRelay = { events += "relay=${it ?: "default"}" },
    )

    private fun show(state: TvLinkUiState, scanner: CodeScanner = TestScanner()) = compose.setContent {
        SlouchcatTheme { TvLinkScreen(state, scanner, actions) }
    }

    private fun systemBack() = compose.runOnUiThread { compose.activity.onBackPressedDispatcher.onBackPressed() }

    private fun button(text: String) = compose.onNode(hasText(text) and hasClickAction())

    // --- intro ---

    private fun intro() {
        show(TvLinkUiState(primary = true, tvs = listOf(tv)))
        compose.onNodeWithText(s(R.string.app_tv_intro_title)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_tv_intro_step3)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_tv_intro_googleHint)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText("Living room").performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        button(s(R.string.app_tv_intro_scan)).performScrollTo().performClick()
        compose.onNodeWithText("Living room").performScrollTo().performClick()
        button(s(R.string.app_tv_relay_change)).performScrollTo().performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("scan", "devices", "relay:true", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun introPhone() = intro()

    @Test @Config(qualifiers = TABLET)
    fun introTablet() = intro()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun introPhoneLargeFont() = intro()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun introTabletLargeFont() = intro()

    private fun notPrimary() {
        show(TvLinkUiState(primary = false, scanProblem = ScanProblem.OtherRelay("pair.example.test")))
        compose.onNodeWithText(s(R.string.app_tv_intro_primaryOnly)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_tv_problem_otherRelay, "pair.example.test")).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_tv_intro_scan)).assertDoesNotExist()
        compose.assertEveryControlLabelledAndTappable()
        button(s(R.string.app_more_switchProfile)).performScrollTo().performClick()
        assertEquals(listOf("profiles"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun notPrimaryPhone() = notPrimary()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun notPrimaryTabletLargeFont() = notPrimary()

    @Test @Config(qualifiers = PHONE)
    fun theOwnScannerSaysItAsksForTheCamera() {
        show(TvLinkUiState(primary = true, scanProblem = ScanProblem.Unavailable), TestScanner(asksForCamera = true))
        compose.onNodeWithText(s(R.string.app_tv_intro_cameraHint)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_tv_problem_unavailable)).performScrollTo().assertIsDisplayed()
    }

    // --- scanner ---

    private fun scan() {
        show(TvLinkUiState(step = TvLinkStep.Scan, primary = true))
        compose.onNodeWithText(s(R.string.app_tv_scan_title)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_tv_scan_aim)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithContentDescription(s(R.string.app_tv_scan_close)).performClick()
        systemBack()
        assertEquals(listOf("closed", "closed"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun scanPhone() = scan()

    @Test @Config(qualifiers = TABLET)
    fun scanTablet() = scan()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun scanPhoneLargeFont() = scan()

    // --- the question ---

    private val confirm = TvLinkStep.Confirm("viewer@example.test", "http://192.0.2.10:8096", "Mara", primary = true, plainHttp = true)

    private fun question() {
        show(TvLinkUiState(step = confirm, primary = true))
        compose.onNodeWithText(s(R.string.app_tv_confirm_title)).assertIsDisplayed()
        compose.onNodeWithText("viewer@example.test").performScrollTo().assertIsDisplayed()
        compose.onNodeWithText("http://192.0.2.10:8096").performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_tv_confirm_primary, "Mara")).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_tv_confirm_plainHttp)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        button(s(R.string.app_tv_confirm_go)).performScrollTo().performClick()
        button(s(R.string.common_cancel)).performScrollTo().performClick()
        systemBack()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("confirm", "intro", "intro", "intro"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun questionPhone() = question()

    @Test @Config(qualifiers = TABLET)
    fun questionTablet() = question()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun questionPhoneLargeFont() = question()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun questionTabletLargeFont() = question()

    @Test @Config(qualifiers = PHONE)
    fun overHttpsThereIsNoWarning() {
        show(TvLinkUiState(step = confirm.copy(server = "https://media.example.test", plainHttp = false, primary = false), primary = false))
        compose.onNodeWithText(s(R.string.app_tv_confirm_plainHttp)).assertDoesNotExist()
        compose.onNodeWithText(s(R.string.app_tv_confirm_profile, "Mara")).performScrollTo().assertIsDisplayed()
    }

    // --- the run ---

    private fun running() {
        show(TvLinkUiState(step = TvLinkStep.Running(1), primary = true))
        compose.onNodeWithText(s(R.string.app_tv_run_title)).assertIsDisplayed()
        compose.onNodeWithContentDescription("${s(R.string.app_tv_run_voucher)}: ${s(R.string.app_tv_run_done)}").assertIsDisplayed()
        compose.onNodeWithContentDescription("${s(R.string.app_tv_run_handedOver)}: ${s(R.string.app_tv_run_busy)}").assertIsDisplayed()
        compose.onNodeWithContentDescription("${s(R.string.app_tv_run_waiting)}: ${s(R.string.app_tv_run_pending)}").assertIsDisplayed()
        // No arrow during the run (mockup); the system's back leaves the page through the NavHost.
        compose.onNodeWithContentDescription(s(R.string.common_back)).assertDoesNotExist()
    }

    @Test @Config(qualifiers = PHONE)
    fun runningPhone() = running()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun runningTabletLargeFont() = running()

    // --- failures ---

    private fun failure(failure: TvLinkFailure, at: Int, text: String, buttons: List<String>, expected: List<String>) {
        show(TvLinkUiState(step = TvLinkStep.Failed(failure, at), primary = true))
        compose.onNodeWithText(s(R.string.app_tv_run_failedTitle)).assertIsDisplayed()
        compose.onNodeWithContentDescription("${RUN[at]}: ${s(R.string.app_tv_run_failed)}").performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(text).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        for (b in buttons) button(b).performScrollTo().performClick()
        assertEquals(expected, events)
    }

    private val RUN get() = listOf(s(R.string.app_tv_run_voucher), s(R.string.app_tv_run_handedOver), s(R.string.app_tv_run_waiting))

    private fun occupied() = failure(TvLinkFailure.Occupied, 1, s(R.string.app_tv_fail_occupied), listOf(s(R.string.app_tv_fail_ok)), listOf("intro"))

    @Test @Config(qualifiers = PHONE)
    fun occupiedPhone() {
        occupied()
        compose.onNodeWithText(s(R.string.app_tv_fail_lapses)).performScrollTo().assertIsDisplayed()
    }

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun occupiedTabletLargeFont() = occupied()

    @Test @Config(qualifiers = PHONE)
    fun timedOutPhone() = failure(
        TvLinkFailure.TimedOut, 2, s(R.string.app_tv_fail_timedOut),
        listOf(s(R.string.app_tv_fail_again), s(R.string.common_cancel)), listOf("scan", "intro"),
    )

    @Test @Config(qualifiers = TABLET)
    fun expiredTablet() = failure(TvLinkFailure.Expired, 1, s(R.string.app_tv_fail_expired), listOf(s(R.string.app_tv_fail_again)), listOf("scan"))

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun relayDownLargeFont() = failure(TvLinkFailure.RelayUnreachable, 1, s(R.string.app_tv_fail_relay), listOf(s(R.string.app_tv_fail_ok)), listOf("intro"))

    @Test @Config(qualifiers = PHONE)
    fun serverDown() = failure(TvLinkFailure.ServerOffline, 0, s(R.string.app_tv_fail_server), listOf(s(R.string.app_tv_fail_again)), listOf("scan"))

    @Test @Config(qualifiers = PHONE)
    fun onlyThePrimaryProfile() = failure(TvLinkFailure.PrimaryOnly, 0, s(R.string.app_tv_intro_primaryOnly), listOf(s(R.string.app_tv_fail_ok)), listOf("intro"))

    @Test @Config(qualifiers = PHONE)
    fun notAllowed() = failure(TvLinkFailure.NotAllowed, 0, s(R.string.app_tv_fail_notAllowed), listOf(s(R.string.app_tv_fail_ok)), listOf("intro"))

    @Test @Config(qualifiers = PHONE)
    fun tooManyAttempts() = failure(TvLinkFailure.TooManyAttempts(30), 0, s(R.string.errors_code_too_many_attempts), listOf(s(R.string.app_tv_fail_again)), listOf("scan"))

    @Test @Config(qualifiers = PHONE)
    fun serverUnconfirmed() = failure(TvLinkFailure.ServerUnconfirmed, 0, s(R.string.app_serverKey_unconfirmedBody), listOf(s(R.string.app_tv_fail_ok)), listOf("intro"))

    @Test @Config(qualifiers = TABLET)
    fun leftAfterThePacket() = failure(
        TvLinkFailure.Left(delivered = true), 2, s(R.string.app_tv_fail_leftAfter),
        listOf(s(R.string.nav_devices), s(R.string.app_tv_fail_ok)), listOf("devices", "intro"),
    )

    @Test @Config(qualifiers = PHONE)
    fun leftBeforeThePacket() = failure(TvLinkFailure.Left(delivered = false), 0, s(R.string.app_tv_fail_leftBefore), listOf(s(R.string.app_tv_fail_again)), listOf("scan"))

    @Test @Config(qualifiers = PHONE)
    fun somethingElse() = failure(TvLinkFailure.Failed, 1, s(R.string.app_state_failed), listOf(s(R.string.app_tv_fail_ok)), listOf("intro"))

    @Test @Config(qualifiers = PHONE)
    fun backOnAFailureIsTheStart() {
        show(TvLinkUiState(step = TvLinkStep.Failed(TvLinkFailure.Occupied, 1), primary = true))
        systemBack()
        assertEquals(listOf("intro"), events)
    }

    // --- connected ---

    private fun done() {
        show(TvLinkUiState(step = TvLinkStep.Done("Living room"), primary = true))
        compose.onNodeWithText(s(R.string.app_tv_done_title)).assertIsDisplayed()
        compose.onNodeWithText("Living room").assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        button(s(R.string.nav_devices)).performScrollTo().performClick()
        button(s(R.string.app_tv_done_finish)).performScrollTo().performClick()
        systemBack()
        assertEquals(listOf("devices", "finish", "finish"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun donePhone() = done()

    @Test @Config(qualifiers = TABLET)
    fun doneTablet() = done()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun donePhoneLargeFont() = done()

    @Test @Config(qualifiers = PHONE)
    fun aTvWithoutNameIsUnnamed() {
        show(TvLinkUiState(step = TvLinkStep.Done(null), primary = true))
        compose.onNodeWithText(s(R.string.settings_devices_unnamed)).assertIsDisplayed()
    }

    // --- the relay ---

    private fun relaySheet() {
        show(TvLinkUiState(primary = true, editingRelay = true))
        compose.onNodeWithText(s(R.string.tv_relay_lead)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        val field = compose.onNode(androidx.compose.ui.test.hasSetTextAction())
        // Plain HTTP only in a debug build: the unit tests run the debug build, so a typo is the refusal here.
        field.performTextReplacement("not an address")
        button(s(R.string.tv_relay_save)).performScrollTo().performClick()
        compose.onNodeWithText(s(R.string.tv_relay_invalid)).performScrollTo().assertIsDisplayed()
        field.performTextReplacement("pair.example.test")
        button(s(R.string.tv_relay_save)).performScrollTo().performClick()
        button(s(R.string.tv_relay_reset)).performScrollTo().performClick()
        assertEquals(listOf("relay=${"https://pair.example.test/".toHttpUrl()}", "relay=default"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun relaySheetPhone() = relaySheet()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun relaySheetTabletLargeFont() = relaySheet()
}
