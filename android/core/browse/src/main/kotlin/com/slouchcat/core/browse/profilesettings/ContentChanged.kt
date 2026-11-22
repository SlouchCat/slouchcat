package com.slouchcat.core.browse.profilesettings

import com.slouchcat.core.api.generated.models.Profile

/**
 * Whether saving the profile in use changed what it may see: rating, libraries or language of the
 * metadata. Then a new session epoch, so no page keeps titles of the old state; name and colour
 * alone change in place (code review m-3 of stage 4 part a). Without the earlier state: a new epoch.
 * The libraries count only without «alle Bibliotheken» (the server ignores the list then).
 */
fun contentChanged(before: Profile?, saved: Profile): Boolean {
    if (before == null) return true
    if (before.maxRating != saved.maxRating || before.language != saved.language || before.allLibraries != saved.allLibraries) return true
    return !saved.allLibraries && before.libraryIds.toSet() != saved.libraryIds.toSet()
}
