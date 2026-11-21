package com.slouchcat.app.mobile.shell

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.consumeWindowInsets
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.paneTitle
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.intl.Locale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.toUpperCase
import androidx.compose.ui.unit.dp
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.BarNavigation
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.Wordmark
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.strings.R
import kotlinx.coroutines.launch
import java.text.NumberFormat

/** What a page of the shell may ask of it: the menu of the media choice (phone only, when there is one). */
@Immutable
data class ShellChrome(val openMenu: (() -> Unit)? = null)

val LocalShell = staticCompositionLocalOf { ShellChrome() }

/**
 * The bar of a top page of the shell (`bar(titel, {back: false})` in the mockup): the menu button
 * of the media choice when the phone has one, otherwise the title alone.
 */
@Composable
fun ShellTopBar(title: String, modifier: Modifier = Modifier, actions: @Composable RowScope.() -> Unit = {}) {
    val menu = LocalShell.current.openMenu
    TopBar(
        title = title,
        modifier = modifier,
        navigation = if (menu != null) BarNavigation.Menu else BarNavigation.None,
        navigationLabel = if (menu != null) stringResource(R.string.app_shell_menuOpen) else null,
        onNavigation = { menu?.invoke() },
        actions = actions,
    )
}

/** What the shell draws around the current page; computed from the registry and the back stack. */
data class ShellFrame(
    /** The page draws alone (entry pages, player). */
    val fullScreen: Boolean,
    /** The targets of the bar (empty: no bar). */
    val targets: List<TopDestination>,
    /** The key of the marked target. */
    val selectedTarget: String?,
    val media: List<MediaEntry>,
    /** The key of the marked media entry; it wins over the target in the side column. */
    val selectedMedia: String?,
    /** The badge of each target that has one now. */
    val badges: Map<String, ShellBadge> = emptyMap(),
)

/**
 * The frame of the app (mockup `core.js`, BAUKASTEN «Medienauswahl links», concept 27 3.2):
 * - phone: the bottom bar with the targets, the media choice as a menu from the left that only
 *   the menu button opens (the edge belongs to the system's back gesture), Back closes it;
 * - from 700 dp: a fixed side column with the targets and the media choice;
 * - [accessory] (the small player) above the bar or at the foot of the column.
 * A bar with a single target is no choice and is not drawn; a media choice without entries neither.
 *
 * One fixed tree for every case (code review B-2): the page ([content], the NavHost) always sits
 * at the same place, so switching to a full-screen page, the media entries arriving or the width
 * changing never dispose it, and every page keeps its scroll position and saved state. Pages in
 * the frame get the safe area as padding, consumed; full-screen pages handle it themselves.
 */
@Composable
fun AppShell(
    frame: ShellFrame,
    profile: ActiveProfile?,
    onTarget: (TopDestination) -> Unit,
    onMedia: (MediaEntry) -> Unit,
    onSwitchProfile: () -> Unit,
    modifier: Modifier = Modifier,
    snackbar: SnackbarHostState? = null,
    mediaCount: @Composable (MediaEntry) -> Int? = { it.count },
    accessory: (@Composable () -> Unit)? = null,
    content: @Composable () -> Unit,
) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    val framed = !frame.fullScreen
    val showTargets = framed && frame.targets.size > 1
    val showColumn = framed && wide && (showTargets || frame.media.isNotEmpty() || accessory != null)
    val showBar = framed && !wide && showTargets
    val menuAllowed = framed && !wide && frame.media.isNotEmpty()
    val drawer = rememberDrawerState(DrawerValue.Closed)
    val scope = rememberCoroutineScope()
    // The menu never stays open behind a page that has none (a full-screen page, a wide window).
    LaunchedEffect(menuAllowed) { if (!menuAllowed) drawer.snapTo(DrawerValue.Closed) }
    BackHandler(enabled = drawer.isOpen) { scope.launch { drawer.close() } }
    val chrome = ShellChrome(openMenu = if (menuAllowed) ({ scope.launch { drawer.open() } }) else null)
    val safe = WindowInsets.safeDrawing
    val title = stringResource(R.string.app_shell_mediaChoice)

    ModalNavigationDrawer(
        drawerState = drawer,
        // Only open (to close with a swipe); the left edge belongs to the system's back gesture.
        gesturesEnabled = drawer.isOpen,
        scrimColor = c.scrim,
        modifier = modifier,
        drawerContent = {
            // The sheet is always there with its width: an empty drawer has its open and closed
            // anchors at the same place and may count as open once the entries arrive.
            ModalDrawerSheet(
                drawerState = drawer,
                drawerContainerColor = c.elev,
                drawerContentColor = c.tx,
                drawerShape = RoundedCornerShape(topEnd = 24.dp, bottomEnd = 24.dp),
                modifier = Modifier.width(300.dp).semantics { paneTitle = title },
            ) {
                if (menuAllowed) {
                    Column(Modifier.verticalScroll(rememberScrollState()).windowInsetsPadding(safe.only(WindowInsetsSides.Vertical + WindowInsetsSides.Start))) {
                        if (profile != null) {
                            DrawerHead(profile, onClick = {
                                scope.launch { drawer.close() }
                                onSwitchProfile()
                            })
                        }
                        MediaList(frame, mediaCount, onMedia = {
                            scope.launch { drawer.close() }
                            onMedia(it)
                        })
                    }
                }
            }
        },
    ) {
        Row(
            Modifier
                .fillMaxSize()
                .background(c.bg)
                .then(if (framed) Modifier.windowInsetsPadding(safe.only(WindowInsetsSides.Top + WindowInsetsSides.Horizontal)) else Modifier),
        ) {
            if (showColumn) SideColumn(frame, showTargets, mediaCount, onTarget, onMedia, accessory)
            Column(Modifier.weight(1f).fillMaxHeight()) {
                Box(
                    Modifier
                        .weight(1f)
                        .fillMaxWidth()
                        .then(
                            if (framed) {
                                val bottom = if (showBar || (!wide && accessory != null)) WindowInsets(0) else safe.only(WindowInsetsSides.Bottom)
                                Modifier
                                    .consumeWindowInsets(safe.only(WindowInsetsSides.Top + WindowInsetsSides.Horizontal))
                                    .windowInsetsPadding(bottom)
                                    .consumeWindowInsets(bottom)
                            } else {
                                Modifier
                            },
                        ),
                ) {
                    CompositionLocalProvider(LocalShell provides chrome) { content() }
                    if (snackbar != null) {
                        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).padding(16.dp))
                    }
                }
                if (framed && !wide && accessory != null) {
                    Box(Modifier.fillMaxWidth().then(if (showBar) Modifier else Modifier.windowInsetsPadding(safe.only(WindowInsetsSides.Bottom)))) {
                        accessory()
                    }
                }
                if (showBar) BottomBar(frame, onTarget)
            }
        }
    }
}

