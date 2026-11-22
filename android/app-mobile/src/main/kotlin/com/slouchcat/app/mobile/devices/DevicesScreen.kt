package com.slouchcat.app.mobile.devices

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.common.RowGroup
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.SessionKind
import com.slouchcat.core.design.Badge
import com.slouchcat.core.design.BadgeTone
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.strings.R
import android.icu.text.RelativeDateTimeFormatter
import android.icu.util.ULocale
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import java.time.Duration
import java.time.Instant
import java.time.OffsetDateTime
import java.time.temporal.ChronoUnit
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/**
 * «Meine Geräte» (mockup `/devices`, reference photos 06-devices): every device with a session or
 * downloads of the account, the own one marked; another device can be signed out from the primary
 * profile, the others read with an explanation.
 */
@Composable
fun DevicesScreen(
    state: DevicesUiState,
    snackbar: SnackbarHostState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onAsk: (MyDevice) -> Unit,
    onConfirm: (String) -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
    /** The moment the relative times count from; tests fix it. */
    now: Instant = Instant.now(),
) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            TopBar(stringResource(R.string.nav_devices), navigationLabel = stringResource(R.string.common_back), onNavigation = onBack)
            Column(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = gutter(), vertical = 8.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Column(Modifier.widthIn(max = 728.dp).fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        SlouchcatIconImage(SlouchcatIcon.Info, contentDescription = null, size = 18.dp, tint = c.tx2, modifier = Modifier.padding(top = 2.dp))
                        Text(stringResource(R.string.app_devices_lede), style = SlouchcatType.bodySmall, color = c.tx2)
                    }
                    if (!state.canSignOut) Note(stringResource(R.string.settings_devices_onlyPrimary_body), icon = SlouchcatIcon.Lock)
                    LoadableContent(
                        state.devices,
                        onRetry = onRetry,
                        empty = { EmptyState(stringResource(R.string.settings_devices_none), icon = SlouchcatIcon.Phone) },
                    ) { devices ->
                        RowGroup(rows = devices.map { device -> { DeviceRow(device, state.canSignOut, now, onAsk) } })
                    }
                }
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).padding(16.dp))
    }
    state.asking?.let { AskSheet(it, state, onConfirm, onDismiss) }
}

private fun iconOf(device: MyDevice): SlouchcatIcon = when (device.type) {
    MyDevice.Type.PHONE -> SlouchcatIcon.Phone
    MyDevice.Type.TABLET -> SlouchcatIcon.Tablet
    MyDevice.Type.TV -> SlouchcatIcon.Tv
    else -> SlouchcatIcon.Globe
}

@Composable
private fun nameOf(device: MyDevice): String = device.name?.takeIf { it.isNotBlank() } ?: stringResource(R.string.settings_devices_unnamed)

/** The app session of a tablet is a «Tablet-App», not a «Handy-App» (code review minor 26). */
@Composable
private fun kindOf(device: MyDevice, kind: SessionKind): String = stringResource(
    when {
        kind == SessionKind.APP && device.type == MyDevice.Type.TABLET -> R.string.app_devices_tabletApp
        kind == SessionKind.APP -> R.string.settings_devices_kinds_app
        kind == SessionKind.DEVICE -> R.string.settings_devices_kinds_device
        else -> R.string.settings_devices_kinds_browser
    },
)

/**
 * «heute», «gestern», «vor 3 Tagen», then «am 12. November» (mockup «angemeldet seit heute»), in
 * the language of the app through the platform's ICU (no own wording of times).
 */
@Composable
private fun signedInText(time: OffsetDateTime, now: Instant): String {
    val locale = LocalConfiguration.current.locales[0]
    val zone = ZoneId.systemDefault()
    val days = ChronoUnit.DAYS.between(time.atZoneSameInstant(zone).toLocalDate(), now.atZone(zone).toLocalDate())
    val relative = RelativeDateTimeFormatter.getInstance(ULocale.forLocale(locale))
    return when {
        days <= 0 -> relative.format(RelativeDateTimeFormatter.Direction.THIS, RelativeDateTimeFormatter.AbsoluteUnit.DAY)
        days == 1L -> relative.format(RelativeDateTimeFormatter.Direction.LAST, RelativeDateTimeFormatter.AbsoluteUnit.DAY)
        days < WEEK -> relative.format(days.toDouble(), RelativeDateTimeFormatter.Direction.LAST, RelativeDateTimeFormatter.RelativeUnit.DAYS)
        else -> stringResource(R.string.app_devices_onDate, time.atZoneSameInstant(zone).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.LONG).withLocale(locale)))
    }
}

