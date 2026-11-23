package com.slouchcat.app.mobile.showcase

import android.Manifest
import android.accessibilityservice.AccessibilityService
import android.os.Build
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.browse.NotificationsRoute
import com.slouchcat.app.mobile.home.HomeRoute
import com.slouchcat.app.mobile.more.MoreRoute
import com.slouchcat.app.mobile.notifications.NotificationSettingsRoute
import com.slouchcat.app.mobile.notifications.NotificationsFeature
import com.slouchcat.app.mobile.push.PushNotifier
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.app.mobile.wishes.WishTitleRoute
import com.slouchcat.app.mobile.wishes.WishesFeature
import com.slouchcat.app.mobile.wishes.WishesRoute
import com.slouchcat.core.api.push.PushMessage
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.strings.R
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The pages of stage 8 with made-up data (visual check against the mockup, photos `w…` with
 * `slouchcatShots`): «Wünsche» filled, empty and with Seerr not set up, «Meine Wünsche», a wish,
 * the sheet «Anfordern», the search with «Noch nicht in der Bibliothek?», «Benachrichtigungen»,
 * «Einstellungen › Benachrichtigungen», the start with the bell's badge, and the notification itself.
 */
@RunWith(AndroidJUnit4::class)
class MessagesShowcaseTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private var showcase: Showcase? = null

    private fun start(data: MessagesShowcaseData): Showcase {
        val pages = features.map { feature ->
            if (feature is NotificationsFeature) NotificationsFeature(devicePush = { data.devicePush }, allowed = { true }) else feature
        }
        val images = ShowcaseImages(BrowseShowcaseData.names + PagesShowcaseData.names + data.names).also { it.squareIds = BrowseShowcaseData.squareIds }
        return Showcase(compose, pages, BrowseShowcaseData.libraries, images, PagesShowcaseData.auth, PagesShowcaseData.devices) { graph ->
            data.data(PagesShowcaseData.data(MediaData.api(graph.api, graph.http)))
        }.also {
            showcase = it
            it.start()
            it.waitFor(hasText(s(R.string.home_rows_continue)))
        }
    }

    @After
    fun close() {
        showcase?.close()
    }

    private fun Showcase.open(target: String, vararg routes: Any) =
        InstrumentationRegistry.getInstrumentation().runOnMainSync { graph.links.request(AppLink(target, routes.toList())) }

    @Test
    fun wishesAndNotifications() {
        val data = MessagesShowcaseData()
        val show = start(data)

        // The start with the bell and its badge (unread entries and the announcement).
        show.waitFor(hasContentDescription(context.resources.getQuantityString(R.plurals.app_notifications_bell, 3, 3)))
        show.shot("w10-start-bell")

        // «Wünsche»: «Entdecken» filled, «Meine Wünsche», a wish, a series with its sheet.
        show.open(WishesFeature.TARGET, WishesRoute())
        show.waitFor(hasText(s(R.string.wishes_discover_rows_trending)))
        show.shot("w11-wishes-discover")
        show.open(WishesFeature.TARGET, WishesRoute(tab = WishesFeature.MINE))
        show.waitFor(hasText("Nebelhafen", substring = true))
        show.shot("w12-wishes-mine")
        show.open(WishesFeature.TARGET, WishesRoute(), WishTitleRoute("movie", data.requestedId))
        show.waitFor(hasText(s(R.string.wishes_withdraw_action)))
        show.shot("w13-wish")
        show.open(WishesFeature.TARGET, WishesRoute(), WishTitleRoute("movie", data.approvedId))
        show.waitFor(hasText("Kupfermond") and isHeading())
        show.shot("w13b-wish-approved")
        show.open(WishesFeature.TARGET, WishesRoute(), WishTitleRoute("show", data.seriesId))
        show.waitFor(hasText(s(R.string.app_wishes_request)))
        show.shot("w14-wish-series")
        compose.onAllNodes(hasText(s(R.string.app_wishes_request)) and hasClickAction()).onFirst().performClick()
        show.waitFor(hasText(s(R.string.app_wishes_allSeasons)))
        show.shot("w15-wish-sheet")
        show.back()

        // The search with «Noch nicht in der Bibliothek?» and the card to the wishes' search.
        show.open("search")
        show.waitFor(hasSetTextAction())
        compose.onAllNodes(hasSetTextAction()).onFirst().performTextInput("Glas")
        show.waitFor(hasText(s(R.string.app_wishes_missingTitle)))
        show.shot("w16-search-wishes")

        // «Benachrichtigungen» and their settings.
        show.open("start", HomeRoute, NotificationsRoute)
        show.waitFor(hasText(s(R.string.notifications_title)) and isHeading())
        show.waitFor(hasText(s(R.string.announcements_acknowledge)))
        show.shot("w17-notifications")
        show.open("more", MoreRoute, NotificationSettingsRoute)
        show.waitFor(hasText(s(R.string.app_notifications_push)))
        show.shot("w18-settings-notifications")
        // Switched off without the server's answer: the removal waits (code review M-3).
        data.pushState.value = DevicePushState.OffPending
        show.waitFor(hasText(s(R.string.app_notifications_pushOffPending)))
        show.shot("w18b-settings-off-pending")

        // The notification itself, in the shade of the system.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            InstrumentationRegistry.getInstrumentation().uiAutomation.grantRuntimePermission(context.packageName, Manifest.permission.POST_NOTIFICATIONS)
        }
        PushNotifier.show(context, PushMessage("Mara: Neue Folge", "Nebelhafen · Staffel 3, Folge 4 ist da", "/items/x"), "/items/x")
        InstrumentationRegistry.getInstrumentation().uiAutomation.performGlobalAction(AccessibilityService.GLOBAL_ACTION_NOTIFICATIONS)
        InstrumentationRegistry.getInstrumentation().uiAutomation.waitForIdle(500, 5_000)
        show.shot("w19-notification")
        InstrumentationRegistry.getInstrumentation().uiAutomation.performGlobalAction(AccessibilityService.GLOBAL_ACTION_BACK)
    }

    @Test
    fun wishesEmptyAndNotSetUp() {
        val empty = start(MessagesShowcaseData(emptyWishes = true))
        empty.open(WishesFeature.TARGET, WishesRoute(tab = WishesFeature.MINE))
        empty.waitFor(hasText(s(R.string.wishes_page_empty_title)))
        empty.shot("w20-wishes-empty")
        empty.close()
        showcase = null
        val off = start(MessagesShowcaseData(wishesOff = true))
        off.open(WishesFeature.TARGET, WishesRoute())
        off.waitFor(hasText(s(R.string.app_wishes_off_title)))
        off.shot("w21-wishes-off")
    }
}