/** `.dr-head`: the active profile and «Profil wechseln» at the top of the menu. */
@Composable
private fun DrawerHead(profile: ActiveProfile, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxWidth()
            .clickable(role = Role.Button, onClick = onClick)
            .padding(horizontal = 20.dp, vertical = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Avatar(profile.name, profile.accent, size = 44.dp)
        Column(Modifier.weight(1f)) {
            Text(profile.name, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(stringResource(R.string.app_more_switchProfile), style = SlouchcatType.caption, color = c.tx2)
        }
        SlouchcatIconImage(SlouchcatIcon.Right, contentDescription = null, size = 18.dp, tint = c.tx2)
    }
}

/** `mediaNav()`: «Medien» and «Meine Medien» with their entries. */
@Composable
private fun ColumnScope.MediaList(frame: ShellFrame, count: @Composable (MediaEntry) -> Int?, onMedia: (MediaEntry) -> Unit) {
    for (section in MediaSection.entries) {
        val entries = frame.media.filter { it.section == section }
        if (entries.isEmpty()) continue
        SectionLabel(stringResource(if (section == MediaSection.Media) R.string.app_shell_media else R.string.app_shell_myMedia))
        for (entry in entries) {
            NavRow(
                label = entry.label.text(),
                icon = entry.icon,
                selected = entry.key == frame.selectedMedia,
                count = count(entry),
                onClick = { onMedia(entry) },
            )
        }
    }
}

@Composable
private fun SectionLabel(text: String) {
    Text(
        text.toUpperCase(Locale.current),
        style = SlouchcatType.section,
        color = SlouchcatTheme.colors.tx3,
        modifier = Modifier.padding(start = 26.dp, end = 16.dp, top = 18.dp, bottom = 6.dp).semantics { heading() },
    )
}

/** The text a screen reader says for a badge, and what the badge shows. */
@Composable
private fun badgeText(badge: ShellBadge?): Pair<String, String>? = when (badge) {
    null -> null
    ShellBadge.Dot -> stringResource(R.string.app_shell_badgeDot) to ""
    is ShellBadge.Count -> pluralStringResource(R.plurals.app_shell_badge, badge.count, badge.count) to
        if (badge.count > MAX_BADGE) "$MAX_BADGE+" else NumberFormat.getIntegerInstance().format(badge.count)
}

private const val MAX_BADGE = 99

@Composable
private fun BadgeMark(shown: String, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    if (shown.isEmpty()) {
        Box(modifier.size(8.dp).clip(CircleShape).background(c.accent))
    } else {
        Box(
            modifier
                .heightIn(min = 16.dp)
                .defaultMinSize(minWidth = 16.dp)
                .clip(CircleShape)
                .background(c.accent)
                .padding(horizontal = 4.dp),
            contentAlignment = Alignment.Center,
        ) {
            Text(shown, style = SlouchcatType.badge, color = c.onAccent, maxLines = 1)
        }
    }
}

/** `.dn` and `.rn`: a row of the side column or the menu; marked with the accent, as the mockup does. */
@Composable
private fun NavRow(
    label: String,
    icon: SlouchcatIcon,
    selected: Boolean,
    onClick: () -> Unit,
    count: Int? = null,
    dense: Boolean = false,
    badge: ShellBadge? = null,
) {
    val c = SlouchcatTheme.colors
    val said = badgeText(badge)
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 12.dp, vertical = 1.dp)
            .clip(RoundedCornerShape(if (dense) 16.dp else 24.dp))
            .background(if (selected) c.accent else c.elev)
            .clickable(role = Role.Tab, onClick = onClick)
            .semantics {
                this.selected = selected
                if (said != null) contentDescription = "$label, ${said.first}"
            }
            .heightIn(min = 48.dp)
            .padding(horizontal = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        val tint = if (selected) c.onAccent else c.tx2
        SlouchcatIconImage(icon, contentDescription = null, size = 22.dp, tint = tint)
        Text(
            label,
            style = SlouchcatType.body.copy(fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal),
            color = if (selected) c.onAccent else c.tx,
            modifier = Modifier.weight(1f),
        )
        if (said != null) BadgeMark(said.second)
        if (count != null) {
            Text(NumberFormat.getIntegerInstance().format(count), style = SlouchcatType.caption, color = if (selected) c.onAccent else c.tx2)
        }
    }
}

