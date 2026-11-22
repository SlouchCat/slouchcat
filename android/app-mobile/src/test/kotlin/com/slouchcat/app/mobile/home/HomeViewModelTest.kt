package com.slouchcat.app.mobile.home

import com.slouchcat.app.mobile.testing.CHILD
import com.slouchcat.app.mobile.testing.FakeAccounts
import com.slouchcat.app.mobile.testing.FakeHome
import com.slouchcat.app.mobile.testing.FakeItems
import com.slouchcat.app.mobile.testing.FakeServers
import com.slouchcat.app.mobile.testing.FakeTokens
import com.slouchcat.app.mobile.testing.HTTP_SERVER
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.homeRow
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.StoredCredentials
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class HomeViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val spot = (1..3).map { itemRef(it, title = "Spot $it") }
    private val recent = (10..29).map { itemRef(it, title = "Recent $it") }
    private val page = HomePage(
        rows = listOf(
            homeRow(HomeRowKey.SPOTLIGHT, spot),
            homeRow(HomeRowKey.RECENT, recent),
            homeRow(HomeRowKey.GENRES, emptyList(), genres = listOf(GenreRef("Drama", 3))),
            homeRow(HomeRowKey.LIBRARY, recent.take(3), id = "library:4", libraryId = 4, libraryName = "Films"),
            homeRow(HomeRowKey.RADIO, emptyList()),
            homeRow(HomeRowKey.WATCHLIST, emptyList()),
        ),
        heroRow = HomeRowKey.SPOTLIGHT,
        heroStart = 1,
        heroIntervalSeconds = 9,
    )
    private val home = FakeHome(ApiResult.Ok(page))
    private val items = FakeItems()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun rows(vm: HomeViewModel) = (vm.state.value.rows as Loadable.Ready).value

    @Test
    fun theRowsOfTheServerInItsOrderWithTheStage() = runTest(dispatcher) {
        val vm = HomeViewModel(session(backgroundScope), home, items)
        settle()
        val rows = rows(vm)
        // The radio row waits for stage 7, an empty row is left out.
        assertEquals(listOf("spotlight", "recent", "genres", "library:4"), rows.map { it.id })
        val stage = rows[0] as HomeSection.Spotlight
        assertEquals(1, stage.start)
        assertEquals(9, stage.intervalSeconds)
        val library = rows[3] as HomeSection.Cards
        assertEquals(4L, library.libraryId)
        assertTrue("a row of three has no more", library.complete)
        assertEquals(PRIMARY, vm.state.value.profile)
    }

    @Test
    fun nothingOnTheStartPageIsEmpty() = runTest(dispatcher) {
        home.page = ApiResult.Ok(HomePage(emptyList()))
        val vm = HomeViewModel(session(backgroundScope), home, items)
        settle()
        assertEquals(Loadable.Empty, vm.state.value.rows)
    }

    @Test
    fun aFailureCanBeTriedAgain() = runTest(dispatcher) {
        home.page = ApiResult.Offline
        val vm = HomeViewModel(session(backgroundScope), home, items)
        settle()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.rows)
        home.page = ApiResult.Ok(page)
        vm.retry()
        settle()
        assertEquals(4, rows(vm).size)
    }

    @Test
    fun aRowCutByTheServerLoadsMoreAtItsEnd() = runTest(dispatcher) {
        home.lists[HomeListKey.RECENT] = ApiResult.Ok(recent + (30..39).map { itemRef(it) })
        val vm = HomeViewModel(session(backgroundScope), home, items)
        settle()
        val before = rows(vm)[1] as HomeSection.Cards
        assertFalse(before.complete)
        vm.onRowEnd("recent")
        settle()
        val after = rows(vm)[1] as HomeSection.Cards
        assertEquals(30, after.items.size)
        assertTrue(after.complete)
        vm.onRowEnd("recent")
        settle()
        assertEquals(listOf("home", "row:recent:20"), home.asked)
    }

    @Test
    fun aFailedNextPageOfARowShowsTheRetry() = runTest(dispatcher) {
        home.lists[HomeListKey.RECENT] = ApiResult.Offline
        val vm = HomeViewModel(session(backgroundScope), home, items)
        settle()
        vm.onRowEnd("recent")
        settle()
        assertEquals(MoreState.Failed, (rows(vm)[1] as HomeSection.Cards).more)
    }

    @Test
    fun pullToRefreshKeepsTheRowsWhenItFails() = runTest(dispatcher) {
        val vm = HomeViewModel(session(backgroundScope), home, items)
        settle()
        home.page = ApiResult.Offline
        home.gate = CompletableDeferred()
        vm.refresh()
        settle()
        assertTrue(vm.state.value.refreshing)
        assertEquals(4, rows(vm).size)
        home.gate!!.complete(Unit)
        settle()
        assertFalse(vm.state.value.refreshing)
        assertEquals(4, rows(vm).size)
    }

    @Test
    fun watchlistMarksAtOnceAndReturnsOnAFailure() = runTest(dispatcher) {
        val vm = HomeViewModel(session(backgroundScope), home, items)
        settle()
        val first = (rows(vm)[0] as HomeSection.Spotlight).items[0]
        val message = async { vm.messages.first() }
        vm.onWatchlist(first)
        settle()
        assertTrue((rows(vm)[0] as HomeSection.Spotlight).items[0].inWatchlist)
        assertEquals(HomeMessage.Watchlist("Spot 1", added = true), message.await())
        assertEquals(listOf(first.id to true), items.watchlistCalls)

        items.watchlist = ApiResult.Failed(500, null)
        val failed = async { vm.messages.first() }
        vm.onWatchlist((rows(vm)[0] as HomeSection.Spotlight).items[0])
        settle()
        assertEquals(HomeMessage.WatchlistFailed, failed.await())
        assertTrue("back to the state before", (rows(vm)[0] as HomeSection.Spotlight).items[0].inWatchlist)
    }

    /** A second tap while «Merken» is on its way sends nothing; the count moves once (code review m-4 of part a). */
    @Test
    fun aDoubleTapOnMerkenSendsOneRequest() = runTest(dispatcher) {
        val count = LiveCount().apply { set(3) }
        val vm = HomeViewModel(session(backgroundScope), home, items, count)
        settle()
        val first = (rows(vm)[0] as HomeSection.Spotlight).items[0]
        val gate = CompletableDeferred<Unit>()
        items.gate = gate
        vm.onWatchlist(first)
        vm.onWatchlist(first)
        settle()
        gate.complete(Unit)
        settle()
        assertEquals(listOf(first.id to true), items.watchlistCalls)
        assertEquals(4, count.count.value)
        // The next tap reads the page's state: off again.
        items.gate = null
        vm.onWatchlist(first)
        settle()
        assertEquals(listOf(first.id to true, first.id to false), items.watchlistCalls)
        assertEquals(3, count.count.value)
    }

    @Test
    fun anotherProfileLoadsAfresh() = runTest(dispatcher) {
        val accounts = FakeAccounts("viewer@example.test", PRIMARY)
        val s = SessionManager(FakeServers(HTTP_SERVER), FakeTokens(StoredCredentials("token", "device-1")), accounts).also { it.start(backgroundScope) }
        val vm = HomeViewModel(s, home, items)
        settle()
        assertEquals(listOf("home"), home.asked)
        home.page = ApiResult.Ok(HomePage(listOf(homeRow(HomeRowKey.RECENT, recent.take(2)))))
        s.profileChosen(CHILD)
        settle()
        assertEquals(CHILD, vm.state.value.profile)
        assertEquals(listOf("home", "home"), home.asked)
        assertEquals(listOf("recent"), rows(vm).map { it.id })
    }
}
