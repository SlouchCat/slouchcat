package com.slouchcat.app.mobile.connected

import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.toLoadable
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Health
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerRepository
import com.slouchcat.core.session.ServerStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test

/** The pattern of a loading page: Loading, then Ready, Empty or Failed; retry loads again. */
@OptIn(ExperimentalCoroutinesApi::class)
class ConnectedViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val address = "http://192.0.2.10:8096/".toHttpUrl()

    private class FakeStore(initial: SavedServer?) : ServerStore {
        override val server = MutableStateFlow(initial)
        override suspend fun save(server: SavedServer) = Unit
        override suspend fun clear() = Unit
    }

    private class FakeRepository(var answer: ApiResult<Health>) : ServerRepository {
        var calls = 0
        override suspend fun health(server: HttpUrl): ApiResult<Health> {
            calls++
            return answer
        }
    }

    private val health = Health(
        status = Health.Status.OK,
        version = "1.42.1",
        uptimeSeconds = 5,
        contractVersion = "1.42.0",
        serverKey = "c2VydmVyLWtleQ",
    )

    @Before
    fun setUp() = Dispatchers.setMain(dispatcher)

    @After
    fun tearDown() = Dispatchers.resetMain()

    @Test
    fun loadsTheStoredServerWithItsCurrentVersion() = runTest(dispatcher) {
        val model = ConnectedViewModel(FakeStore(SavedServer(address, "1.42.0")), FakeRepository(ApiResult.Ok(health)))
        assertEquals(Loadable.Loading, model.state.value.server)
        advanceUntilIdle()
        assertEquals(Loadable.Ready(ConnectedServer(address, "1.42.1")), model.state.value.server)
    }

    @Test
    fun withoutStoredServerItIsEmpty() = runTest(dispatcher) {
        val repository = FakeRepository(ApiResult.Ok(health))
        val model = ConnectedViewModel(FakeStore(null), repository)
        advanceUntilIdle()
        assertEquals(Loadable.Empty, model.state.value.server)
        assertEquals(0, repository.calls)
    }

    @Test
    fun aFailureCanBeRetried() = runTest(dispatcher) {
        val repository = FakeRepository(ApiResult.Offline)
        val model = ConnectedViewModel(FakeStore(SavedServer(address, "1.42.0")), repository)
        advanceUntilIdle()
        assertEquals(Loadable.Failed(LoadError.Offline), model.state.value.server)
        repository.answer = ApiResult.Ok(health)
        model.retry()
        assertEquals(Loadable.Loading, model.state.value.server)
        advanceUntilIdle()
        assertEquals(Loadable.Ready(ConnectedServer(address, "1.42.1")), model.state.value.server)
    }

    @Test
    fun everyApiResultHasItsLoadable() {
        val cases = mapOf(
            ApiResult.NotFound to Loadable.Failed(LoadError.NotFound),
            ApiResult.NotAllowed("client_not_allowed") to Loadable.Failed(LoadError.NotAllowed),
            ApiResult.Offline to Loadable.Failed(LoadError.Offline),
            ApiResult.SignedOut to Loadable.Failed(LoadError.Failed),
            ApiResult.AppTooOld to Loadable.Failed(LoadError.Failed),
            ApiResult.Failed(500, null) to Loadable.Failed(LoadError.Failed),
        )
        for ((result, loadable) in cases) {
            assertEquals(result.toString(), loadable, result.toLoadable())
        }
        assertEquals(Loadable.Empty, ApiResult.Ok(emptyList<Int>()).toLoadable { it.isEmpty() })
    }
}
