package com.slouchcat.app.tv.shell

import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvMenuItem
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable

/** The entry of the rail that opens [LibrariesMenu]; no page of its own. */
@Serializable
data object LibrariesMenuRoute

/** The key of that entry. */
const val LIBRARIES_MENU = "libraries-menu"

/** The entry «Alle Bibliotheken …» at the end of «Medien» (mockup `NAV` `libs`, `ACT.libs`). */
fun librariesMenuEntry(): RailEntry = RailEntry(
    key = LIBRARIES_MENU,
    section = RailSection.Media,
    order = Int.MAX_VALUE,
    label = Label.Res(R.string.tv_shell_allLibraries),
    icon = SlouchcatIcon.Grid.vector,
    route = LibrariesMenuRoute,
    isCurrent = { false },
)

/**
 * How many libraries the rail shows (code review M-3): what fits on the TV's height after brand,
 * profile, Start, Suche, Wünsche, Einstellungen, the two section gaps, and two more entries of
 * «Medien» (the radio of stage 4 and «Alle Bibliotheken …»). On 1080p (540 dp) that is five.
 */
fun railLibraryCapacity(height: Dp): Int {
    val r = TvDimens.Rail
    val inner = height - TvDimens.safeY * 2 - r.Brand - r.Profile
    val fixed = (r.Item + r.Gap) * FIXED_ENTRIES + r.Section * 2
    val slots = ((inner - fixed) / (r.Media + r.Gap)).toInt()
    return (slots - RESERVED).coerceAtLeast(1)
}

/** Start, Suche, Wünsche, Einstellungen. */
private const val FIXED_ENTRIES = 4

/** Webradio (stage 4) and «Alle Bibliotheken …». */
private const val RESERVED = 2

/**
 * «Bibliotheken» from the right (mockup `41-start-menu-libs`, `ACT.libs`): every library of the
 * profile with its count, then «Meine Listen» (Merkliste, Gesehen, Sammlungen). OK opens the page,
 * Back closes the menu and the focus goes back to where it was.
 */
@Composable
fun LibrariesMenu(libraries: List<RailEntry>, lists: List<RailEntry>, onOpen: (RailEntry) -> Unit, onDismiss: () -> Unit) {
    val first = remember { FocusRequester() }
    TvSideMenu(stringResource(R.string.tv_shell_libraries), onDismiss = onDismiss, initialFocus = first) {
        (libraries + lists).forEachIndexed { index, entry ->
            if (index == libraries.size && lists.isNotEmpty()) {
                Text(
                    stringResource(R.string.tv_shell_myLists),
                    style = TvType.largeStrong,
                    color = SlouchcatTheme.colors.tx2,
                    modifier = Modifier.padding(top = TvDimens.xl, bottom = TvDimens.s).semantics { heading() },
                )
            }
            TvMenuItem(
                entry.label.text(),
                onClick = { onOpen(entry) },
                icon = entry.icon,
                subtitle = entry.count?.let { pluralCount(R.plurals.home_list_count, it) },
                modifier = if (index == 0) Modifier.focusRequester(first) else Modifier,
            )
        }
    }
}
