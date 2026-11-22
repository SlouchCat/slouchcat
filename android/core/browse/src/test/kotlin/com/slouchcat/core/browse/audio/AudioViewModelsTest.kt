package com.slouchcat.core.browse.audio

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.radio.RadioMessage
import com.slouchcat.core.browse.radio.RadioViewModel
import com.slouchcat.core.browse.testing.FakeAudio
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeRadio
import com.slouchcat.core.browse.testing.episodeRef
import com.slouchcat.core.browse.testing.radioStation
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.browse.testing.ulid
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.api.generated.models.RadioPreferences
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class AudioViewModelsTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    /** A session whose stores are read (the pages are built once a profile is chosen). */
    private fun kotlinx.coroutines.test.TestScope.ready() = session(backgroundScope).also { settle() }

    @Test
    fun aSeriesWithItsVolumes() = runTest(dispatcher) {
        val audio = FakeAudio()
        val vm = AudioSeriesViewModel(ready(), audio, 4, "series-1")
        settle()
        val series = (vm.state.value as Loadable.Ready).value
        assertEquals(3, series.volumes.size)
        assertEquals("series:4:series-1", audio.asked.single())
        // Back from the player: read again without a spinner.
        vm.onShown()
        vm.onShown()
        settle()
        assertEquals(2, audio.asked.size)
        assertTrue(vm.state.value is Loadable.Ready)
    }

    @Test
    fun aSeriesTheProfileDoesNotSee() = runTest(dispatcher) {
        val audio = FakeAudio().apply { series = ApiResult.NotFound }
        val vm = AudioSeriesViewModel(ready(), audio, 4, "series-1")
        settle()
        assertEquals(Loadable.Failed(LoadError.NotFound), vm.state.value)
    }

    @Test
    fun episodesInPagesAndTheirCheckMark() = runTest(dispatcher) {
        val audio = FakeAudio().apply { episodes = (1..70).map { episodeRef(100 + it) } }
        val details = FakeDetails()
        val vm = PodcastEpisodesViewModel(ready(), audio, details, ulid(60))
        settle()
        assertEquals(50, vm.state.value.loaded.size)
        assertEquals(70, vm.state.value.total)
        vm.onNearEnd()
        settle()
        assertEquals(70, vm.state.value.loaded.size)
        val first = vm.state.value.loaded.first()
        vm.onHeard(first)
        // At once, before the server answered.
        testScheduler.runCurrent()
        assertEquals(WatchedMark.Full, vm.state.value.loaded.first().watchedMark)
        settle()
        assertEquals("watched:${first.id}:true", details.changes.single())
        assertTrue(vm.busy.value.isEmpty())
    }

    @Test
    fun aRefusedCheckMarkGoesBack() = runTest(dispatcher) {
        val details = FakeDetails().apply { watched = ApiResult.Failed(500, null) }
        val vm = PodcastEpisodesViewModel(ready(), FakeAudio(), details, ulid(60))
        settle()
        vm.onHeard(vm.state.value.loaded.first())
        settle()
        assertEquals(WatchedMark.None, vm.state.value.loaded.first().watchedMark)
        assertEquals(EpisodesMessage.Failed, vm.messages.first())
    }

    @Test
    fun theRadioOpensOnTheProfilesCountry() = runTest(dispatcher) {
        val radio = FakeRadio().apply { stations = stations + radioStation(9, country = "DE") }
        val vm = RadioViewModel(ready(), radio)
        settle()
        assertEquals("CH", vm.ui.value.myCountry)
        assertEquals("CH", vm.ui.value.filter.country)
        assertEquals(5, vm.stations.value.loaded.size)
        assertEquals(listOf("CH", "DE"), vm.ui.value.countries.map { it.`value` })
        vm.onCountry(null)
        settle()
        assertEquals(6, vm.stations.value.loaded.size)
        vm.onFavoritesOnly()
        settle()
        assertEquals(listOf(2L), vm.stations.value.loaded.map { it.id })
        vm.onSearch("  Radio 9 ")
        settle()
        assertEquals("Radio 9", radio.queries.last().text)
    }

    @Test
    fun withoutStationsOfItsCountryTheRadioOpensUnfiltered() = runTest(dispatcher) {
        val radio = FakeRadio().apply { preferences = ApiResult.Ok(RadioPreferences(defaultCountry = "IS", effectiveCountry = null)) }
        val vm = RadioViewModel(ready(), radio)
        settle()
        assertNull(vm.ui.value.myCountry)
        assertTrue(vm.ui.value.filter.unfiltered)
    }

    @Test
    fun aHeartChangesBothListsAtOnce() = runTest(dispatcher) {
        val radio = FakeRadio()
        val count = LiveCount().apply { set(1) }
        val vm = RadioViewModel(ready(), radio, count)
        settle()
        val three = vm.stations.value.loaded.first { it.id == 3L }
        vm.onFavorite(three)
        testScheduler.runCurrent()
        assertTrue(vm.stations.value.loaded.first { it.id == 3L }.favorite)
        assertEquals(listOf(2L, 3L), (vm.ui.value.favorites as Loadable.Ready).value.map { it.id })
        settle()
        assertEquals("favorite:3:true", radio.calls.single())
        assertEquals(2, count.count.value)
        assertEquals(RadioMessage.Added("Radio 3"), vm.messages.first())
        // Removing the last ones leaves «Meine Sender» empty.
        vm.onFavorite(vm.stations.value.loaded.first { it.id == 2L })
        vm.onFavorite(vm.stations.value.loaded.first { it.id == 3L })
        settle()
        assertEquals(Loadable.Empty, vm.ui.value.favorites)
    }

    @Test
    fun aRefusedHeartGoesBack() = runTest(dispatcher) {
        val radio = FakeRadio().apply { favoriteAnswer = ApiResult.Offline }
        val vm = RadioViewModel(ready(), radio)
        settle()
        vm.onFavorite(vm.stations.value.loaded.first { it.id == 3L })
        settle()
        assertFalse(vm.stations.value.loaded.first { it.id == 3L }.favorite)
        assertEquals(listOf(2L), (vm.ui.value.favorites as Loadable.Ready).value.map { it.id })
        assertEquals(RadioMessage.Failed, vm.messages.first())
    }
}
