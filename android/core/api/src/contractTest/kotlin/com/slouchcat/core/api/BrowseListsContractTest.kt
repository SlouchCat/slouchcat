package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.ProfilesApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import com.slouchcat.core.api.generated.models.OfflinePinRequest
import com.slouchcat.core.api.generated.models.PinRequest
import com.slouchcat.core.api.generated.models.PlaybackLimits
import com.slouchcat.core.api.generated.models.ProfileRequest
import com.slouchcat.core.api.generated.models.ProfileUpdate
import com.slouchcat.core.api.generated.models.SearchRequest
import com.slouchcat.core.api.generated.models.SelectProfileRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.io.File

/**
 * The operations of the last part of stage 4 against a real server (task android:contract):
 * `search`, `listGenres`, `listWatchlist` with `addToWatchlist`/`removeFromWatchlist`,
 * `listWatched`, `listVisibleCollections`, and «Profile verwalten» (`listProfiles`,
 * `createProfile`, `updateProfile`, `setProfilePin`, `removeProfilePin`, `setOfflinePin`,
 * `removeOfflinePin`, `listAccountLibraries`, `deleteProfile`), including 403
 * `primary_profile_required` for another profile. The test library comes from
 * [DetailContractTest.Admin] (the media tree of scripts/android-test-media.sh).
 */
class BrowseListsContractTest {
    private val base: HttpUrl = System.getProperty("slouchcat.contract.base").orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which starts the server" }
    }.toHttpUrl()

    private val media: File = File(
        System.getProperty("slouchcat.contract.media").orEmpty().also {
            require(it.isNotEmpty()) { "run through `task android:contract`, which writes the test media" }
        },
    )

    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")

    /** The app's session of the owner's primary profile (Bearer, no gate: the server is local). */
    private fun appClient(): OkHttpClient {
        val login = SlouchcatApi.create(::AuthApi, base, SlouchcatHttp.client(identity))
            .appLogin(AppLoginRequest(EMAIL, PASSWORD, DEVICE_NAME, AppLoginRequest.Kind.PHONE))!!
        val credentials = object : SessionCredentials {
            override val server = base
            override val bearerToken = login.token
            override val deviceId: String? = null
        }
        val http = SlouchcatHttp.client(identity, credentials)
        val auth = SlouchcatApi.create(::AuthApi, base, http)
        auth.selectProfile(auth.listMyProfiles()!!.single { it.isPrimary }.id)
        return http
    }

    private fun prepare(): OkHttpClient {
        val setup = SlouchcatApi.create(::SetupApi, base, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = EMAIL, password = PASSWORD))
        DetailContractTest.Admin(base).ensureLibrary(media)
        return appClient()
    }

    private fun refused(status: Int, code: String? = null, block: () -> Unit) {
        try {
            block()
            fail("the server took it")
        } catch (e: ClientException) {
            assertEquals(status, e.statusCode)
            if (code != null) assertTrue(e.response.toString(), (e.response as? com.slouchcat.core.api.generated.infrastructure.ClientError<*>)?.body.toString().contains(code))
        }
    }

    @Test
    fun searchListsAndCollections() {
        val http = prepare()
        val discover = SlouchcatApi.create(::DiscoverApi, base, http)
        val items = SlouchcatApi.create(::ItemsApi, base, http)

        // The search: a title by a part of its name, people and collections as lists; one character is too short.
        val found = discover.search(SearchRequest(query = "night"))!!
        val film = found.items.single { it.title == "Night Train" }
        assertEquals(found.items.size, found.total)
        assertNotNull(found.people)
        assertNotNull(found.collections)
        assertEquals(0, discover.search(SearchRequest(query = "zzzqqq"))!!.total)
        refused(422) { discover.search(SearchRequest(query = "n")) }
        assertNotNull(discover.listGenres())

        // The watchlist: on, listed, off.
        discover.addToWatchlist(film.id)
        val listed = discover.listWatchlist(0, 50)!!
        assertTrue(listed.items.any { it.id == film.id })
        assertEquals(listed.items.size, listed.total)
        discover.removeFromWatchlist(film.id)
        assertFalse(discover.listWatchlist(0, 50)!!.items.any { it.id == film.id })

        // «Gesehen»: the check mark makes the entry, its removal takes it off.
        items.setItemWatched(film.id, ItemWatchedRequest(true))
        val watched = discover.listWatched(offset = 0, limit = 50)!!
        val entry = watched.items.single { it.item.id == film.id }
        assertTrue(entry.episodeCount == null)
        items.setItemWatched(film.id, ItemWatchedRequest(false))
        assertFalse(discover.listWatched(offset = 0, limit = 50)!!.items.any { it.item.id == film.id })

        // Collections: without TMDB the test media have none; the list is there and empty.
        assertNotNull(discover.listVisibleCollections())
        refused(404) { discover.getCollection("01J0000000000000000000ZZZZ") }
    }

    @Test
    fun manageProfilesWithPinsAndTheRefusalForAnotherProfile() {
        val http = prepare()
        val profiles = SlouchcatApi.create(::ProfilesApi, base, http)
        val auth = SlouchcatApi.create(::AuthApi, base, http)

        val primary = profiles.listProfiles()!!.single { it.isPrimary }
        assertTrue(profiles.listAccountLibraries()!!.any { it.name == LIBRARY })
        val kid = profiles.createProfile(
            ProfileRequest(name = "Contract kid", maxRating = 12, accent = Accent.SKY, limits = PlaybackLimits(watchLimitMinutes = 90)),
        )!!
        assertEquals(12, kid.maxRating)
        assertEquals(90, kid.limits.watchLimitMinutes)
        assertFalse(kid.hasPin)

        // Only the fields sent change (`limits` is one field).
        val renamed = profiles.updateProfile(kid.id, ProfileUpdate(name = "Contract child", accent = Accent.ROSE))!!
        assertEquals("Contract child", renamed.name)
        assertEquals(Accent.ROSE, renamed.accent)
        assertEquals(12, renamed.maxRating)
        assertEquals(90, renamed.limits.watchLimitMinutes)

        profiles.setProfilePin(kid.id, PinRequest("2468"))
        profiles.setOfflinePin(kid.id, OfflinePinRequest("135790"))
        val locked = profiles.listProfiles()!!.single { it.id == kid.id }
        assertTrue(locked.hasPin)
        assertTrue(locked.hasOfflinePin)
        refused(422) { profiles.setProfilePin(kid.id, PinRequest("12")) }
        refused(422) { profiles.setOfflinePin(kid.id, OfflinePinRequest("1234")) }

        // Another profile only reads: the server refuses the management (403, not 404).
        auth.selectProfile(kid.id, SelectProfileRequest("2468"))
        refused(403, "primary_profile_required") { profiles.listProfiles() }
        refused(403, "primary_profile_required") { profiles.deleteProfile(kid.id) }
        auth.selectProfile(primary.id)

        profiles.removeProfilePin(kid.id)
        profiles.removeOfflinePin(kid.id)
        val open = profiles.listProfiles()!!.single { it.id == kid.id }
        assertFalse(open.hasPin)
        assertFalse(open.hasOfflinePin)

        // The primary profile stays; the other one goes.
        refused(409, "primary_profile") { profiles.deleteProfile(primary.id) }
        profiles.deleteProfile(kid.id)
        assertFalse(profiles.listProfiles()!!.any { it.id == kid.id })
    }

    private companion object {
        // The account of ContractTest: the server of the task has one owner.
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
        const val DEVICE_NAME = "Contract phone · Android 16"
        const val LIBRARY = "Android test media"
    }
}
