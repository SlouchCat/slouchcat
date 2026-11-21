package com.slouchcat.app.mobile.profilesettings

import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalResources
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.ProfilesSettingsRoute
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R

/**
 * «Profile verwalten» of stage 4 (mockup `/settings/profiles`): the first row of «Einstellungen»
 * in «Mehr» and the page. The profile choice links here with `opener(ProfilesSettingsRoute)`.
 * [repository]: tests bring a fake.
 */
class ProfileSettingsFeature(
    private val repository: (AppGraph) -> ProfileSettingsRepository = { ApiProfileSettingsRepository(it.api, it.http) },
) : Feature {
    override val more = listOf(
        MoreEntry("profiles", MoreSection.Settings, 10, R.string.app_profiles_manage_title, SlouchcatIcon.Users, ProfilesSettingsRoute, subtitle = { subtitle(it) }),
    )

    @Composable
    private fun subtitle(graph: AppGraph): String {
        val session by graph.session.state.collectAsStateWithLifecycle()
        val canManage = (session as? SessionState.Ready)?.profile?.isPrimary == true
        val count by produceState<Int?>(null, graph) {
            val server = graph.session.server ?: return@produceState
            value = (graph.auth.profiles(server) as? ApiResult.Ok)?.value?.size
        }
        return profilesSubtitle(count, canManage)
    }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<ProfilesSettingsRoute> {
            val vm = viewModel {
                ProfileSettingsViewModel(graph.session, repository(graph)) { saved -> activeChanged(graph, saved) }
            }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            val resources = LocalResources.current
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    snackbar.showSnackbar(
                        when (message) {
                            is ProfileMessage.Added -> resources.getString(R.string.app_profiles_manage_added, message.name)
                            ProfileMessage.Saved -> resources.getString(R.string.app_profiles_manage_saved)
                            is ProfileMessage.Deleted -> resources.getString(R.string.app_profiles_manage_deleted, message.name)
                            ProfileMessage.Partly -> resources.getString(R.string.app_profiles_manage_partly)
                        },
                    )
                }
            }
            val events = remember(vm) { events(vm, onBack = navigator::back, onSwitchProfile = navigator::toProfiles) }
            ProfileSettingsScreen(state, snackbar, events)
        }
    }

    /** The profile in use was saved: its name and colour in the session, its libraries in the media choice. */
    private suspend fun activeChanged(graph: AppGraph, saved: Profile) {
        val now = (graph.session.state.value as? SessionState.Ready)?.profile ?: return
        graph.session.profileUpdated(ActiveProfile(saved.id, saved.name, saved.isPrimary, saved.accent?.value, saved.image ?: now.image))
        graph.librariesSource.reload()
    }
}

/** The events of the page, bound to its ViewModel and the navigation. */
internal fun events(vm: ProfileSettingsViewModel, onBack: () -> Unit, onSwitchProfile: () -> Unit) = object : ProfileSettingsEvents {
    override fun onBack() = onBack()
    override fun onRetry() = vm.retry()
    override fun onSwitchProfile() = onSwitchProfile()
    override fun onAdvice() = vm.onAdvice()
    override fun onAdd() = vm.onAdd()
    override fun onEdit(profile: Profile) = vm.onEdit(profile)
    override fun onDismiss() = vm.onDismiss()
    override fun onName(name: String) = vm.onName(name)
    override fun onAccent(accent: Accent) = vm.onAccent(accent)
    override fun onRating(age: Int) = vm.onRating(age)
    override fun onToggleLanguages() = vm.onToggleLanguages()
    override fun onLanguage(language: Language) = vm.onLanguage(language)
    override fun onAllLibraries(all: Boolean) = vm.onAllLibraries(all)
    override fun onLibrary(id: Long, on: Boolean) = vm.onLibrary(id, on)
    override fun onWatchLimit(text: String) = vm.onWatchLimit(text)
    override fun onPinForm(kind: PinKind?) = vm.onPinForm(kind)
    override fun onPinFirst(digits: String) = vm.onPinFirst(digits)
    override fun onPinSecond(digits: String) = vm.onPinSecond(digits)
    override fun onPinApply() {
        vm.onPinApply()
    }
    override fun onPinRemove(kind: PinKind) = vm.onPinRemove(kind)
    override fun onDeleteAsk(ask: Boolean) = vm.onDeleteAsk(ask)
    override fun onDelete() = vm.onDelete()
    override fun onSave() = vm.onSave()
}
