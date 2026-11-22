package com.slouchcat.app.tv.notifications

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import com.slouchcat.app.tv.shell.ShellTags
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeHome
import com.slouchcat.core.browse.testing.FakeNotifications
import com.slouchcat.core.browse.testing.FakeWishes
import com.slouchcat.core.browse.testing.announcement
import com.slouchcat.core.browse.testing.film
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.browse.testing.notification
import com.slouchcat.core.strings.R
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * «Benachrichtigungen» with the remote (mockup `/notifications`, reference photo tv 35): from
 * «Einstellungen», the announcement with «Verstanden» first, the unread entries, an entry to its
 * title or to «Meine Wünsche» and back, «Alle als gelesen markieren», the empty inbox, a failure,
 * and the number on «Einstellungen» in the rail. The focus moves on after every change.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class NotificationsScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote

    private val notes = FakeNotifications(
        entries = mutableListOf(
            notification(1, item = itemRef(5, "Pilot Five")),
            notification(2, Notification.Kind.WISH_APPROVED, item = null, wishTitle = "Wished Two"),
            notification(3, read = true, item = itemRef(6, "Old Six")),
        ),
        open = mutableListOf(announcement(9, "Maintenance Nine")),
    )
    private val details = FakeDetails(film(5, title = "Pilot Five"))

    private fun show() = pages.show({ it.copy(home = FakeHome(), notifications = notes, wishes = FakeWishes(), detail = details, items = details) })

    /** «Einstellungen» from the rail, then down to «Benachrichtigungen», OK. */
    private fun open() {
        pages.openRail(pages.s(R.string.tv_settings_title))
        pages.waitFor(pages.s(R.string.notifications_title))
        repeat(MAX_ROWS) { if (!remote.focusedLabel().contains(pages.s(R.string.notifications_title))) remote.down() }
        remote.assertFocused(pages.s(R.string.notifications_title))
        remote.ok()
    }

    /** The entry «Einstellungen» of the rail says [text] (its number is read a moment after the rail is drawn). */
    private fun badgeSays(text: String) = compose.waitUntil(5_000) {
        runCatching { compose.onNodeWithTag(ShellTags.entry("settings")).assert(hasContentDescription(text, substring = true)) }.isSuccess
    }

    private fun unreadLabel(count: Int) = pages.context.resources.getQuantityString(R.plurals.notifications_unreadLabel, count, count)

    @Test
    fun theAnnouncementComesFirstAndAnEntryLeadsToItsTitle() {
        show()
        // Two unread entries and an announcement: the number on «Einstellungen».
        badgeSays(unreadLabel(3))
        open()
        pages.waitFor("Maintenance Nine")
        remote.assertFocused(pages.s(R.string.announcements_acknowledge))
        remote.assertFocused("Maintenance Nine")
        remote.assertEveryControlReachable(pages.page)
        // «Verstanden»: the card goes, the focus to «Alle als gelesen markieren».
        remote.ok()
        compose.waitUntil(5_000) { notes.calls.contains("ack:9") }
        remote.settle()
        remote.assertFocused(pages.s(R.string.notifications_markAllRead))
        remote.down()
        remote.assertFocused("Pilot Five")
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        assertTrue(notes.calls.contains("read:1"))
        remote.back()
        remote.assertFocused("Pilot Five")
        badgeSays(unreadLabel(1))
    }

    @Test
    fun anEntryAboutAWishLeadsToMyWishes() {
        show()
        open()
        pages.waitFor("Wished Two")
        repeat(MAX_ROWS) { if (!remote.focusedLabel().contains("Wished Two")) remote.down() }
        remote.assertFocused(pages.s(R.string.notifications_wish_approved))
        remote.ok()
        pages.waitFor(pages.s(R.string.wishes_page_empty_title))
        remote.assertFocused(pages.s(R.string.wishes_area_mine))
    }

    @Test
    fun markAllReadMovesTheFocusToTheList() {
        notes.open.clear()
        show()
        open()
        pages.waitFor("Pilot Five")
        // Without an announcement the first unread entry has the focus.
        remote.assertFocused("Pilot Five")
        remote.up()
        remote.assertFocused(pages.s(R.string.notifications_markAllRead))
        remote.ok()
        compose.waitUntil(5_000) { notes.calls.contains("readAll") }
        pages.waitFor(pages.s(R.string.tv_notifications_allRead))
        remote.assertFocused("Pilot Five")
    }

    @Test
    fun anEmptyInboxLeadsToStartAndAFailureOffersRetry() {
        notes.entries.clear()
        notes.open.clear()
        show()
        open()
        pages.waitFor(pages.s(R.string.notifications_empty_title))
        remote.assertFocused(pages.s(R.string.app_lists_toStart))
        remote.back()
        notes.failure = ApiResult.Offline
        remote.ok()
        pages.waitFor(pages.s(R.string.app_state_retry))
        remote.assertFocused(pages.s(R.string.app_state_retry))
        notes.failure = null
        notes.entries += notification(4, item = itemRef(5, "Pilot Five"))
        remote.ok()
        pages.waitFor("Pilot Five")
        remote.assertFocused("Pilot Five")
    }

    private companion object {
        const val MAX_ROWS = 8
    }
}
