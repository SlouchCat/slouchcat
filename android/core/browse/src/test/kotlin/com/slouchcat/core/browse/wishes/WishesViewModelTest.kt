package com.slouchcat.core.browse.wishes

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.api.generated.models.WishSeasonOffer
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.browse.testing.FakeWishes
import com.slouchcat.core.browse.testing.externalRef
import com.slouchcat.core.browse.testing.externalTitle
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.browse.testing.rights
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.browse.testing.ulid
import com.slouchcat.core.browse.testing.wish
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.data.WishFilter
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
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
class WishesViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    private companion object {
        /** The pause of the TV's search (any value; the model takes it as given). */
        const val PAUSE = 400L
    }

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.area(repository: FakeWishes, count: LiveCount = LiveCount(), tab: WishesTab = WishesTab.Discover, query: String? = null): WishesViewModel {
        val s = session(backgroundScope)
        settle()
        val vm = WishesViewModel(s, repository, count, tab, query)
        settle()
        return vm
    }

    @Test
    fun theAreaLoadsRightsRowsAndTheOwnWishes() = runTest(dispatcher) {
        val repository = FakeWishes(wishList = mutableListOf(wish(1, "Open"), wish(2, "Done", WishState.AVAILABLE, item = itemRef(5))))
        val count = LiveCount()
        val vm = area(repository, count)
        val state = vm.state.value
        assertTrue(state.discover)
        assertEquals(WishesTab.Discover, state.shownTab)
        // The genre row has no tiles: only the title row stays.
        val rows = (state.rows as Loadable.Ready).value
        assertEquals(1, rows.size)
        assertEquals(listOf("Trend One", "Trend Two"), rows.single().cards.map { it.title })
        assertTrue(rows.single().cards[1].requested)
        assertEquals(listOf("Open", "Done"), (state.mine.items as Loadable.Ready).value.map { it.title })
        assertEquals(ulid(5), state.mine.loaded[1].itemId)
        assertEquals(1, count.count.value)
        assertEquals(2, state.total)
    }

    @Test
    fun wishesOffOrSeerrMissingIsAnEmptyArea() = runTest(dispatcher) {
        val off = area(FakeWishes(rightsAnswer = ApiResult.Ok(rights(enabled = false))))
        assertEquals(Loadable.Empty, off.state.value.rights)
        val old = area(FakeWishes(rightsAnswer = ApiResult.Ok(null)))
        assertEquals(Loadable.Empty, old.state.value.rights)
        val offline = area(FakeWishes().apply { failure = ApiResult.Offline })
        assertEquals(Loadable.Failed(LoadError.Offline), offline.state.value.rights)
    }

    @Test
    fun withoutDiscoverOnlyTheOwnWishesShow() = runTest(dispatcher) {
        val repository = FakeWishes(rightsAnswer = ApiResult.Ok(rights(canRequest = false, discover = false)))
        val vm = area(repository)
        assertEquals(WishesTab.Mine, vm.state.value.shownTab)
        assertFalse("discover" in repository.calls)
    }

    @Test
    fun seerrOutOfReachShowsTheProblemOfTheRows() = runTest(dispatcher) {
        val repository = FakeWishes().apply { discoverFailure = ApiResult.Failed(503, "seerr_unreachable") }
        val vm = area(repository)
        assertEquals(Loadable.Failed(LoadError.Failed), vm.state.value.rows)
        // The own wishes still load.
        assertEquals(Loadable.Empty, vm.state.value.mine.items)
    }

    @Test
    fun theSearchNeedsTwoCharactersAndShowsItsHits() = runTest(dispatcher) {
        val repository = FakeWishes()
        val vm = area(repository)
        vm.onText("a")
        vm.onSubmit()
        settle()
        assertNull(vm.state.value.results)
        vm.onText("found")
        vm.onSubmit()
        settle()
        assertEquals(listOf("Found Seven", "Found Eight"), (vm.state.value.results!!.items as Loadable.Ready).value.map { it.title })
        assertTrue("search:found:1" in repository.calls)
        vm.onClear()
        settle()
        assertNull(vm.state.value.results)
    }

    @Test
    fun aQueryFromTheSearchPageSearchesAtOnce() = runTest(dispatcher) {
        val repository = FakeWishes()
        val vm = area(repository, query = "seven")
        assertEquals("seven", vm.state.value.text)
        assertTrue("search:seven:1" in repository.calls)
    }

    @Test
    fun theTvSearchesByItselfAPauseAfterTheLastKey() = runTest(dispatcher) {
        val repository = FakeWishes()
        val s = session(backgroundScope)
        settle()
        val vm = WishesViewModel(s, repository, LiveCount(), searchPauseMs = PAUSE)
        settle()
        vm.onText("fo")
        advanceTimeBy(PAUSE / 2)
        vm.onText("found")
        advanceTimeBy(PAUSE - 1)
        assertFalse(repository.calls.any { it.startsWith("search:") })
        advanceTimeBy(2)
        settle()
        // One search for the last text, none for «fo».
        assertEquals(listOf("search:found:1"), repository.calls.filter { it.startsWith("search:") })
        assertEquals(listOf("Found Seven", "Found Eight"), (vm.state.value.results!!.items as Loadable.Ready).value.map { it.title })
        // The same text again (a key and its deletion) does not ask Seerr once more.
        vm.onText("found ")
        vm.onText("found")
        settle()
        assertEquals(1, repository.calls.count { it.startsWith("search:") })
        // The phone's model (no pause) waits for its search key.
        val phone = area(FakeWishes())
        phone.onText("found")
        settle()
        assertNull(phone.state.value.results)
    }

    @Test
    fun aKeyAndItsDeletionDoNotSearchAgainAndALateAnswerIsDropped() = runTest(dispatcher) {
        val repository = FakeWishes()
        val s = session(backgroundScope)
        settle()
        val vm = WishesViewModel(s, repository, LiveCount(), searchPauseMs = PAUSE)
        settle()
        vm.onText("found")
        advanceTimeBy(PAUSE + 1)
        settle()
        // «x» and its deletion within the pause: the planned search goes, nothing is asked again.
        vm.onText("foundx")
        vm.onText("found")
        advanceTimeBy(PAUSE * 2)
        settle()
        assertEquals(1, repository.calls.count { it.startsWith("search:") })
        // «fo» answers slowly, «found» at once: the hits of «found» stay.
        repository.searchDelayMs = { if (it == "fo") PAUSE * 10 else 0 }
        repository.searchHits = listOf(externalRef(9, "Found Nine"))
        vm.onText("fo")
        advanceTimeBy(PAUSE + 1)
        vm.onText("found!")
        advanceTimeBy(PAUSE * 20)
        settle()
        assertEquals(listOf("Found Nine"), (vm.state.value.results!!.items as Loadable.Ready).value.map { it.title })
        assertEquals("found!", vm.state.value.text)
    }

    @Test
    fun aRowOfDiscoverLoadsItsNextPagesUntilSeerrHasNoMore() = runTest(dispatcher) {
        val repository = FakeWishes()
        repository.rows[0] = repository.rows[0].copy(nextPage = 2)
        repository.rowPages[com.slouchcat.core.api.generated.models.DiscoverRowKey.TRENDING] = listOf(listOf(externalRef(3, "Trend Three"), externalRef(1, "Trend One")))
        val vm = area(repository)
        val key = com.slouchcat.core.api.generated.models.DiscoverRowKey.TRENDING
        vm.onMoreRow(key)
        settle()
        val row = (vm.state.value.rows as Loadable.Ready).value.single()
        // The new tile only once (Seerr's pages overlap), and page 2 was the last.
        assertEquals(listOf("Trend One", "Trend Two", "Trend Three"), row.cards.map { it.title })
        assertNull(row.nextPage)
        vm.onMoreRow(key)
        settle()
        assertEquals(1, repository.calls.count { it.startsWith("row:") })
    }

    @Test
    fun theTvReadsTheLimitWithTheOwnWishes() = runTest(dispatcher) {
        val s = session(backgroundScope)
        settle()
        val tv = WishesViewModel(s, FakeWishes(), LiveCount(), WishesTab.Mine, withQuota = true)
        settle()
        assertEquals(5, tv.state.value.quota?.movie?.limit)
        assertEquals(1, tv.state.value.quota?.movie?.used)
        assertNull(area(FakeWishes()).state.value.quota)
    }

    @Test
    fun theFilterReadsTheListAgain() = runTest(dispatcher) {
        val repository = FakeWishes(wishList = mutableListOf(wish(1, "Open"), wish(2, "Done", WishState.AVAILABLE), wish(3, "Gone", WishState.DECLINED)))
        val vm = area(repository, tab = WishesTab.Mine)
        vm.onFilter(WishFilter.Closed)
        settle()
        assertEquals(listOf("Gone"), vm.state.value.mine.loaded.map { it.title })
        vm.onFilter(WishFilter.Available)
        settle()
        assertEquals(listOf("Done"), vm.state.value.mine.loaded.map { it.title })
    }

    @Test
    fun withdrawingAsksThenWithdrawsAndTellsIt() = runTest(dispatcher) {
        val repository = FakeWishes(wishList = mutableListOf(wish(1, "Open")))
        val count = LiveCount()
        val vm = area(repository, count, WishesTab.Mine)
        val messages = mutableListOf<WishesMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        val card = vm.state.value.mine.loaded.single()
        vm.onAskWithdraw(card)
        settle()
        assertEquals(card, vm.state.value.withdrawing)
        vm.onConfirmWithdraw()
        settle()
        assertTrue("withdraw:1" in repository.calls)
        assertEquals(listOf<WishesMessage>(WishesMessage.Withdrawn("Open", joined = false)), messages)
        assertEquals(WishState.WITHDRAWN, vm.state.value.mine.loaded.single().state)
        assertEquals(0, count.count.value)
    }

    @Test
    fun aWishSeerrApprovedMeanwhileCannotBeWithdrawn() = runTest(dispatcher) {
        val repository = FakeWishes(wishList = mutableListOf(wish(1, "Open"))).apply { changeFailure = ApiResult.Failed(409, "wish_not_withdrawable") }
        val vm = area(repository, tab = WishesTab.Mine)
        val messages = mutableListOf<WishesMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        vm.onAskWithdraw(vm.state.value.mine.loaded.single())
        vm.onConfirmWithdraw()
        settle()
        assertEquals(listOf<WishesMessage>(WishesMessage.Failed("wish_not_withdrawable")), messages)
        assertTrue(vm.state.value.busy.isEmpty())
    }
}

