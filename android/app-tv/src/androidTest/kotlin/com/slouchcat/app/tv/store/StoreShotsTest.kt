package com.slouchcat.app.tv.store

import android.graphics.Bitmap
import android.os.SystemClock
import android.view.KeyEvent
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isFocused
import androidx.compose.ui.test.junit4.ComposeTestRule
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.tv.MainActivity
import com.slouchcat.app.tv.ui.QrContent
import com.slouchcat.core.session.DataStoreRelaySettings
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.io.InputStreamReader

/**
 * The photos of the store listing for the TV (docs/store/grafiken.md) against the demo server of
 * scripts/store-screenshots.sh: 04 the setup with the QR code (the script stops the local relay
 * afterwards, so the code on the photo leads nowhere), then set up by hand with the remote, the
 * primary profile with its PIN, 01 the start page with the focus on a film, 02 its detail page,
 * 03 the player with the sound menu open. Only scripts/store-screenshots.sh runs it (`-e class`,
 * with the argument `slouchcatStore`); the flow of scripts/android-tv-e2e.sh leaves it out
 * (`-e notClass`). Photos go to the app's `files/store/`.
 *
 * Arguments: `slouchcatServer`, `slouchcatRelay`, `slouchcatEmail`, `slouchcatPassword`,
 * `slouchcatPin` (of the primary profile).
 */
