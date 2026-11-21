package com.slouchcat.app.mobile.signin

import androidx.lifecycle.SavedStateHandle
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.FakeAuth
import com.slouchcat.app.mobile.testing.FakeKeys
import com.slouchcat.app.mobile.testing.FakeProbe
import com.slouchcat.app.mobile.testing.FakeServers
import com.slouchcat.app.mobile.testing.FakeTokens
import com.slouchcat.app.mobile.testing.HTTP_SERVER
import com.slouchcat.app.mobile.testing.PHONE
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
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

    private fun TestScope.model(confirmKey: Boolean = false): Pair<SignInViewModel, com.slouchcat.core.session.SessionManager> {
        val session = session(backgroundScope, token = null, tokens = tokens, servers = servers)
        settle()
        val vm = SignInViewModel(saved, confirmKey, session, auth, probe, servers, PHONE, keys)
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
        // The server reported a newer version: the page and the store follow.
        assertEquals("1.42.1", vm.state.value.server?.version)
        assertEquals("1.42.1", servers.flow.value?.version)
        val done = backgroundScope.launch { vm.done.first() }
        vm.typeIn(" $EMAIL ", "secret-password")
        vm.onSubmit()
        assertTrue(vm.state.value.busy)
        settle()
        assertTrue(done.isCompleted)
        assertEquals(listOf("login:$EMAIL:phone"), auth.calls)
        assertEquals("secret-password", auth.lastPassword)
        // The device id of the former sign-in goes along.
        assertEquals("device-1", auth.lastDeviceId)
        assertEquals(StoredCredentials("new-token", "device-2"), tokens.stored)
        assertEquals(SessionState.ChooseProfile(servers.flow.value!!, EMAIL), session.state.value)
        assertEquals("", vm.state.value.password)
        assertFalse("the password never reaches the saved state", saved.keys().any { saved.get<Any>(it) == "secret-password" })
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
    fun noAnswerAndAnOldAppAreSaid() = runTest(dispatcher) {
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
        assertTrue(vm.state.value.error is SignInError.ServerTooOld)
    }

    @Test
    fun aNewKeyCountsOnlyAfterTheServerAcceptedThePassword() = runTest(dispatcher) {
        keys.presentedKey = "new-key"
        val (vm, _) = model(confirmKey = true)
        assertTrue(vm.state.value.confirmKey)
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
    fun withoutANewKeyTheConfirmationIsAnOrdinarySignIn() = runTest(dispatcher) {
        val (vm, _) = model(confirmKey = true)
        assertFalse(vm.state.value.confirmKey)
        vm.typeIn(EMAIL, "secret")
        vm.onSubmit()
        settle()
        assertEquals(emptyList<String>(), keys.log)
    }
}
