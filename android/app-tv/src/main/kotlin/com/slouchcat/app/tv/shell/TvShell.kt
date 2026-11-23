package com.slouchcat.app.tv.shell

import android.text.format.DateFormat
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.zIndex
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.TvWordmark
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.strings.R
import kotlinx.coroutines.delay
import java.util.Date

/**
 * Where the content of a page begins on the left: right of the rail (`--cx`) on a page with the
 * rail, at the safe margin (`--safe-x`) on a full-screen page. Read it through [tvPage].
 */
val LocalContentStart = staticCompositionLocalOf { TvDimens.contentStart }

/** The padding of a page (`.page`): the safe margins and the start right of the rail. */
@Composable
fun Modifier.tvPage(): Modifier = padding(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.safeY, bottom = TvDimens.safeY)

/** Test tags of the shell, for the tests of later stages. */
object ShellTags {
    const val RAIL = "tv-rail"
    const val CONTENT = "tv-content"
    fun entry(key: String) = "tv-rail-$key"
    const val PROFILE = "tv-rail-profile"
    const val CLOCK = "tv-clock"
}

/**
 * The frame of the TV app (mockup `core.js`, BAUKASTEN «Fokus-Engine»): the rail on the left with
 * symbols, opening with labels as soon as it has the focus, above the page with a scrim; the
 * clock at the top right. Left out of a page goes into the rail on the marked entry; right out of
 * the rail returns to the element of the page that had the focus. [showRail] `false`: a page
 * without rail and clock (setup, profile choice, player).
 */
@Composable
fun TvShell(
    showRail: Boolean,
    rail: List<RailEntry>,
    media: List<RailEntry>,
    selected: String?,
    profile: ActiveProfile?,
    onEntry: (RailEntry) -> Unit,
    onProfile: () -> Unit,
    railFocus: FocusRequester,
    railOpen: Boolean,
    onRailOpen: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    /** The number on an entry and what a screen reader adds (stage 5); `null`: none. */
    badge: @Composable (RailEntry) -> Pair<Int, String>? = { null },
    content: @Composable () -> Unit,
) {
    val open = railOpen && showRail
    // Right out of the rail goes to the page's group, which restores the element focused last. The
    // group lies under the rail as well, so a geometric search from the rail would not choose it.
    val contentFocus = remember { FocusRequester() }
    Box(modifier.fillMaxSize().background(SlouchcatTheme.colors.bg)) {
        CompositionLocalProvider(LocalContentStart provides if (showRail) TvDimens.contentStart else TvDimens.safeX) {
            Box(
                Modifier
                    .fillMaxSize()
                    .testTag(ShellTags.CONTENT)
                    .focusRequester(contentFocus)
                    // Remembers the element that had the focus when it went to the rail. (A
                    // focusRestorer here would also catch every start focus of a new page.)
                    .focusProperties { onExit = { contentFocus.saveFocusedChild() } }
                    .focusGroup(),
            ) { content() }
        }
        if (showRail) {
            val scrim by animateFloatAsState(if (open) TvMotion.railScrim else 0f, tween(TvMotion.RAIL_MS))
            Box(Modifier.fillMaxSize().zIndex(1f).alpha(scrim).background(SlouchcatTheme.colors.scrim))
            Clock(Modifier.align(Alignment.TopEnd).padding(top = TvDimens.safeY, end = TvDimens.safeX).zIndex(1f).testTag(ShellTags.CLOCK))
            Rail(
                rail = rail,
                media = media,
                selected = selected,
                profile = profile,
                open = open,
                onOpen = onRailOpen,
                onEntry = onEntry,
                onProfile = onProfile,
                railFocus = railFocus,
                contentFocus = contentFocus,
                badge = badge,
                modifier = Modifier.zIndex(2f),
            )
        }
    }
}

