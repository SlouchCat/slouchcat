package com.slouchcat.app.mobile.signin

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.ViewModelStoreOwner
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.slouchcat.app.mobile.testing.FakeAuth
import com.slouchcat.app.mobile.testing.FakeKeys
import com.slouchcat.app.mobile.testing.FakeProbe
import com.slouchcat.app.mobile.testing.PHONE
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.AppLogin
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTP_SERVER
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
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

@OptIn(ExperimentalCoroutinesApi::class)
class SignInViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val auth = FakeAuth()
    private val tokens = FakeTokens(StoredCredentials(null, "device-1"))
    private val servers = FakeServers(HTTP_SERVER)
    private val keys = FakeKeys()
    private val saved = SavedStateHandle()
    private var probe = FakeProbe(ProbeResult.Found(ServerInfo(HTTP_SERVER.address, "1.42.1", "1.42.0", serverKey = "server-key")))

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.model(confirmKey: Boolean = false): Pair<SignInViewModel, SessionManager> {
        val session = session(backgroundScope, token = null, tokens = tokens, servers = servers)
        settle()
        val vm = SignInViewModel(saved, confirmKey, session, auth, probe, PHONE, keys)
        settle()
        return vm to session
    }

    private fun SignInViewModel.typeIn(email: String, password: String) {
        onEmailChange(email)
        onPasswordChange(password)
    }

    @Test
    fun theFieldsAreCheckedBeforeAnythingIsSent() = runTest(dispatcher) {
        val (vm, _) = model()
        vm.onSubmit()
        assertEquals(FieldError.EmailEmpty, vm.state.value.emailError)
        assertEquals(FieldError.PasswordEmpty, vm.state.value.passwordError)
        vm.typeIn("viewer@example", "secret")
        vm.onSubmit()
        assertEquals(FieldError.EmailInvalid, vm.state.value.emailError)
        settle()
        assertEquals(emptyList<String>(), auth.calls)
    }

    @Test
    fun aSignInStoresTheTokenAndForgetsThePassword() = runTest(dispatcher) {
        val (vm, session) = model()
        // The version the server reports is shown, not stored: it comes before any proof (L-4).
        assertEquals("1.42.1", vm.state.value.server?.version)
        assertEquals("1.42.0", servers.flow.value?.version)
        val epoch = session.epoch.value
        vm.typeIn(" $EMAIL ", "secret-password")
        vm.onSubmit()
        assertTrue(vm.state.value.busy)
        settle()
        assertEquals(listOf("login:$EMAIL:phone"), auth.calls)
        assertEquals("secret-password", auth.lastPassword)
        assertEquals("device-1", auth.lastDeviceId)
        assertEquals(StoredCredentials("new-token", "device-2"), tokens.stored)
        assertEquals(SessionState.ChooseProfile(HTTP_SERVER, EMAIL), session.state.value)
        assertEquals("a new session epoch opens the next page", epoch + 1, session.epoch.value)
        assertEquals("", vm.state.value.password)
        assertFalse("the password never reaches the saved state", saved.keys().any { saved.get<Any>(it) == "secret-password" })
        assertFalse("the password reaches toString", vm.state.value.copy(password = "secret-password").toString().contains("secret-password"))
    }

    @Test
    fun aWrongPasswordAndTooManyAttemptsAreSaid() = runTest(dispatcher) {
        val (vm, session) = model()
        auth.login = ApiResult.Failed(401, "invalid_credentials")
        vm.typeIn(EMAIL, "wrong")
        vm.onSubmit()
        settle()
        assertEquals(SignInError.InvalidCredentials, vm.state.value.error)
        assertEquals("", vm.state.value.password)
        assertTrue(session.state.value is SessionState.SignedOut)

        auth.login = ApiResult.Failed(429, "too_many_attempts", 42)
        vm.onPasswordChange("again")
        vm.onSubmit()
        settle()
        assertEquals(SignInError.TooManyAttempts(42), vm.state.value.error)
    }

    @Test
    fun noAnswerAnOldAppAndAnUnconfirmedServerAreSaid() = runTest(dispatcher) {
        val (vm, _) = model()
        auth.login = ApiResult.Offline
        vm.typeIn(EMAIL, "secret")
        vm.onSubmit()
        settle()
        assertEquals(SignInError.Unreachable, vm.state.value.error)
        auth.login = ApiResult.AppTooOld
        vm.onPasswordChange("secret")
        vm.onSubmit()
        settle()
        assertEquals(SignInError.AppTooOld, vm.state.value.error)
        auth.login = ApiResult.Failed(null, ApiCaller.SERVER_UNCONFIRMED)
        vm.onPasswordChange("secret")
        vm.onSubmit()
        settle()
        assertEquals(SignInError.ServerUnconfirmed, vm.state.value.error)
    }

    @Test
    fun aServerThatIsTooOldGetsNoPassword() = runTest(dispatcher) {
        probe = FakeProbe(ProbeResult.TooOld("1.41.3", "1.42.0"))
        val (vm, _) = model()
        assertEquals(SignInError.ServerTooOld("1.41.3", "1.42.0"), vm.state.value.error)
        vm.typeIn(EMAIL, "secret")
        vm.onSubmit()
        settle()
        assertEquals(emptyList<String>(), auth.calls)
    }

    @Test
    fun aNewKeyIsOnlyUsedAfterTheOwnNetworkIsStated() = runTest(dispatcher) {
        keys.newKey.value = "new-key"
        val (vm, _) = model()
        assertEquals("new-key", vm.state.value.newKey)
        assertFalse(vm.state.value.confirmKey)
        // Without the statement the sign-in does not go to the new key.
        vm.onCheckKey()
        assertTrue(vm.state.value.keyCheck)
        vm.onConfirmKey()
        assertFalse(vm.state.value.confirmKey)
        vm.onOwnNetwork(true)
        vm.onConfirmKey()
        assertTrue(vm.state.value.confirmKey)
        assertFalse(vm.state.value.keyCheck)

        auth.login = ApiResult.Failed(401, "invalid_credentials")
        vm.typeIn(EMAIL, "wrong")
        vm.onSubmit()
        settle()
        assertEquals(listOf("begin:new-key", "finish:false"), keys.log)

        auth.login = FakeAuth().login
        vm.onPasswordChange("right")
        vm.onSubmit()
        settle()
        assertEquals(listOf("begin:new-key", "finish:false", "begin:new-key", "finish:true"), keys.log)
        assertNull(vm.state.value.error)
    }

    @Test
    fun withoutANewKeyTheRouteToConfirmIsAnOrdinarySignIn() = runTest(dispatcher) {
        val (vm, _) = model(confirmKey = true)
        assertFalse(vm.state.value.confirmKey)
        vm.typeIn(EMAIL, "secret")
        vm.onSubmit()
        settle()
        assertEquals(emptyList<String>(), keys.log)
    }

    @Test
    fun aPageThatGoesAwayWhileConfirmingGivesTheCredentialsBack() = runTest(dispatcher) {
        keys.newKey.value = "new-key"
        val hanging = CompletableDeferred<ApiResult<AppLogin>>()
        val slow = object : com.slouchcat.core.session.AuthRepository by auth {
            override suspend fun appLogin(server: okhttp3.HttpUrl, email: String, password: String, device: com.slouchcat.core.session.DeviceDescription, deviceId: String?) =
                hanging.await()
        }
        val session = session(backgroundScope, token = null, tokens = tokens, servers = servers)
        settle()
        val store = ViewModelStore()
        val owner = object : ViewModelStoreOwner { override val viewModelStore = store }
        val vm = ViewModelProvider.create(owner, viewModelFactory { initializer { SignInViewModel(saved, true, session, slow, probe, PHONE, keys) } })[SignInViewModel::class]
        settle()
        vm.typeIn(EMAIL, "secret")
        vm.onSubmit()
        settle()
        assertTrue(keys.suppressed)
        // Back on the only page: the ViewModel is cleared while appLogin hangs (code review M-1).
        store.clear()
        settle()
        assertFalse("the credentials stay suppressed", keys.suppressed)
        assertEquals(listOf("begin:new-key", "finish:false"), keys.log)
    }

    /**
     * The same abort with the app's real confirmation (GateKeyConfirmation over the SessionManager):
     * after an appLogin that never answered, the session sends its credentials again
     * (`credentialsSuppressed` reset) and the gate forgets the candidate key.
     */
    @Test
    fun anAbortedSignInResetsTheSuppressedCredentialsOfTheSession() = runTest(dispatcher) {
        val hanging = CompletableDeferred<ApiResult<AppLogin>>()
        val slow = object : com.slouchcat.core.session.AuthRepository by auth {
            override suspend fun appLogin(server: okhttp3.HttpUrl, email: String, password: String, device: com.slouchcat.core.session.DeviceDescription, deviceId: String?) =
                hanging.await()
        }
        val session = session(backgroundScope, token = null, tokens = tokens, servers = servers)
        settle()
        val gate = com.slouchcat.core.api.ServerIdentityGate(session, { 0L }, { _, _ -> null })
        val real = com.slouchcat.app.mobile.common.GateKeyConfirmation(gate, session)
        // The gate met another key under the address (what it holds in `newKey` after a failed proof).
        val presented = object : com.slouchcat.app.mobile.common.KeyConfirmation by real {
            override val newKey = kotlinx.coroutines.flow.MutableStateFlow<String?>("new-key")
        }
        val store = ViewModelStore()
        val owner = object : ViewModelStoreOwner { override val viewModelStore = store }
        val vm = ViewModelProvider.create(owner, viewModelFactory { initializer { SignInViewModel(saved, true, session, slow, probe, PHONE, presented) } })[SignInViewModel::class]
        settle()
        vm.typeIn(EMAIL, "secret")
        vm.onSubmit()
        settle()
        assertTrue("suppressed while confirming", session.credentialsSuppressed)
        assertEquals("new-key", gate.candidateKey)
        store.clear()
        settle()
        assertFalse("the credentials stay suppressed", session.credentialsSuppressed)
        assertNull(gate.candidateKey)
    }
}
