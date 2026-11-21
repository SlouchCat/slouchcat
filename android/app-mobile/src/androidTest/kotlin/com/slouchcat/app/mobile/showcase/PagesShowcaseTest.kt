package com.slouchcat.app.mobile.showcase

import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.browse.CollectionRoute
import com.slouchcat.app.mobile.browse.CollectionsRoute
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.PersonRoute
import com.slouchcat.app.mobile.browse.ProfilesSettingsRoute
import com.slouchcat.app.mobile.browse.WatchedRoute
import com.slouchcat.app.mobile.browse.WatchlistRoute
import com.slouchcat.app.mobile.playback.PlaybackSettingsFeature
import com.slouchcat.app.mobile.playback.PlaybackSettingsRoute
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.strings.R
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The pages after the start with content (concept 27, visual check against the mockup): film,
 * series with seasons, season, episode, person, search empty and with hits, «Merkliste»,
 * «Gesehen», the collections, a collection, «Profile verwalten», the profile choice with «Profile
 * verwalten», «Einstellungen › Wiedergabe» and «Mehr», over [PagesShowcaseData] and drawn
 * pictures; no server. Runs in every emulator flow; photos with `slouchcatShots` as `d…`.
 */
@RunWith(AndroidJUnit4::class)
class PagesShowcaseTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private val pages = features.map { feature ->
        if (feature is PlaybackSettingsFeature) PlaybackSettingsFeature(settings = { MemoryDeviceSettingsStore() }, repository = { PagesShowcaseData.playbackPrefs }) else feature
    }

    private val images = ShowcaseImages(BrowseShowcaseData.names + PagesShowcaseData.names).also { it.squareIds = BrowseShowcaseData.squareIds }

    private val showcase = Showcase(compose, pages, BrowseShowcaseData.libraries, images, PagesShowcaseData.auth, PagesShowcaseData.devices) { graph ->
        PagesShowcaseData.data(MediaData.api(graph.api, graph.http))
    }

    @After
    fun close() = showcase.close()

    private fun open(target: String, vararg routes: Any) =
        InstrumentationRegistry.getInstrumentation().runOnMainSync { showcase.graph.links.request(AppLink(target, routes.toList())) }

    private fun heading(text: String) = (hasText(text) or hasContentDescription(text)) and isHeading()

    /** Brings the first node of [matcher] into view (the detail pages scroll as a column, not a lazy list). */
    private fun scrollTo(matcher: SemanticsMatcher) = compose.onAllNodes(matcher).onFirst().performScrollTo()

    private fun startPage() = showcase.waitFor(hasText(s(R.string.home_rows_continue)))

    @Test
    fun pagesWithContent() {
        showcase.start()
        startPage()

        // Film: logo, facts, the big button with «Von vorn», actions, summary; below cast and collection.
        open("start", ItemRoute(PagesShowcaseData.filmId))
        showcase.waitFor(hasText("Jede Station ein neues Leben."))
        showcase.shot("d01-film")
        scrollTo(hasText(s(R.string.app_item_cast)))
        showcase.shot("d02-film-cast")

        // Series with season chips and episodes.
        open("start", ItemRoute(PagesShowcaseData.seriesId))
        showcase.waitFor(hasText("Unter dem Eis", substring = true))
        showcase.shot("d03-series")
        scrollTo(hasText("Die Karte", substring = true))
        showcase.shot("d04-series-episodes")

        // A season and an episode as pages of their own.
        open("start", ItemRoute(PagesShowcaseData.seasonId))
        showcase.waitFor(hasText("Das erste Licht", substring = true))
        showcase.shot("d05-season")
        open("start", ItemRoute(PagesShowcaseData.episodeId))
        showcase.waitFor(hasText(s(R.string.item_seriesLink)))
        showcase.shot("d06-episode")

        // A person.
        open("start", PersonRoute(PagesShowcaseData.PERSON_ID))
        showcase.waitFor(heading("Ilka Morand"))
        showcase.shot("d07-person")

        // The search: without a text, then with hits.
        open("search")
        showcase.waitFor(hasText("Science Fiction"))
        showcase.shot("d08-search-empty")
        compose.onAllNodes(hasSetTextAction()).onFirst().performTextInput("Nacht")
        showcase.waitFor(hasText("Nachtzug nach Lumen", substring = true) or hasContentDescription("Nachtzug nach Lumen", substring = true))
        showcase.shot("d09-search-hits")

        // My lists and the collections.
        open("start", WatchlistRoute)
        showcase.waitFor(heading(s(R.string.nav_watchlist)))
        showcase.waitFor(hasContentDescription("Stimmen im Schnee", substring = true) or hasContentDescription("Der Kartograf", substring = true))
        showcase.shot("d10-watchlist")
        open("start", WatchedRoute)
        showcase.waitFor(heading(s(R.string.nav_watched)))
        showcase.waitFor(hasContentDescription("Fünf Brücken", substring = true) or hasText("Fünf Brücken", substring = true))
        showcase.shot("d11-watched")
        open("start", CollectionsRoute)
        showcase.waitFor(hasText("Hafengeschichten", substring = true) or hasContentDescription("Hafengeschichten", substring = true))
        showcase.shot("d12-collections")
        open("start", CollectionsRoute, CollectionRoute(PagesShowcaseData.collectionId))
        showcase.waitFor(heading("Lumen-Reihe"))
        showcase.shot("d13-collection")

        // «Profile verwalten» and the settings of the playback.
        open("more", ProfilesSettingsRoute)
        showcase.waitFor(hasText("Kinder"))
        showcase.shot("d14-profiles-manage")
        open("more", PlaybackSettingsRoute)
        showcase.waitFor(heading(s(R.string.app_playback_title)))
        showcase.shot("d15-settings-playback")

        // «Mehr», then the profile choice with «Profile verwalten».
        open("more")
        showcase.waitFor(heading(s(R.string.app_more_title)))
        showcase.waitFor(hasText(s(R.string.app_profiles_manage_title)))
        showcase.shot("d16-more")
        compose.onAllNodes((hasText(s(R.string.app_more_switchProfile)) or hasContentDescription(s(R.string.app_more_switchProfile))) and hasClickAction()).onFirst().performClick()
        showcase.waitFor(hasText(s(R.string.profiles_picker_title)))
        showcase.waitFor(hasText("Jonas"))
        showcase.shot("d17-profile-choice")
    }
}
