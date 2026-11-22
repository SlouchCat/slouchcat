package com.slouchcat.core.browse.profilesettings

import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.browse.testing.profile
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** When saving the profile in use starts a new session epoch (code review m-3 of part a). */
class ContentChangedTest {
    private val before = profile(1, "Mara", primary = true, rating = 16, allLibraries = false, libraryIds = listOf(1, 2))

    @Test
    fun nameAndColourAloneChangeInPlace() {
        assertFalse(contentChanged(before, before.copy(name = "Mara B.", accent = Accent.SKY)))
        // The same libraries in another order are the same.
        assertFalse(contentChanged(before, before.copy(libraryIds = listOf(2, 1))))
    }

    @Test
    fun ratingLanguageOrLibrariesStartANewEpoch() {
        assertTrue(contentChanged(before, before.copy(maxRating = 18)))
        assertTrue(contentChanged(before, before.copy(language = Language.EN)))
        assertTrue(contentChanged(before, before.copy(allLibraries = true)))
        assertTrue(contentChanged(before, before.copy(libraryIds = listOf(1))))
    }

    @Test
    fun withAllLibrariesTheListDoesNotCount() {
        val all = before.copy(allLibraries = true, libraryIds = emptyList())
        assertFalse(contentChanged(all, all.copy(libraryIds = listOf(1, 2))))
    }

    @Test
    fun withoutTheEarlierStateANewEpoch() {
        assertTrue(contentChanged(null, before))
    }
}