@Composable
private fun Rail(
    rail: List<RailEntry>,
    media: List<RailEntry>,
    selected: String?,
    profile: ActiveProfile?,
    open: Boolean,
    onOpen: (Boolean) -> Unit,
    onEntry: (RailEntry) -> Unit,
    onProfile: () -> Unit,
    railFocus: FocusRequester,
    contentFocus: FocusRequester,
    badge: @Composable (RailEntry) -> Pair<Int, String>?,
    modifier: Modifier,
) {
    val c = SlouchcatTheme.colors
    val r = TvDimens.Rail
    val width by animateDpAsState(if (open) r.Open else r.Width, tween(TvMotion.RAIL_MS))
    val labels by animateFloatAsState(if (open) 1f else 0f, tween(TvMotion.FOCUS_MS))
    val background = if (open) {
        Brush.horizontalGradient(listOf(c.elev, c.elev))
    } else {
        Brush.horizontalGradient(0f to SlouchcatTokens.Tv.Color.Rail, RAIL_SOLID to SlouchcatTokens.Tv.Color.Rail, 1f to Color.Transparent)
    }
    val all = rail + media
    val marked = all.firstOrNull { it.key == selected } ?: all.firstOrNull()
    Column(
        modifier
            .width(width)
            .fillMaxHeight()
            .background(background)
            .padding(vertical = TvDimens.safeY)
            .testTag(ShellTags.RAIL)
            .onFocusChanged { onOpen(it.hasFocus) }
            // `data-enter`: coming into the rail always lands on the marked entry, not the last one.
            .focusProperties { onEnter = { runCatching { railFocus.requestFocus() } } }
            .focusGroup(),
    ) {
        Row(Modifier.height(r.Brand).padding(start = TvDimens.safeX - TvDimens.xs), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(r.Logo).clip(RoundedCornerShape(TvDimens.radiusLogo)).background(c.accent), contentAlignment = Alignment.Center) {
                Text(stringResource(R.string.app_name).take(1), style = TvType.logo, color = c.onAccent)
            }
            Spacer(Modifier.width(r.Pad))
            TvWordmark(stringResource(R.string.app_name), small = true, modifier = Modifier.alpha(labels))
        }
        // Scrolls as a safety (code review M-3): the focus brings its entry into view, so it never
        // lands on one below the profile; the number of libraries keeps it from being needed.
        Column(Modifier.weight(1f).fillMaxWidth().verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(r.Gap, Alignment.CenterVertically)) {
            val top = rail.filter { it.section == RailSection.Top }
            val bottom = rail.filter { it.section == RailSection.Bottom }
            for (entry in top) RailItem(entry, entry == marked, labels, onEntry, railFocus.takeIf { entry == marked }, contentFocus, badge(entry))
            if (media.isNotEmpty()) {
                Box(Modifier.height(r.Section).padding(start = TvDimens.safeX + TvDimens.xs / 4), contentAlignment = Alignment.BottomStart) {
                    Text(stringResource(R.string.tv_shell_media).uppercase(), style = TvType.section, color = c.tx3, modifier = Modifier.alpha(labels).semantics { heading() })
                }
                for (entry in media) RailItem(entry, entry == marked, labels, onEntry, railFocus.takeIf { entry == marked }, contentFocus, badge(entry), small = true)
            }
            if (bottom.isNotEmpty()) Spacer(Modifier.height(r.Section))
            for (entry in bottom) RailItem(entry, entry == marked, labels, onEntry, railFocus.takeIf { entry == marked }, contentFocus, badge(entry))
        }
        if (profile != null) ProfileItem(profile, labels, onProfile, contentFocus)
    }
}

@Composable
private fun RailItem(
    entry: RailEntry,
    current: Boolean,
    labels: Float,
    onEntry: (RailEntry) -> Unit,
    requester: FocusRequester?,
    contentFocus: FocusRequester,
    badge: Pair<Int, String>?,
    small: Boolean = false,
) {
    val c = SlouchcatTheme.colors
    val r = TvDimens.Rail
    val label = entry.label.text().let { name -> if (badge != null) "$name, ${badge.second}" else name }
    RailSurface(
        onClick = { onEntry(entry) },
        height = if (small) r.Media else r.Item,
        current = current,
        contentFocus = contentFocus,
        modifier = Modifier
            .testTag(ShellTags.entry(entry.key))
            .then(if (requester != null) Modifier.focusRequester(requester) else Modifier)
            .semantics {
                contentDescription = label
                selected = current
            },
    ) { focused ->
        Box {
            TvIcon(entry.icon, if (small) r.MediaIcon else r.Icon, tint = if (focused) c.bg else if (current) c.tx else c.tx3)
            if (badge != null) RailCount(badge.first, Modifier.align(Alignment.TopEnd).offset(x = TvDimens.s, y = -TvDimens.xs))
        }
        Spacer(Modifier.width(r.Pad))
        Text(entry.label.text(), style = if (small) TvType.body else TvType.bodyStrong, maxLines = 1, modifier = Modifier.alpha(labels))
    }
}

