package com.slouchcat.app.mobile.lists

import com.slouchcat.app.mobile.browse.ListRoute
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.testing.FakeHome
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.HomeListKey
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class ListViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val home = FakeHome()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    @Test
    fun aRowOfTheStartPageInPages() = runTest(dispatcher) {
        home.lists[HomeListKey.RELEASED] = ApiResult.Ok((1..60).map { itemRef(it) })
        val vm = ListViewModel(session(backgroundScope), home, "released")
        settle()
        assertEquals(ListKind.Row(HomeListKey.RELEASED), vm.kind)
        assertEquals(50, vm.state.value.loaded.size)
        vm.onNearEnd()
        settle()
        assertEquals(60, vm.state.value.loaded.size)
        assertEquals(listOf("row:released:0", "row:released:50"), home.asked)
    }

    @Test
    fun thePicks() = runTest(dispatcher) {
        home.picks = ApiResult.Ok(listOf(itemRef(1), itemRef(2)))
        val vm = ListViewModel(session(backgroundScope), home, ListRoute.PICKS)
        settle()
        assertEquals(ListKind.Picks, vm.kind)
        assertEquals(2, vm.state.value.total)
    }

    @Test
    fun emptyGoneAndUnknown() = runTest(dispatcher) {
        home.lists[HomeListKey.RECENT] = ApiResult.Ok(emptyList())
        val empty = ListViewModel(session(backgroundScope), home, "recent")
        // The spotlight switched off: its list answers 404.
        val gone = ListViewModel(session(backgroundScope), home, "spotlight")
        val unknown = ListViewModel(session(backgroundScope), home, "nonsense")
        settle()
        assertEquals(Loadable.Empty, empty.state.value.items)
        assertEquals(Loadable.Failed(LoadError.NotFound), gone.state.value.items)
        assertEquals(null, unknown.kind)
        assertEquals(Loadable.Failed(LoadError.NotFound), unknown.state.value.items)
    }

    @Test
    fun aFailureCanBeTriedAgain() = runTest(dispatcher) {
        home.lists[HomeListKey.CONTINUE] = ApiResult.Offline
        val vm = ListViewModel(session(backgroundScope), home, "continue")
        settle()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.items)
        home.lists[HomeListKey.CONTINUE] = ApiResult.Ok(listOf(itemRef(1)))
        vm.retry()
        settle()
        assertEquals(1, vm.state.value.loaded.size)
    }
}
