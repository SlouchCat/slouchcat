package com.slouchcat.app.mobile.account

import com.slouchcat.app.mobile.testing.FakeAuth
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.sessionInfo
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class AccountViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val auth = FakeAuth()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.model(): Pair<AccountViewModel, SessionManager> {
        val s = session(backgroundScope)
        settle()
        val vm = AccountViewModel(s, auth)
        settle()
        return vm to s
    }

    @Test
    fun theNewPasswordIsCheckedBeforeItIsSent() = runTest(dispatcher) {
        val (vm, _) = model()
        assertEquals(EMAIL, vm.state.value.email)
        vm.onPasswordAsk()
        vm.onSave()
        assertEquals(AccountFieldError.CurrentEmpty, vm.state.value.form.errors[AccountViewModel.CURRENT])
        assertEquals(AccountFieldError.TooShort, vm.state.value.form.errors[AccountViewModel.NEW_PASSWORD])
        vm.onField(AccountViewModel.CURRENT, "old-password-1")
        vm.onField(AccountViewModel.NEW_PASSWORD, "old-password-1")
        vm.onSave()
        assertEquals(AccountFieldError.SamePassword, vm.state.value.form.errors[AccountViewModel.NEW_PASSWORD])
        vm.onField(AccountViewModel.NEW_PASSWORD, "new-password-1")
        vm.onField(AccountViewModel.REPEAT, "new-password-2")
        vm.onSave()
        assertEquals(AccountFieldError.Mismatch, vm.state.value.form.errors[AccountViewModel.REPEAT])
        settle()
        assertEquals(emptyList<String>(), auth.calls)
    }

    @Test
    fun aNewPasswordIsSavedAndSaid() = runTest(dispatcher) {
        val (vm, _) = model()
        vm.onPasswordAsk()
        vm.onField(AccountViewModel.CURRENT, "old-password-1")
        vm.onField(AccountViewModel.NEW_PASSWORD, "new-password-1")
        vm.onField(AccountViewModel.REPEAT, "new-password-1")
        val message = async { vm.messages.first() }
        vm.onSave()
        settle()
        assertEquals(AccountMessage.PasswordChanged, message.await())
        assertEquals(listOf("update:-:new"), auth.calls)
        assertNull(vm.state.value.sheet)
        assertEquals("", vm.state.value.form.newPassword)
    }

    @Test
    fun aWrongCurrentPasswordIsMarkedAndNoSessionIsLost() = runTest(dispatcher) {
        val (vm, s) = model()
        auth.update = ApiResult.Failed(401, "invalid_credentials")
        vm.onEmailAsk()
        vm.onField(AccountViewModel.NEW_EMAIL, "other@example.test")
        vm.onField(AccountViewModel.CURRENT, "wrong")
        vm.onSave()
        settle()
        assertEquals(AccountFieldError.WrongPassword, vm.state.value.form.errors[AccountViewModel.CURRENT])
        assertEquals("", vm.state.value.form.current)
        assertEquals(AccountSheet.Email, vm.state.value.sheet)
        assertEquals(SessionState.Ready::class, s.state.value::class)
    }

    @Test
    fun aNewEmailFollowsIntoTheSession() = runTest(dispatcher) {
        val (vm, s) = model()
        auth.update = ApiResult.Ok(sessionInfo("other@example.test"))
        vm.onEmailAsk()
        vm.onField(AccountViewModel.NEW_EMAIL, EMAIL.uppercase())
        vm.onField(AccountViewModel.CURRENT, "secret")
        vm.onSave()
        assertEquals(AccountFieldError.SameEmail, vm.state.value.form.errors[AccountViewModel.NEW_EMAIL])
        vm.onField(AccountViewModel.NEW_EMAIL, "other@example.test")
        val message = async { vm.messages.first() }
        vm.onSave()
        settle()
        assertEquals(AccountMessage.EmailChanged, message.await())
        assertEquals("other@example.test", (s.state.value as SessionState.Ready).email)
        assertEquals("other@example.test", vm.state.value.email)
    }

    @Test
    fun aTakenEmailAndThePauseAreSaid() = runTest(dispatcher) {
        val (vm, _) = model()
        auth.update = ApiResult.Failed(409, "email_taken")
        vm.onEmailAsk()
        vm.onField(AccountViewModel.NEW_EMAIL, "other@example.test")
        vm.onField(AccountViewModel.CURRENT, "secret")
        vm.onSave()
        settle()
        assertEquals(AccountFieldError.EmailTaken, vm.state.value.form.errors[AccountViewModel.NEW_EMAIL])
        auth.update = ApiResult.Failed(429, "too_many_attempts", 30)
        vm.onSave()
        settle()
        assertEquals(AccountError.TooManyAttempts(30), vm.state.value.form.error)
    }
}
