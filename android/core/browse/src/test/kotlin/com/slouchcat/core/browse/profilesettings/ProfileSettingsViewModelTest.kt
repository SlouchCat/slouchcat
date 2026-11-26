package com.slouchcat.core.browse.profilesettings

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.generated.models.WishRights
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.log.LogSink
import com.slouchcat.core.browse.testing.CHILD
import com.slouchcat.core.browse.testing.FakeProfileSettings
import com.slouchcat.core.browse.testing.profile
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class ProfileSettingsViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private val saved = mutableListOf<Profile>()
    private val partlies = mutableListOf<Boolean>()

    private fun fake() = FakeProfileSettings(profile(1, "Primary", primary = true), profile(2, "Child", rating = 12, hasPin = true))

    private fun TestScope.model(repository: FakeProfileSettings, primary: Boolean = true, requestRight: Boolean = false): ProfileSettingsViewModel {
        val s = if (primary) session(backgroundScope) else session(backgroundScope, profile = CHILD)
        settle()
        return ProfileSettingsViewModel(s, repository, requestRight = requestRight) { _, it, partly -> saved += it; partlies += partly }.also { settle() }
    }

    private fun wishes(reason: WishRights.Reason? = null) =
        WishRights(enabled = true, canRequest = reason == null, movies = true, shows = true, reason = reason)

    /** The primary profile may request, the child may not; the wishes are on. */
    private fun requesting() = FakeProfileSettings(profile(1, "Primary", primary = true, canRequest = true), profile(2, "Child", rating = 12, canRequest = false)).apply { wishes = wishes() }

    private fun ProfileSettingsViewModel.list() = (state.value.profiles as Loadable.Ready).value

    private fun ProfileSettingsViewModel.editor() = checkNotNull(state.value.editor)

    @Test
    fun theListWithRatingsLibrariesAndThePinAdvice() = runTest(dispatcher) {
        val vm = model(fake())
        assertEquals(listOf("Primary", "Child"), vm.list().map { it.name })
        assertEquals(listOf(0, 6, 12, 16, 18), vm.state.value.ratings.map { it.age })
        assertEquals("FSK 12", vm.state.value.ratingLabel(12))
        assertEquals(2, vm.state.value.libraries.size)
        // A child with an age limit and no PIN on the primary profile (E-9, F-19).
        assertTrue(vm.state.value.advisePin)
        vm.onAdvice()
        assertEquals(1L, vm.editor().id)
        assertEquals(PinKind.Pin, vm.editor().pinForm)
    }

    @Test
    fun noAdviceWhenThePrimaryProfileHasAPinOrNoOneHasALimit() = runTest(dispatcher) {
        val withPin = model(FakeProfileSettings(profile(1, "P", primary = true, hasPin = true), profile(2, "C", rating = 6)))
        assertFalse(withPin.state.value.advisePin)
        val adults = model(FakeProfileSettings(profile(1, "P", primary = true), profile(2, "C", rating = 18)))
        assertFalse(adults.state.value.advisePin)
    }

    @Test
    fun anotherProfileOnlyReads() = runTest(dispatcher) {
        val repository = fake()
        val vm = model(repository, primary = false)
        assertFalse(vm.state.value.canManage)
        assertEquals(listOf("Primary", "Child"), (vm.state.value.summaries as Loadable.Ready).value.map { it.name })
        assertTrue("list" !in repository.calls)
        vm.onAdd()
        assertNull(vm.state.value.editor)
    }

    @Test
    fun theServerRefusingTheListMakesThePageReadOnly() = runTest(dispatcher) {
        val repository = fake()
        repository.listAnswer = ApiResult.NotAllowed("primary_profile_required")
        val vm = model(repository)
        assertFalse(vm.state.value.canManage)
        assertTrue(vm.state.value.summaries is Loadable.Ready)
    }

    @Test
    fun aFailedListCanBeRetried() = runTest(dispatcher) {
        val repository = fake()
        repository.listAnswer = ApiResult.Offline
        val vm = model(repository)
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.profiles)
        repository.listAnswer = null
        vm.retry()
        settle()
        assertEquals(2, vm.list().size)
    }

    @Test
    fun addAProfileWithAPin() = runTest(dispatcher) {
        val repository = fake()
        val vm = model(repository)
        val messages = mutableListOf<ProfileMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        vm.onAdd()
        // The first colour no other profile has, the account's ceiling.
        assertEquals(Accent.CORAL, vm.editor().accent)
        assertEquals(18, vm.editor().rating)
        vm.onSave()
        assertTrue(vm.editor().nameMissing)
        vm.onName("  Kids ")
        vm.onRating(6)
        vm.onWatchLimit("90 min")
        vm.onAllLibraries(false)
        vm.onLibrary(2, true)
        vm.onPinForm(PinKind.Pin)
        vm.onPinFirst("12")
        vm.onPinSecond("12")
        assertFalse(vm.onPinApply())
        assertEquals(PinError.Format, vm.editor().pinError)
        vm.onPinFirst("1234")
        vm.onPinSecond("1243")
        assertFalse(vm.onPinApply())
        assertEquals(PinError.Mismatch, vm.editor().pinError)
        vm.onPinSecond("1234")
        vm.onSave()
        settle()
        assertNull(vm.state.value.editor)
        val request = repository.created.single()
        assertEquals("Kids", request.name)
        assertEquals(6, request.maxRating)
        assertEquals(90, request.limits?.watchLimitMinutes)
        assertEquals(false, request.allLibraries)
        assertEquals(setOf(2L), request.libraryIds)
        assertEquals("create:Kids:pin=4", repository.calls.first { it.startsWith("create") })
        assertEquals(ProfileMessage.Added("Kids"), messages.single())
        assertEquals(3, vm.list().size)
    }

    @Test
    fun editKeepsTheOtherLimitsAndChangesBothPins() = runTest(dispatcher) {
        val repository = fake()
        val vm = model(repository)
        val messages = mutableListOf<ProfileMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        vm.onEdit(vm.list()[1])
        assertTrue(vm.editor().pinOn(PinKind.Pin))
        vm.onName("Kid")
        vm.onAccent(Accent.SKY)
        vm.onToggleLanguages()
        assertTrue(vm.editor().languagesOpen)
        vm.onLanguage(Language.EN)
        assertFalse(vm.editor().languagesOpen)
        vm.onWatchLimit("60")
        vm.onPinRemove(PinKind.Pin)
        assertFalse(vm.editor().pinOn(PinKind.Pin))
        vm.onPinForm(PinKind.Offline)
        vm.onPinFirst("123456")
        vm.onPinSecond("123456")
        vm.onSave()
        settle()
        val (id, update) = repository.updated.single()
        assertEquals(2L, id)
        assertEquals("Kid", update.name)
        assertEquals(Accent.SKY, update.accent)
        assertEquals(Language.EN, update.language)
        assertEquals(2, update.limits?.maxStreams)
        assertEquals(60, update.limits?.watchLimitMinutes)
        assertTrue("unpin:2" in repository.calls)
        assertTrue("offline:2:6" in repository.calls)
        assertEquals(ProfileMessage.Saved, messages.single())
        assertFalse(vm.list()[1].hasPin)
        assertTrue(vm.list()[1].hasOfflinePin)
        // Not the profile in use: the session stays as it is.
        assertTrue(saved.isEmpty())
    }

    @Test
    fun savingTheProfileInUseUpdatesTheSessionAndAnOffLimitStays() = runTest(dispatcher) {
        val repository = fake()
        val vm = model(repository)
        vm.onEdit(vm.list()[0])
        vm.onWatchLimit("2000")
        vm.onSave()
        assertTrue(vm.editor().watchLimitInvalid)
        vm.onWatchLimit("")
        vm.onName("Main")
        vm.onSave()
        settle()
        assertEquals("Main", saved.single().name)
        assertNull(repository.updated.single().second.limits?.watchLimitMinutes)
    }

    @Test
    fun aRefusedSaveKeepsTheSheetWithTheReason() = runTest(dispatcher) {
        val repository = fake()
        repository.saveFailure = ApiResult.Failed(422, "validation_failed")
        val vm = model(repository)
        vm.onEdit(vm.list()[1])
        vm.onSave()
        settle()
        assertEquals("validation_failed", vm.editor().error)
        assertFalse(vm.editor().busy)
        vm.onDismiss()
        assertNull(vm.state.value.editor)
    }

    @Test
    fun aRefusedPinIsReportedAfterTheProfileWasSaved() = runTest(dispatcher) {
        val repository = fake()
        repository.pinFailure = ApiResult.Offline
        val vm = model(repository)
        val messages = mutableListOf<ProfileMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        vm.onEdit(vm.list()[0])
        vm.onPinForm(PinKind.Pin)
        vm.onPinFirst("4321")
        vm.onPinSecond("4321")
        vm.onSave()
        settle()
        assertEquals(ProfileMessage.Partly, messages.single())
        // The profile in use: the result goes with the change, past a new epoch (code review m-2 of part a).
        assertEquals(listOf(true), partlies)
    }

    @Test
    fun aPinTypedAndRemovedAgainChangesNothing() = runTest(dispatcher) {
        val repository = fake()
        val vm = model(repository)
        vm.onEdit(vm.list()[0])
        vm.onPinForm(PinKind.Pin)
        vm.onPinFirst("1111")
        vm.onPinSecond("1111")
        vm.onPinApply()
        assertTrue(vm.editor().pinOn(PinKind.Pin))
        vm.onPinRemove(PinKind.Pin)
        assertEquals(PinChange.Keep, vm.editor().pin)
        vm.onPinForm(PinKind.Pin)
        vm.onPinForm(null)
        assertNull(vm.editor().pinForm)
        assertFalse(vm.editor().toString().contains("1111"))
    }

    /**
     * The digits of a PIN never leave the ViewModel's state (code review L-1): not in `toString` of
     * the editor or the page state, not in any line of the log while they are typed and saved, and
     * the ViewModel keeps no saved state (no `SavedStateHandle`) that could outlive the process.
     */
    @Test
    fun theDigitsOfAPinAreNeverPrintedLoggedOrSaved() = runTest(dispatcher) {
        val lines = mutableListOf<String>()
        val before = Log.sink
        Log.sink = object : LogSink {
            override fun write(priority: Int, tag: String, message: String) { lines += "$tag $message" }
        }
        try {
            val vm = model(fake())
            vm.onEdit(vm.list()[0])
            vm.onPinForm(PinKind.Pin)
            vm.onPinFirst("5678")
            vm.onPinSecond("5678")
            vm.onPinApply()
            vm.onPinForm(PinKind.Offline)
            vm.onPinFirst("424242")
            vm.onPinSecond("424242")
            val shown = listOf(vm.editor().toString(), vm.state.value.toString(), PinChange.Set("5678").toString())
            vm.onSave()
            settle()
            for (text in shown + lines) {
                assertFalse(text, "5678" in text)
                assertFalse(text, "424242" in text)
            }
            assertTrue(ProfileSettingsViewModel::class.java.constructors.all { c -> c.parameterTypes.none { it.name == "androidx.lifecycle.SavedStateHandle" } })
        } finally {
            Log.sink = before
        }
    }

    @Test
    fun deleteAsksAndCanBeKept() = runTest(dispatcher) {
        val repository = fake()
        val vm = model(repository)
        val messages = mutableListOf<ProfileMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        // The primary profile cannot be deleted.
        vm.onEdit(vm.list()[0])
        vm.onDeleteAsk(true)
        assertFalse(vm.editor().confirmDelete)
        vm.onEdit(vm.list()[1])
        vm.onDeleteAsk(true)
        assertTrue(vm.editor().confirmDelete)
        vm.onDeleteAsk(false)
        assertFalse(vm.editor().confirmDelete)
        vm.onDeleteAsk(true)
        vm.onDelete()
        settle()
        assertNull(vm.state.value.editor)
        assertEquals(ProfileMessage.Deleted("Child"), messages.single())
        assertEquals(listOf("Primary"), vm.list().map { it.name })
    }

    @Test
    fun theRequestSwitchIsHiddenWhileTheWishesAreOff() = runTest(dispatcher) {
        val off = model(FakeProfileSettings(profile(1, "P", primary = true)).apply { wishes = wishes().copy(enabled = false) }, requestRight = true)
        assertFalse(off.state.value.requestShown)
        val none = model(FakeProfileSettings(profile(1, "P", primary = true)), requestRight = true)
        assertFalse(none.state.value.requestShown)
    }

    @Test
    fun theTvNeverShowsTheSwitchNorSendsTheRight() = runTest(dispatcher) {
        val repository = requesting()
        val vm = model(repository)
        assertFalse(vm.state.value.requestShown)
        vm.onEdit(vm.list()[1])
        vm.onCanRequest(true)
        vm.onSave()
        settle()
        assertNull(repository.updated.single().second.canRequest)
        vm.onAdd()
        vm.onName("New")
        vm.onSave()
        settle()
        assertNull(repository.created.single().canRequest)
    }

    @Test
    fun theSwitchStartsFromTheProfileAndUpdateSendsIt() = runTest(dispatcher) {
        val repository = requesting()
        val vm = model(repository, requestRight = true)
        assertTrue(vm.state.value.requestShown)
        assertTrue(vm.state.value.requestAllowed)
        // The primary profile's own sheet shows the switch too, as the web form.
        vm.onEdit(vm.list()[0])
        assertTrue(vm.editor().canRequest)
        vm.onEdit(vm.list()[1])
        assertFalse(vm.editor().canRequest)
        vm.onCanRequest(true)
        vm.onSave()
        settle()
        assertEquals(true, repository.updated.single().second.canRequest)
        assertEquals(true, vm.list()[1].canRequest)
    }

    @Test
    fun aNewProfileStartsWithoutTheRightAndCreateSendsIt() = runTest(dispatcher) {
        val repository = requesting()
        val vm = model(repository, requestRight = true)
        vm.onAdd()
        assertFalse(vm.editor().canRequest)
        vm.onName("Guest")
        vm.onSave()
        settle()
        assertEquals(false, repository.created.single().canRequest)
        vm.onAdd()
        vm.onName("Teen")
        vm.onCanRequest(true)
        vm.onSave()
        settle()
        assertEquals(true, repository.created.last().canRequest)
    }

    @Test
    fun anAccountWithoutTheRightLocksTheSwitchAndLeavesTheFieldOut() = runTest(dispatcher) {
        val repository = requesting().apply { wishes = wishes(WishRights.Reason.ACCOUNT_NOT_ALLOWED) }
        val vm = model(repository, requestRight = true)
        assertTrue(vm.state.value.requestShown)
        assertTrue(vm.state.value.requestCapped)
        assertFalse(vm.state.value.requestAllowed)
        vm.onEdit(vm.list()[1])
        vm.onCanRequest(true)
        assertFalse(vm.editor().canRequest)
        vm.onSave()
        settle()
        assertNull(repository.updated.single().second.canRequest)
    }

    @Test
    fun aRefusedRightLocksTheSwitch() = runTest(dispatcher) {
        val repository = requesting()
        repository.saveFailure = ApiResult.Failed(422, "validation_failed").apply {
            problem = buildJsonObject {
                put("code", JsonPrimitive("validation_failed"))
                put("errors", buildJsonArray { add(buildJsonObject { put("field", JsonPrimitive("canRequest")); put("code", JsonPrimitive("account_not_allowed")) }) })
            }
        }
        val vm = model(repository, requestRight = true)
        vm.onEdit(vm.list()[1])
        vm.onCanRequest(true)
        vm.onSave()
        settle()
        assertTrue(vm.editor().requestRefused)
        assertFalse(vm.state.value.requestAllowed)
        assertEquals("validation_failed", vm.editor().error)
        vm.onCanRequest(true)
        repository.saveFailure = null
        vm.onSave()
        settle()
        // The second try leaves the right out.
        assertNull(repository.updated.single().second.canRequest)
    }

    @Test
    fun anotherRefusalLeavesTheSwitchFree() = runTest(dispatcher) {
        val repository = requesting()
        repository.saveFailure = ApiResult.Failed(422, "validation_failed").apply {
            problem = buildJsonObject {
                put("errors", buildJsonArray { add(buildJsonObject { put("field", JsonPrimitive("name")); put("code", JsonPrimitive("required")) }) })
            }
        }
        val vm = model(repository, requestRight = true)
        vm.onEdit(vm.list()[1])
        vm.onSave()
        settle()
        assertFalse(vm.editor().requestRefused)
        assertTrue(vm.state.value.requestAllowed)
    }
}
