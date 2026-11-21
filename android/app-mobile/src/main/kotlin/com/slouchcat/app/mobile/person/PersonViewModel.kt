package com.slouchcat.app.mobile.person

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.toLoadable
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.Person
import com.slouchcat.core.api.generated.models.PersonCreditsInner
import com.slouchcat.core.api.map
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** A title of the person with every role in it, in the order of the credits (web `titlesOf`). */
data class PersonTitle(val item: ItemRef, val roles: List<PersonCreditsInner>)

/** The person and the titles the profile sees, one card per title. */
data class PersonPage(val person: Person, val titles: List<PersonTitle>)

data class PersonUiState(
    val person: Loadable<PersonPage> = Loadable.Loading,
    /** The biography beyond its first lines, closed from the start. */
    val biographyOpen: Boolean = false,
)

/** A person (mockup `/person/:id`, web `PersonPage`): `getPerson`; not visible is 404. */
class PersonViewModel(
    private val session: SessionManager,
    private val repository: PersonRepository,
    private val id: Long,
) : ViewModel() {
    private val _state = MutableStateFlow(PersonUiState())
    val state: StateFlow<PersonUiState> = _state.asStateFlow()

    private var loading: Job? = null

    init {
        load()
    }

    fun retry() = load()

    fun onToggleBiography() = _state.update { it.copy(biographyOpen = !it.biographyOpen) }

    private fun load() {
        val server = session.server ?: return
        loading?.cancel()
        _state.update { it.copy(person = Loadable.Loading) }
        loading = viewModelScope.launch {
            val loaded = repository.person(server, id).map { PersonPage(it, titlesOf(it.credits)) }.toLoadable()
            _state.update { it.copy(person = loaded) }
        }
    }

    private fun titlesOf(credits: List<PersonCreditsInner>): List<PersonTitle> =
        credits.groupBy { it.item.id }.values.map { PersonTitle(it.first().item, it) }
}