/** «jetzt», «vor 5 Minuten», «vor 3 Stunden», «vor 2 Tagen», then the date (mockup «Letzter Kontakt: vor 2 Tagen»). */
@Composable
private fun lastSeenText(time: OffsetDateTime, now: Instant): String {
    val locale = LocalConfiguration.current.locales[0]
    val gone = Duration.between(time.toInstant(), now)
    val relative = RelativeDateTimeFormatter.getInstance(ULocale.forLocale(locale))
    val last = RelativeDateTimeFormatter.Direction.LAST
    return when {
        gone.toMinutes() < 1 -> relative.format(RelativeDateTimeFormatter.Direction.PLAIN, RelativeDateTimeFormatter.AbsoluteUnit.NOW)
        gone.toHours() < 1 -> relative.format(gone.toMinutes().toDouble(), last, RelativeDateTimeFormatter.RelativeUnit.MINUTES)
        gone.toDays() < 1 -> relative.format(gone.toHours().toDouble(), last, RelativeDateTimeFormatter.RelativeUnit.HOURS)
        gone.toDays() < WEEK -> relative.format(gone.toDays().toDouble(), last, RelativeDateTimeFormatter.RelativeUnit.DAYS)
        else -> time.atZoneSameInstant(ZoneId.systemDefault()).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM).withLocale(locale))
    }
}

private const val WEEK = 7L

/** `.su-dev`: type, name, «dieses Gerät», session and sign-in, last contact and downloads. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun DeviceRow(device: MyDevice, canSignOut: Boolean, now: Instant, onAsk: (MyDevice) -> Unit) {
    val c = SlouchcatTheme.colors
    val name = nameOf(device)
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 72.dp)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.size(44.dp).clip(RoundedCornerShape(12.dp)).background(c.s2), contentAlignment = Alignment.Center) {
            SlouchcatIconImage(iconOf(device), contentDescription = null, size = 24.dp, tint = c.tx2)
        }
        Column(Modifier.weight(1f).semantics(mergeDescendants = true) {}, verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(name, style = SlouchcatType.rowTitle, color = c.tx)
            // Badge and kind in one line that wraps cleanly (mockup `.su-dev small`).
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                verticalArrangement = Arrangement.spacedBy(2.dp),
                itemVerticalAlignment = Alignment.CenterVertically,
            ) {
                if (device.current) Badge(stringResource(R.string.settings_devices_thisDevice), tone = BadgeTone.Accent)
                val kind = device.sessionKind
                val signedIn = device.signedInAt
                Text(
                    if (kind != null && signedIn != null) {
                        stringResource(R.string.app_devices_signedIn, kindOf(device, kind), signedInText(signedIn, now))
                    } else {
                        stringResource(R.string.settings_devices_downloadsOnly)
                    },
                    style = SlouchcatType.caption,
                    color = c.tx2,
                )
            }
            val lastSeen = stringResource(R.string.app_devices_lastSeen, lastSeenText(device.lastSeenAt, now))
            Text(
                if (device.downloads > 0) {
                    lastSeen + " · " + pluralStringResource(R.plurals.settings_devices_downloads, device.downloads, device.downloads)
                } else {
                    lastSeen
                },
                style = SlouchcatType.caption,
                color = c.tx3,
            )
        }
        if (canSignOut && !device.current) {
            val label = stringResource(R.string.settings_devices_signOut_label, name)
            SlouchcatButton(
                stringResource(R.string.settings_devices_signOut_action),
                onClick = { onAsk(device) },
                kind = ButtonKind.Ghost,
                small = true,
                modifier = Modifier.semantics { contentDescription = label },
            )
        }
    }
}

/** «„Pixel 9“ abmelden?»: what the device loses, then sign it out. */
@Composable
private fun AskSheet(device: MyDevice, state: DevicesUiState, onConfirm: (String) -> Unit, onDismiss: () -> Unit) {
    val name = nameOf(device)
    val unnamed = stringResource(R.string.settings_devices_unnamed)
    val title = stringResource(R.string.settings_devices_signOut_title, name)
    SlouchcatSheet(title = title, onDismiss = onDismiss) {
        SheetHeadIcon(iconOf(device))
        SheetTitle(title, centered = true)
        SheetText(stringResource(R.string.settings_devices_signOut_body), centered = true)
        state.error?.let {
            Note(
                stringResource(
                    when (it) {
                        DeviceSignOutError.PrimaryOnly -> R.string.errors_code_primary_profile_required
                        DeviceSignOutError.Unreachable -> R.string.app_setup_error_unreachable
                        DeviceSignOutError.Failed -> R.string.app_state_failed
                    },
                ),
                warning = true,
                modifier = Modifier.padding(start = gutter(), end = gutter(), top = 12.dp),
            )
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = onDismiss, kind = ButtonKind.Ghost, enabled = !state.signingOut, modifier = Modifier.weight(1f))
            SlouchcatButton(
                stringResource(R.string.settings_devices_signOut_confirm),
                onClick = { onConfirm(unnamed) },
                kind = ButtonKind.Danger,
                busy = state.signingOut,
                modifier = Modifier.weight(1f),
            )
        }
    }
}
