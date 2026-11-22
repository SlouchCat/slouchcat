package com.slouchcat.app.tv.profilesettings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.tv.material3.Text
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.browse.ProfilesManageRoute
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.shell.SettingsEntry
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcons
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.map
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R

/**
 * «Profile verwalten» (concept 26, 3.3, E-8; mockup `/settings/profiles`): the row of the settings
 * (order 20) with the profiles' count and a pane, the page, and the way from the profile choice
 * (`manageProfiles`). Only the primary profile changes profiles; the server decides.
 */
object ProfileSettingsFeature : TvFeature {
    override val settings = listOf(
        SettingsEntry(
            key = "manage",
            order = 20,
            title = R.string.app_profiles_manage_title,
            icon = TvIcons.Edit,
            route = ProfilesManageRoute,
            subtitle = { graph -> subtitle(graph) },
            pane = { graph -> Pane(graph) },
        ),
    )

    override val manageProfiles: Any = ProfilesManageRoute

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<ProfilesManageRoute> {
            val vm = viewModel { ProfileSettingsViewModel(graph.session, graph.media.profiles, graph.media.profilesCount) { before, saved, partly -> activeChanged(graph, before, saved, partly) } }
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        is ProfileMessage.Added -> graph.toasts.show(R.string.app_profiles_manage_added, message.name)
                        ProfileMessage.Saved -> graph.toasts.show(R.string.app_profiles_manage_saved)
                        is ProfileMessage.Deleted -> graph.toasts.show(R.string.app_profiles_manage_deleted, message.name)
                        ProfileMessage.Partly -> graph.toasts.show(R.string.app_profiles_manage_partly)
                    }
                }
            }
            val events = remember(vm, navigator) {
                object : ProfileSettingsEvents {
                    override fun onRetry() = vm.retry()
                    override fun onSwitchProfile() = navigator.toProfiles()
                    override fun onBack() {
                        navigator.back()
                    }
                    override fun onAdvice() = vm.onAdvice()
                    override fun onAdd() = vm.onAdd()
                    override fun onEdit(profile: Profile) = vm.onEdit(profile)
                    override fun onDismiss() = vm.onDismiss()
                    override fun onName(name: String) = vm.onName(name)
                    override fun onAccent(accent: com.slouchcat.core.api.generated.models.Accent) = vm.onAccent(accent)
                    override fun onRating(age: Int) = vm.onRating(age)
                    override fun onToggleLanguages() = vm.onToggleLanguages()
                    override fun onLanguage(language: com.slouchcat.core.api.generated.models.Language) = vm.onLanguage(language)
                    override fun onAllLibraries(all: Boolean) = vm.onAllLibraries(all)
                    override fun onLibrary(id: Long, on: Boolean) = vm.onLibrary(id, on)
                    override fun onPinForm(open: Boolean) = vm.onPinForm(if (open) PinKind.Pin else null)
                    override fun onPinFirst(digits: String) = vm.onPinFirst(digits)
                    override fun onPinSecond(digits: String) = vm.onPinSecond(digits)
                    override fun onPinApply(): Boolean = vm.onPinApply()
                    override fun onPinRemove() = vm.onPinRemove()
                    override fun onDeleteAsk(ask: Boolean) = vm.onDeleteAsk(ask)
                    override fun onDelete() = vm.onDelete()
                    override fun onSave() = vm.onSave()
                }
            }
            ProfileSettingsScreen(state, events)
        }
    }

    /** «3 Profile» once the count is known (read by the page, kept by [com.slouchcat.core.media.MediaData]). */
    @Composable
    private fun subtitle(graph: TvGraph): String? {
        val count by graph.media.profilesCount.count.collectAsStateWithLifecycle()
        LaunchedEffect(graph) { graph.session.server?.let { server -> graph.media.profilesCount.refresh { graph.media.profiles.summaries(server).map { it.size } } } }
        return count?.let { pluralCount(R.plurals.app_profiles_manage_subtitle, it) }
    }

    /** The right half of the settings while the row has the focus (mockup `mdSetPane('manage')`). */
    @Composable
    private fun Pane(graph: TvGraph) {
        val state by graph.session.state.collectAsStateWithLifecycle()
        val primary = (state as? SessionState.Ready)?.profile?.isPrimary == true
        Column(verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            Text(stringResource(R.string.app_profiles_manage_title), style = TvType.h2)
            Text(
                stringResource(if (primary) R.string.tv_profiles_pane else R.string.profiles_settings_onlyPrimary_title),
                style = TvType.body,
                color = SlouchcatTheme.colors.tx2,
            )
        }
    }

    /**
     * The profile in use was saved: its name and colour in the session. When what it may see
     * changed (rating, libraries, language), a new epoch: every page reads again, the message
     * outlives it (a PIN that did not take must not read as «Gespeichert»).
     */
    private suspend fun activeChanged(graph: TvGraph, before: Profile?, saved: Profile, partly: Boolean) {
        val now = (graph.session.state.value as? SessionState.Ready)?.profile ?: return
        val content = contentChanged(before, saved)
        if (content) graph.toasts.show(if (partly) R.string.app_profiles_manage_partly else R.string.app_profiles_manage_saved)
        graph.session.profileUpdated(ActiveProfile(saved.id, saved.name, saved.isPrimary, saved.accent?.value, saved.image ?: now.image), contentChanged = content)
        if (!content) graph.librariesSource.reload()
    }
}

/**
 * Whether saving the profile in use changed what it may see (the phone app's rule): rating,
 * language, libraries (only without «alle Bibliotheken»). Without the earlier state: yes.
 */
internal fun contentChanged(before: Profile?, saved: Profile): Boolean {
    if (before == null) return true
    if (before.maxRating != saved.maxRating || before.language != saved.language || before.allLibraries != saved.allLibraries) return true
    return !saved.allLibraries && before.libraryIds.toSet() != saved.libraryIds.toSet()
}