/** The number on an entry (`.md-cnt`, smaller): the accent with its text colour; «99+» beyond 99. */
@Composable
private fun RailCount(count: Int, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val size = SlouchcatTokens.Tv.Size.Wishes.RailBadge
    Box(
        modifier
            .heightIn(min = size)
            .widthIn(min = size)
            .clip(CircleShape)
            .background(c.accent)
            .padding(horizontal = TvDimens.xs),
        contentAlignment = Alignment.Center,
    ) {
        Text(if (count > MAX_COUNT) stringResource(R.string.notifications_badgeMax, MAX_COUNT.toString()) else count.toString(), style = TvType.badge, color = c.onAccent, maxLines = 1)
    }
}

@Composable
private fun ProfileItem(profile: ActiveProfile, labels: Float, onProfile: () -> Unit, contentFocus: FocusRequester) {
    val c = SlouchcatTheme.colors
    val r = TvDimens.Rail
    val switchLabel = stringResource(R.string.tv_shell_switchProfile)
    RailSurface(
        onClick = onProfile,
        height = r.Profile,
        current = false,
        contentFocus = contentFocus,
        modifier = Modifier.testTag(ShellTags.PROFILE).semantics { contentDescription = "${profile.name}, $switchLabel" },
        startPad = r.Pad / 2,
    ) { focused ->
        Avatar(profile.name, profile.accent, size = r.Avatar)
        Spacer(Modifier.width(r.Pad))
        Column(Modifier.alpha(labels)) {
            Text(profile.name, style = TvType.bodyStrong, maxLines = 1)
            Text(switchLabel, style = TvType.small, color = if (focused) c.bg else c.tx3, maxLines = 1)
        }
    }
}

/** An item of the rail (`.r-it`): pill, light in focus, a bar of the accent when it is the current page. */
@Composable
private fun RailSurface(
    onClick: () -> Unit,
    height: Dp,
    current: Boolean,
    contentFocus: FocusRequester,
    modifier: Modifier,
    startPad: Dp = TvDimens.Rail.Pad,
    content: @Composable (focused: Boolean) -> Unit,
) {
    val c = SlouchcatTheme.colors
    val r = TvDimens.Rail
    var focused by remember { mutableStateOf(false) }
    Box(Modifier.padding(start = r.Start, end = r.End)) {
        if (current && !focused) {
            Box(
                Modifier
                    .align(Alignment.CenterStart)
                    .offset(x = -(TvDimens.s + TvDimens.xs / 2))
                    .size(r.Current, height - r.CurrentInset * 2)
                    .clip(CircleShape)
                    .background(c.accent),
            )
        }
        Surface(
            onClick = onClick,
            modifier = modifier
                .fillMaxWidth()
                .height(height)
                .onPreviewKeyEvent { event ->
                    // Right out of the rail: back to the element the page had, else into the page.
                    if (event.key != Key.DirectionRight || event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                    if (!contentFocus.restoreFocusedChild()) runCatching { contentFocus.requestFocus() }
                    true
                }
                .onFocusChanged { focused = it.isFocused },
            shape = ClickableSurfaceDefaults.shape(shape = RoundedCornerShape(TvDimens.radiusRailItem)),
            colors = ClickableSurfaceDefaults.colors(
                containerColor = Color.Transparent,
                contentColor = if (current) c.tx else c.tx3,
                focusedContainerColor = c.tx,
                focusedContentColor = c.bg,
                pressedContainerColor = c.tx,
                pressedContentColor = c.bg,
            ),
            scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleRail),
        ) {
            Row(Modifier.fillMaxSize().padding(start = startPad), verticalAlignment = Alignment.CenterVertically) { content(focused) }
        }
    }
}

/** The clock at the top right (`#tvclock`), in the device's 12- or 24-hour format. */
@Composable
private fun Clock(modifier: Modifier) {
    val context = LocalContext.current
    val format = remember(context) { DateFormat.getTimeFormat(context) }
    val time by produceState(format.format(Date())) {
        while (true) {
            value = format.format(Date())
            delay(CLOCK_TICK_MS - System.currentTimeMillis() % CLOCK_TICK_MS)
        }
    }
    Text(time, style = TvType.body.copy(fontWeight = androidx.compose.ui.text.font.FontWeight.Medium), color = SlouchcatTheme.colors.tx2, modifier = modifier)
}

/** Where the closed rail's gradient starts to fade (`#rail` background, 55 %). */
private const val RAIL_SOLID = 0.55f

/** The largest number a badge shows in full. */
private const val MAX_COUNT = 99

/** The clock turns over at the full minute. */
private const val CLOCK_TICK_MS = 60_000L
