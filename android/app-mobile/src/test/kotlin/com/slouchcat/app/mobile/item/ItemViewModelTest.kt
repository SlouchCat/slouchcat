package com.slouchcat.app.mobile.item

import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.testing.FakeDetails
import com.slouchcat.app.mobile.testing.SEASON_1
import com.slouchcat.app.mobile.testing.SEASON_2
import com.slouchcat.app.mobile.testing.SERIES_ID
import com.slouchcat.app.mobile.testing.film
import com.slouchcat.app.mobile.testing.progress
import com.slouchcat.app.mobile.testing.seasonDetail
import com.slouchcat.app.mobile.testing.series
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.app.mobile.testing.ulid
import com.slouchcat.core.api.ApiResult
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
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
import java.time.OffsetDateTime

@OptIn(ExperimentalCoroutinesApi::class)
class ItemViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val now = OffsetDateTime.parse("2026-11-21T20:00:00Z")

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.model(repository: FakeDetails, id: String): ItemViewModel {
        val s = session(backgroundScope)
        settle()
        val vm = ItemViewModel(s, repository, id) { now }
        settle()
        return vm
    }

    /** Every message the page gets while the test runs. */
    private fun TestScope.messages(vm: ItemViewModel): MutableList<ItemMessage> {
        val list = mutableListOf<ItemMessage>()
        backgroundScope.launch { vm.messages.toList(list) }
        return list
    }

    @Test
    fun aFilmLoadsWithItsTrailer() = runTest(dispatcher) {
        val vm = model(FakeDetails(film()), ulid(1))
        assertEquals(film(), vm.state.value.detail)
        assertEquals("abcDEF12345", vm.state.value.trailer)
        assertNull(vm.state.value.episodes)
    }

    @Test
    fun noTrailerWithoutOneOrWithAMalformedId() = runTest(dispatcher) {
        val none = model(FakeDetails(film(trailers = 0)), ulid(1))
        assertNull(none.state.value.trailer)
        val odd = FakeDetails(film()).apply { trailers = ApiResult.Ok(listOf(com.slouchcat.app.mobile.testing.trailer("a/b?c=d&x"))) }
        assertNull(model(odd, ulid(1)).state.value.trailer)
    }

    @Test
    fun aTitleTheProfileMayNotSeeIsNotFound() = runTest(dispatcher) {
        val vm = model(FakeDetails(), ulid(9))
        assertEquals(Loadable.Failed(LoadError.NotFound), vm.state.value.item)
    }

    @Test
    fun aFailedLoadIsRetried() = runTest(dispatcher) {
        val repository = FakeDetails()
        repository.details[ulid(1)] = ApiResult.Offline
        val vm = model(repository, ulid(1))
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.item)
        repository.details[ulid(1)] = ApiResult.Ok(film())
        vm.retry()
        settle()
        assertEquals(film(), vm.state.value.detail)
    }

    @Test
    fun theWatchlistChangesAtOnceAndGoesBackOnARefusal() = runTest(dispatcher) {
        val repository = FakeDetails(film())
        val vm = model(repository, ulid(1))
        val messages = messages(vm)
        repository.gate = CompletableDeferred()
        vm.onWatchlist()
        settle()
        // Optimistic: on before the server answered, a second press waits.
        assertTrue(vm.state.value.detail!!.inWatchlist)
        assertTrue(ItemAction.Watchlist in vm.state.value.busy)
        vm.onWatchlist()
        repository.gate!!.complete(Unit)
        settle()
        assertEquals(listOf("watchlist:${ulid(1)}:true"), repository.changes)
        assertTrue(vm.state.value.detail!!.inWatchlist)
        assertTrue(vm.state.value.busy.isEmpty())

        repository.gate = null
        repository.watchlist = ApiResult.Offline
        vm.onWatchlist()
        settle()
        assertTrue("a refusal puts the mark back", vm.state.value.detail!!.inWatchlist)
        assertEquals(listOf<ItemMessage>(ItemMessage.Failed), messages)
    }

    @Test
    fun aFilmIsMarkedWatchedAndTheMarkGoesBackOnARefusal() = runTest(dispatcher) {
        val repository = FakeDetails(film(progress = progress(40)))
        val vm = model(repository, ulid(1))
        val messages = messages(vm)
        repository.gate = CompletableDeferred()
        vm.onWatched()
        settle()
        assertEquals(WatchedMark.Full, vm.state.value.detail!!.watchedMark)
        // The server's answer, then the page reads the title again in place.
        repository.details[ulid(1)] = ApiResult.Ok(film(progress = progress(100, watched = true)))
        val loads = repository.asked.size
        repository.gate!!.complete(Unit)
        settle()
        assertEquals(loads + 1, repository.asked.size)
        assertTrue(vm.state.value.item is Loadable.Ready)
        assertEquals(WatchedMark.Full, vm.state.value.detail!!.watchedMark)

        // Clearing a single title does not ask; a refusal restores it exactly.
        repository.gate = null
        repository.watched = ApiResult.Failed(500, null)
        vm.onWatched()
        settle()
        assertNull(vm.state.value.ask)
        assertEquals(listOf("watched:${ulid(1)}:true", "watched:${ulid(1)}:false"), repository.changes)
        assertEquals(progress(100, watched = true), vm.state.value.detail!!.progress)
        assertEquals(listOf<ItemMessage>(ItemMessage.Failed), messages)
    }

    @Test
    fun aQuietReloadThatFailsKeepsThePage() = runTest(dispatcher) {
        val repository = FakeDetails(film())
        val vm = model(repository, ulid(1))
        repository.details[ulid(1)] = ApiResult.Offline
        vm.onWatched()
        settle()
        assertTrue(vm.state.value.item is Loadable.Ready)
        assertEquals(WatchedMark.Full, vm.state.value.detail!!.watchedMark)
    }

    @Test
    fun aSeriesOpensTheSeasonOfItsNextEpisode() = runTest(dispatcher) {
        val repository = FakeDetails(series(), seasonDetail(1, watched = 2, resumeAt = 3), seasonDetail(2))
        val vm = model(repository, SERIES_ID)
        assertEquals(SEASON_1, vm.state.value.seasonId)
        assertEquals(3, (vm.state.value.episodes as Loadable.Ready).value.episodes!!.size)
        assertNull("a series without trailers asks for none", vm.state.value.trailer)

        vm.onSeason(SEASON_2)
        assertEquals(SEASON_2, vm.state.value.seasonId)
        assertEquals(Loadable.Loading, vm.state.value.episodes)
        settle()
        assertEquals(SEASON_2, (vm.state.value.episodes as Loadable.Ready).value.id)
        // Back to the first season: no second request.
        val asked = repository.asked.size
        vm.onSeason(SEASON_1)
        settle()
        assertEquals(asked, repository.asked.size)
        assertEquals(SEASON_1, (vm.state.value.episodes as Loadable.Ready).value.id)
    }

    @Test
    fun aSeasonThatFailsOffersRetryThroughItsChip() = runTest(dispatcher) {
        val repository = FakeDetails(series(), seasonDetail(1, watched = 2))
        repository.details[SEASON_2] = ApiResult.Offline
        val vm = model(repository, SERIES_ID)
        vm.onSeason(SEASON_2)
        settle()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.episodes)
        repository.details[SEASON_2] = ApiResult.Ok(seasonDetail(2))
        vm.onSeason(SEASON_2)
        settle()
        assertTrue(vm.state.value.episodes is Loadable.Ready)
    }

    @Test
    fun aHalfSeriesAsksThenMarksEverything() = runTest(dispatcher) {
        val repository = FakeDetails(series(watchedCount = 2), seasonDetail(1, watched = 2), seasonDetail(2))
        val vm = model(repository, SERIES_ID)
        vm.onWatched()
        assertEquals(ItemAsk.Watched("Quiet Ring", audio = false, count = 2, half = true, season = false), vm.state.value.ask)
        vm.onAskChoice(watched = true)
        assertNull(vm.state.value.ask)
        assertEquals(WatchedMark.Full, vm.state.value.detail!!.watchedMark)
        settle()
        assertEquals(listOf("watched:$SERIES_ID:true"), repository.changes)
    }

    @Test
    fun aWatchedSeriesAsksBeforeItsChecksGo() = runTest(dispatcher) {
        val repository = FakeDetails(series(watched = true, watchedCount = 6), seasonDetail(1, watched = 3), seasonDetail(2, watched = 3))
        val vm = model(repository, SERIES_ID)
        vm.onWatched()
        assertEquals(ItemAsk.Watched("Quiet Ring", audio = false, count = 6, half = false, season = false), vm.state.value.ask)
        vm.onDismissAsk()
        assertNull(vm.state.value.ask)
        assertTrue(repository.changes.isEmpty())
        vm.onWatched()
        vm.onAskChoice(watched = false)
        assertEquals(WatchedMark.None, vm.state.value.detail!!.watchedMark)
        settle()
        assertEquals(listOf("watched:$SERIES_ID:false"), repository.changes)
    }

    @Test
    fun aSeasonIsMarkedWatchedAndGoesBackOnARefusal() = runTest(dispatcher) {
        val repository = FakeDetails(series(), seasonDetail(1, watched = 2), seasonDetail(2))
        // Season 1 with two of three watched: its mark is half (the season's own page counts).
        val vm = model(repository, SERIES_ID)
        vm.onSeasonWatched()
        assertEquals(true, (vm.state.value.ask as ItemAsk.Watched).season)
        vm.onAskChoice(watched = true)
        val episodes = (vm.state.value.episodes as Loadable.Ready).value.episodes!!
        assertTrue(episodes.all { it.watchedMark == WatchedMark.Full })
        assertEquals(WatchedMark.Full, vm.state.value.detail!!.seasons!!.first().watchedMark)
        settle()
        assertEquals("watched:$SEASON_1:true", repository.changes.single())

        repository.watched = ApiResult.Offline
        repository.details[SERIES_ID] = ApiResult.Ok(series().copy(seasons = listOf(com.slouchcat.app.mobile.testing.seasonRef(1, watched = 3), com.slouchcat.app.mobile.testing.seasonRef(2))))
        repository.details[SEASON_1] = ApiResult.Ok(seasonDetail(1, watched = 3))
        vm.retry()
        settle()
        vm.onSeasonWatched()
        assertEquals(false, (vm.state.value.ask as ItemAsk.Watched).half)
        // Full: clearing asks first.
        vm.onAskChoice(watched = false)
        settle()
        assertEquals(WatchedMark.Full, vm.state.value.detail!!.seasons!!.first().watchedMark)
    }

    @Test
    fun recommendingAsksAndSaysHowLongTheTitleStays() = runTest(dispatcher) {
        val repository = FakeDetails(film())
        val vm = model(repository, ulid(1))
        val messages = messages(vm)
        vm.onRecommendAsk()
        assertEquals(ItemAsk.Recommend, vm.state.value.ask)
        vm.onRecommendConfirm()
        assertNull(vm.state.value.ask)
        assertTrue(vm.state.value.detail!!.recommendedByMe)
        settle()
        assertEquals(listOf<ItemMessage>(ItemMessage.Recommended(30)), messages)

        // Taking it back; a refusal puts the recommendation back.
        repository.unpick = ApiResult.Offline
        vm.onRecommendAsk()
        vm.onRecommendConfirm()
        assertFalse(vm.state.value.detail!!.recommendedByMe)
        settle()
        assertTrue(vm.state.value.detail!!.recommendedByMe)
        assertEquals(listOf("pick:${ulid(1)}", "unpick:${ulid(1)}"), repository.changes)
        assertEquals(ItemMessage.Failed, messages.last())
    }

    @Test
    fun aTitleTheOwnerMayNotRecommendAsksNothing() = runTest(dispatcher) {
        val vm = model(FakeDetails(film(canRecommend = false)), ulid(1))
        vm.onRecommendAsk()
        assertNull(vm.state.value.ask)
    }

    @Test
    fun foldsAndVersionsAreTheReadersChoice() = runTest(dispatcher) {
        val vm = model(FakeDetails(film()), ulid(1))
        assertFalse(vm.state.value.overviewOpen)
        assertFalse(vm.state.value.techOpen)
        vm.onToggleOverview()
        vm.onToggleTech()
        vm.onVersion("f2")
        assertTrue(vm.state.value.overviewOpen)
        assertTrue(vm.state.value.techOpen)
        assertEquals("f2", vm.state.value.versionId)
    }
}