@RunWith(AndroidJUnit4::class)
class StoreShotsTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8897").toHttpUrl()
    private val relay: HttpUrl = (args.getString("slouchcatRelay") ?: "http://10.0.2.2:8982").toHttpUrl()
    private val email = args.getString("slouchcatEmail") ?: "demo@example.com"
    private val password = args.getString("slouchcatPassword") ?: "demo-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val context = instrumentation.targetContext

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    @Before
    fun onlyForTheStore() = check(args.getString("slouchcatStore") != null) { "store photos only through scripts/store-screenshots.sh" }

    private fun ComposeTestRule.nodesOrNull(matcher: SemanticsMatcher, unmerged: Boolean) = try {
        onAllNodes(matcher, useUnmergedTree = unmerged).fetchSemanticsNodes()
    } catch (e: IllegalStateException) {
        if (e.message?.startsWith("No compose hierarchies found in the app") == true) null else throw e
    }

    /** Waits for [matcher]; at the timeout a photo of the screen goes to the app's `files/failures/`. */
    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 60_000, unmerged: Boolean = true) = try {
        compose.waitUntil(timeout) { compose.nodesOrNull(matcher, unmerged)?.isNotEmpty() == true }
    } catch (e: androidx.compose.ui.test.ComposeTimeoutException) {
        instrumentation.uiAutomation.takeScreenshot()?.let { bitmap ->
            val dir = File(context.filesDir, "failures").apply { mkdirs() }
            File(dir, "wait-${System.currentTimeMillis()}.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
        }
        throw AssertionError("not shown within $timeout ms: ${matcher.description}", e)
    }

    private fun waitForText(text: String, timeout: Long = 60_000) = waitFor(hasText(text, substring = true), timeout)

    /** The way of the spooling in its bubble above the bar («+5:40», `tv_player_ahead`), nothing else with a «+». */
    private val spoolWay: SemanticsMatcher by lazy {
        val marked = s(R.string.tv_player_ahead, MARK)
        val before = marked.substringBefore(MARK)
        val after = marked.substringAfter(MARK)
        SemanticsMatcher("the way of the spooling") { node ->
            node.config.getOrNull(SemanticsProperties.Text).orEmpty().any { line ->
                val t = line.text
                t.length > before.length + after.length && t.startsWith(before) && t.endsWith(after) &&
                    t.substring(before.length, t.length - after.length).first().isDigit()
            }
        }
    }

    private fun present(text: String): Boolean =
        compose.nodesOrNull(hasText(text, substring = true) or hasContentDescription(text, substring = true), unmerged = true)?.isNotEmpty() == true

    private fun isFocusedOn(text: String): Boolean =
        compose.nodesOrNull(isFocused() and (hasText(text, substring = true) or hasContentDescription(text, substring = true)), unmerged = false)?.isNotEmpty() == true

    private fun focused(text: String) = waitFor(isFocused() and (hasText(text, substring = true) or hasContentDescription(text, substring = true)), unmerged = false)

    private fun key(code: Int, times: Int = 1) = repeat(times) {
        instrumentation.sendKeyDownUpSync(code)
        compose.waitForIdle()
    }

    /** A field: OK opens the input of the system, the text goes in, «Fertig» takes it. */
    private fun type(label: String, text: String) {
        focused(label)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasSetTextAction() and hasContentDescription(label))
        compose.onNode(hasSetTextAction() and hasContentDescription(label)).performTextInput(text)
        compose.onNode(hasSetTextAction() and hasContentDescription(label)).performImeAction()
        compose.waitForIdle()
    }

    /** From the digit 1 of the pad down to «Öffnen» (bottom right) with the D-pad, and OK. */
    private fun submitPin() {
        focused("1")
        key(KeyEvent.KEYCODE_DPAD_DOWN, 3)
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
        focused(s(R.string.profiles_pin_submit))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
    }

    /** A photo once the screen holds still ([STILL_COUNT] screenshots in a row the same), at most [settleMs] long. */
    private fun photo(name: String, settleMs: Long = SETTLE_MS) {
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        val deadline = SystemClock.uptimeMillis() + settleMs
        var last: Bitmap? = null
        var bitmap: Bitmap
        var same = 1
        while (true) {
            bitmap = requireNotNull(instrumentation.uiAutomation.takeScreenshot()) { "no screenshot" }
            same = if (last != null && bitmap.sameAs(last)) same + 1 else 1
            if (same >= STILL_COUNT) break
            last?.recycle()
            last = bitmap
            if (SystemClock.uptimeMillis() > deadline) break
            compose.waitForIdle()
            instrumentation.uiAutomation.waitForIdle(STILL_MS, STILL_MAX_MS)
        }
        // The app's own files: the TV image refuses its external files to the test (EACCES on API 34).
        val dir = File(context.filesDir, "store").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    /** The lines «first picture» the player logged (debug build). */
    private fun firstPictures(): Int {
        val process = Runtime.getRuntime().exec(arrayOf("logcat", "-d", "-v", "raw", "-s", "SlouchcatTv/player:I"))
        return try {
            InputStreamReader(process.inputStream).use { reader -> reader.readLines().count { it.contains("first picture") } }
        } finally {
            process.destroy()
        }
    }

    /** Along a row of the start page to the card [title]: to the right, else back to the left. */
    private fun toCard(title: String) {
        repeat(ROW_MAX) { if (!isFocusedOn(title)) key(KeyEvent.KEYCODE_DPAD_RIGHT) }
        repeat(ROW_MAX * 2) { if (!isFocusedOn(title)) key(KeyEvent.KEYCODE_DPAD_LEFT) }
        focused(title)
    }

    /** Down a menu or the buttons of the player until [text] has the focus. */
    private fun along(text: String, code: Int) {
        repeat(MENU_MAX) { if (!isFocusedOn(text)) key(code) }
        focused(text)
    }

    @Test
    fun tv() {
        Runtime.getRuntime().exec(arrayOf("logcat", "-c")).waitFor()
        runBlocking { DataStoreRelaySettings(context, "tv").set(relay) }
        val scenario = ActivityScenario.launch(MainActivity::class.java)

        // The setup by scan: the QR code of a meeting point at the local relay.
        waitForText(s(R.string.tv_setup_welcome))
        focused(s(R.string.tv_setup_phoneTitle))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(SemanticsMatcher.keyIsDefined(QrContent))
        waitForText(s(R.string.tv_pair_waiting))
        focused(s(R.string.tv_pair_newCode))
        photo("04-setup-qr", settleMs = QR_SETTLE_MS)

        // By hand: address, e-mail, password, the primary profile with its PIN.
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.tv_pair_byHand))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_serverLead))
        type(s(R.string.tv_hand_serverLabel), server.toString().trimEnd('/'))
        waitForText(server.host)
        focused(s(R.string.tv_hand_next))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_emailLead))
        type(s(R.string.tv_hand_emailLabel), email)
        focused(s(R.string.tv_hand_next))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_passwordLead, server.host))
        type(s(R.string.tv_hand_passwordLabel), password)
        focused(s(R.string.tv_hand_signIn))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.profiles_picker_title))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_pin_prompt))
        pin.forEach { key(KeyEvent.KEYCODE_0 + (it - '0')) }
        submitPin()

        // Start with the focus on Tears of Steel in the first row: the page stands at its top, no
        // row is cut. A card of «Weiterschauen» starts the player at once: for the detail page
        // the film of the row below.
        waitForText(TEARS, timeout = SCAN_MS)
        toCard(TEARS)
        photo("01-home")
        if (present(s(R.string.home_rows_continue))) {
            key(KeyEvent.KEYCODE_DPAD_DOWN)
            toCard(TEARS)
        }
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasContentDescription(s(R.string.tv_item_more)))
        // «Abspielen», or «Weiterschauen» once the phone's player of this language ran.
        waitFor(isFocused() and (hasText(s(R.string.app_item_play), substring = true) or hasText(s(R.string.app_item_resume), substring = true)), unmerged = false)
        photo("02-detail")

        // The player from the start (beside «Weiterschauen» the button «Von vorne»), so the spooling
        // ends inside the film and not in its end credits; OK seeks, then the sound menu.
        if (isFocusedOn(s(R.string.app_item_resume))) {
            key(KeyEvent.KEYCODE_DPAD_RIGHT)
            focused(s(R.string.app_item_fromStart))
        }
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.player_seek))
        compose.waitUntil(PLAY_MS) { firstPictures() >= 1 }
        key(KeyEvent.KEYCODE_DPAD_RIGHT, SPOOL_STEPS)
        waitFor(spoolWay)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        compose.waitUntil(PLAY_MS) { compose.nodesOrNull(spoolWay, unmerged = true)?.isEmpty() == true }
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        along(s(R.string.player_tracks_audio), KeyEvent.KEYCODE_DPAD_LEFT)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.player_tracks_audio))
        photo("03-player-audio")
        scenario.close()
    }

    private companion object {
        const val TEARS = "Tears of Steel"

        /** A placeholder of the format of `tv_player_ahead`. */
        const val MARK = "\u0001"

        /** The first scan of four films and an audiobook, probes included (seconds on this machine). */
        const val SCAN_MS = 120_000L

        /** A start in the emulator's software decoder, possibly after a remux on the server. */
        const val PLAY_MS = 120_000L

        /** Long enough for the posters of a page to load from the local server. */
        const val SETTLE_MS = 15_000L

        /** The code page counts its validity down every second: it never holds still. */
        const val QR_SETTLE_MS = 2_000L
        /** Two alone once caught a poster still on its way (phone). */
        const val STILL_COUNT = 3
        const val STILL_MS = 500L
        const val STILL_MAX_MS = 3_000L
        const val ROW_MAX = 6
        const val MENU_MAX = 10

        /** Presses of Rechts: the steps of the bar grow to about five minutes into the film. */
        const val SPOOL_STEPS = 7
    }
}