/** The side column from 700 dp (`#rail`): brand, targets, a line, the media choice, the accessory at its foot. */
@Composable
private fun SideColumn(
    frame: ShellFrame,
    showTargets: Boolean,
    mediaCount: @Composable (MediaEntry) -> Int?,
    onTarget: (TopDestination) -> Unit,
    onMedia: (MediaEntry) -> Unit,
    accessory: (@Composable () -> Unit)?,
) {
    val c = SlouchcatTheme.colors
    Column(
        Modifier
            .width(236.dp)
            .fillMaxHeight()
            .background(c.elev)
            .windowInsetsPadding(WindowInsets.navigationBars.only(WindowInsetsSides.Bottom)),
    ) {
        Column(Modifier.weight(1f).verticalScroll(rememberScrollState()).padding(vertical = 14.dp)) {
            Row(Modifier.padding(start = 22.dp, bottom = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                Wordmark(stringResource(R.string.app_name), modifier = Modifier.padding(top = 4.dp))
            }
            if (showTargets) {
                for (target in frame.targets) {
                    NavRow(
                        label = stringResource(target.label),
                        icon = target.icon,
                        // One entry marked: a media entry wins over the target.
                        selected = frame.selectedMedia == null && target.key == frame.selectedTarget,
                        onClick = { onTarget(target) },
                        dense = true,
                        badge = frame.badges[target.key],
                    )
                }
                if (frame.media.isNotEmpty()) HorizontalDivider(color = c.line, modifier = Modifier.padding(top = 10.dp))
            }
            MediaList(frame, mediaCount, onMedia)
            Spacer(Modifier.height(8.dp))
        }
        accessory?.invoke()
    }
}

/** The bottom bar of the phone (`#tabs`): icon in a pill, label under it; the marked one in the accent. */
@Composable
private fun BottomBar(frame: ShellFrame, onTarget: (TopDestination) -> Unit) {
    val c = SlouchcatTheme.colors
    Column(Modifier.fillMaxWidth().background(c.elev)) {
        HorizontalDivider(color = c.line)
        Row(
            Modifier
                .fillMaxWidth()
                .windowInsetsPadding(WindowInsets.navigationBars.only(WindowInsetsSides.Bottom))
                .padding(vertical = 8.dp),
        ) {
            for (target in frame.targets) {
                val selected = target.key == frame.selectedTarget
                val label = stringResource(target.label)
                val said = badgeText(frame.badges[target.key])
                Column(
                    Modifier
                        .weight(1f)
                        .defaultMinSize(minHeight = 56.dp)
                        .clickable(role = Role.Tab, onClick = { onTarget(target) })
                        .clearAndSetSemantics {
                            contentDescription = if (said != null) "$label, ${said.first}" else label
                            role = Role.Tab
                            this.selected = selected
                            onClick { onTarget(target); true }
                        },
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Box {
                        Box(
                            Modifier
                                .size(width = 64.dp, height = 32.dp)
                                .clip(CircleShape)
                                .background(if (selected) c.accent else c.elev),
                            contentAlignment = Alignment.Center,
                        ) {
                            SlouchcatIconImage(target.icon, contentDescription = null, size = 22.dp, tint = if (selected) c.onAccent else c.tx2)
                        }
                        if (said != null) BadgeMark(said.second, Modifier.align(Alignment.TopEnd).offset(x = (-10).dp, y = 2.dp))
                    }
                    Text(
                        label,
                        style = SlouchcatType.caption.copy(fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal),
                        color = if (selected) c.tx else c.tx2,
                        maxLines = 1,
                    )
                }
            }
        }
    }
}