@OptIn(ExperimentalCoroutinesApi::class)
class WishTitleViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.page(repository: FakeWishes, media: ExternalMedia, tmdbId: Long, count: LiveCount = LiveCount()): WishTitleViewModel {
        val s = session(backgroundScope)
        settle()
        val vm = WishTitleViewModel(s, repository, count, media, tmdbId)
        settle()
        return vm
    }

    @Test
    fun aFilmIsRequestedAndThePageShowsTheWish() = runTest(dispatcher) {
        val repository = FakeWishes(titles = mutableMapOf(79L to externalTitle(79, "1984")))
        val count = LiveCount().apply { set(0) }
        val vm = page(repository, ExternalMedia.MOVIE, 79, count)
        val messages = mutableListOf<WishTitleMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        assertTrue((vm.state.value.title as Loadable.Ready).value.canRequest)
        vm.onRequest()
        settle()
        assertEquals(WishSheet.Request, vm.state.value.sheet)
        assertEquals(Loadable.Empty, vm.state.value.seasons)
        vm.onSubmit()
        settle()
        val request = repository.created.single()
        assertEquals(WishCreate(ExternalMedia.MOVIE, 79, source = WishCreate.Source.DETAIL), request)
        assertNull(vm.state.value.sheet)
        assertEquals(listOf<WishTitleMessage>(WishTitleMessage.Requested(joined = false)), messages)
        assertEquals(WishState.PENDING, (vm.state.value.title as Loadable.Ready).value.wish?.state)
        assertEquals(1, count.count.value)
    }

    @Test
    fun aSeriesChoosesItsSeasons() = runTest(dispatcher) {
        val repository = FakeWishes(titles = mutableMapOf(106L to externalTitle(106, "Alexander", ExternalMedia.SHOW)))
        val vm = page(repository, ExternalMedia.SHOW, 106)
        vm.onRequest()
        settle()
        // Season 1 is available: it cannot be chosen. 2 is missing, 3 requested by another profile.
        assertEquals(listOf(2, 3), vm.state.value.choosable.map { it.number })
        assertEquals(setOf(2, 3), vm.state.value.chosen)
        assertTrue(vm.state.value.all)
        assertEquals(1, vm.state.value.fresh)
        vm.onSeason(3)
        assertEquals(setOf(2), vm.state.value.chosen)
        vm.onSubmit()
        settle()
        assertEquals(listOf(2), repository.created.single().seasons)
        assertNull(repository.created.single().allSeasons)
    }

    @Test
    fun everySeasonIsAllSeasons() = runTest(dispatcher) {
        val repository = FakeWishes(
            titles = mutableMapOf(106L to externalTitle(106, "Alexander", ExternalMedia.SHOW)),
            seasonOffers = listOf(WishSeasonOffer(1, 8, WishSeasonOffer.State.MISSING), WishSeasonOffer(2, 8, WishSeasonOffer.State.MISSING)),
        )
        val vm = page(repository, ExternalMedia.SHOW, 106)
        vm.onRequest()
        settle()
        vm.onAllSeasons()
        assertTrue(vm.state.value.chosen.isEmpty())
        vm.onSubmit()
        settle()
        assertTrue(repository.created.isEmpty())
        vm.onAllSeasons()
        vm.onSubmit()
        settle()
        assertEquals(true, repository.created.single().allSeasons)
        assertNull(repository.created.single().seasons)
    }

    @Test
    fun aRefusedRequestStaysInTheSheetWithItsCode() = runTest(dispatcher) {
        val repository = FakeWishes(titles = mutableMapOf(79L to externalTitle(79, "1984"))).apply { changeFailure = ApiResult.Failed(429, "quota_exceeded") }
        val vm = page(repository, ExternalMedia.MOVIE, 79)
        vm.onRequest()
        settle()
        vm.onSubmit()
        settle()
        assertEquals(WishSheet.Request, vm.state.value.sheet)
        assertEquals("quota_exceeded", vm.state.value.error)
        assertFalse(vm.state.value.sending)
    }

    @Test
    fun theOwnWishIsWithdrawn() = runTest(dispatcher) {
        val repository = FakeWishes(titles = mutableMapOf(1079L to externalTitle(1079, "1984")), wishList = mutableListOf(wish(79, "1984")))
        val count = LiveCount().apply { set(1) }
        val vm = page(repository, ExternalMedia.MOVIE, 1079, count)
        assertEquals(79L, (vm.state.value.title as Loadable.Ready).value.wish?.id)
        vm.onWithdraw()
        assertEquals(WishSheet.Withdraw, vm.state.value.sheet)
        vm.onConfirmWithdraw()
        settle()
        assertTrue("withdraw:79" in repository.calls)
        assertNull((vm.state.value.title as Loadable.Ready).value.wish)
        assertEquals(0, count.count.value)
    }

    @Test
    fun withdrawingNeedsTheServersWord() = runTest(dispatcher) {
        // Approved in Seerr (processing): no «Zurückziehen» (code review M-1).
        val approved = FakeWishes(titles = mutableMapOf(1079L to externalTitle(1079, "1984")), wishList = mutableListOf(wish(79, "1984", WishState.PROCESSING)))
        val vm = page(approved, ExternalMedia.MOVIE, 1079)
        assertEquals(79L, vm.state.value.ownWish?.id)
        assertFalse(vm.state.value.canWithdraw)
        vm.onWithdraw()
        settle()
        assertNull(vm.state.value.sheet)
        // Still open for an approval: it can go.
        val pending = page(FakeWishes(titles = mutableMapOf(1079L to externalTitle(1079, "1984")), wishList = mutableListOf(wish(79, "1984"))), ExternalMedia.MOVIE, 1079)
        assertTrue(pending.state.value.canWithdraw)
    }

    @Test
    fun aTitleInTheLibraryMeanwhileLeadsThere() = runTest(dispatcher) {
        val repository = FakeWishes().apply {
            failure = ApiResult.Failed(409, "already_available").apply {
                problem = kotlinx.serialization.json.buildJsonObject {
                    put("code", kotlinx.serialization.json.JsonPrimitive("already_available"))
                    put("item", kotlinx.serialization.json.buildJsonObject { put("id", kotlinx.serialization.json.JsonPrimitive("01JITEM")) })
                }
            }
        }
        val vm = page(repository, ExternalMedia.MOVIE, 603)
        assertEquals("01JITEM", vm.state.value.inLibrary)
    }

    @Test
    fun aTitleTheProfileCannotSeeIsNotFound() = runTest(dispatcher) {
        val vm = page(FakeWishes(), ExternalMedia.MOVIE, 5)
        assertEquals(Loadable.Failed(LoadError.NotFound), vm.state.value.title)
    }
}

@OptIn(ExperimentalCoroutinesApi::class)
class MissingTitlesViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    @Test
    fun theHitsFollowTheQueryOnlyWhenTheProfileMayRequest() = runTest(dispatcher) {
        val repository = FakeWishes()
        val s = session(backgroundScope)
        settle()
        val vm = MissingTitlesViewModel(s, repository)
        settle()
        vm.onQuery("s")
        settle()
        assertNull(vm.state.value.hits)
        vm.onQuery("seven")
        advanceTimeBy(100)
        assertEquals(Loadable.Loading, vm.state.value.hits)
        settle()
        assertEquals(listOf("Found Seven"), (vm.state.value.hits as Loadable.Ready).value.map { it.title })

        val refused = FakeWishes(rightsAnswer = ApiResult.Ok(rights(canRequest = false)))
        val other = MissingTitlesViewModel(s, refused)
        settle()
        other.onQuery("seven")
        settle()
        assertNull(other.state.value.hits)
        assertFalse(refused.calls.any { it.startsWith("missing") })
    }
}
