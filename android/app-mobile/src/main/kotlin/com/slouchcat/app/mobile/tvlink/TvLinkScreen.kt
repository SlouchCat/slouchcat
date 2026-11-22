package com.slouchcat.app.mobile.tvlink

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.RowGroup
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.design.BarNavigation
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.ListRow
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SectionTitle
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatTextField
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** What the page does on a tap; one object so the screen stays readable. */
class TvLinkActions(
    val onBack: () -> Unit = {},
    val onScan: () -> Unit = {},
    val onScanned: (String) -> Unit = {},
    val onScanClosed: () -> Unit = {},
    val onScannerUnavailable: () -> Unit = {},
    val onConfirm: () -> Unit = {},
    val onBackToIntro: () -> Unit = {},
    val onSwitchProfile: () -> Unit = {},
    val onDevices: () -> Unit = {},
    val onFinish: () -> Unit = {},
    val onEditRelay: (Boolean) -> Unit = {},
    val onRelay: (HttpUrl?) -> Unit = {},
)

/**
 * «Fernseher verbinden» (mockup `/tv`, reference photo 07-tv): the intro with the three steps, the
 * scanner, the question with account and server, the three lines of the run, «Verbunden», and a
 * failure with its own message. [scanner] draws the camera of the flavour.
 */
@Composable
fun TvLinkScreen(state: TvLinkUiState, scanner: CodeScanner, actions: TvLinkActions, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    if (state.step == TvLinkStep.Scan) {
        ScanPage(scanner, actions, modifier)
        return
    }
    // Steps after the intro go back to it first, as the mockup's arrow does; «Verbunden» leaves.
    val inner = state.step !is TvLinkStep.Intro && state.step !is TvLinkStep.Running
    BackHandler(enabled = inner) { if (state.step is TvLinkStep.Done) actions.onFinish() else actions.onBackToIntro() }
    Column(modifier.fillMaxSize().background(c.bg)) {
        // During the run the bar has no back arrow (mockup); the system's back still leaves the page.
        val running = state.step is TvLinkStep.Running
        TopBar(
            stringResource(R.string.app_tv_title),
            navigation = if (running) BarNavigation.None else BarNavigation.Back,
            navigationLabel = stringResource(R.string.common_back),
            onNavigation = { if (inner) (if (state.step is TvLinkStep.Done) actions.onFinish() else actions.onBackToIntro()) else actions.onBack() },
        )
        Column(
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = gutter(), vertical = 8.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Column(Modifier.widthIn(max = 528.dp).fillMaxWidth().padding(bottom = 24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                when (val step = state.step) {
                    TvLinkStep.Intro -> Intro(state, scanner, actions)
                    is TvLinkStep.Confirm -> Confirm(step, actions)
                    is TvLinkStep.Running -> Run(step.done, null)
                    is TvLinkStep.Failed -> Failure(step, actions)
                    is TvLinkStep.Done -> Done(step, actions)
                    TvLinkStep.Scan -> Unit
                }
            }
        }
    }
    if (state.editingRelay) RelaySheet(state.relay, actions)
}

/** `.su-hero`: the big icon on its rounded surface; [tone] for «ok» and «bad». */
@Composable
private fun Hero(icon: SlouchcatIcon, tone: HeroTone = HeroTone.Plain) {
    val c = SlouchcatTheme.colors
    val tint = when (tone) {
        HeroTone.Plain -> c.tx
        HeroTone.Warn -> c.warn
        HeroTone.Ok -> c.ok
    }
    Box(
        Modifier
            .padding(top = 24.dp, bottom = 20.dp)
            .size(88.dp)
            .clip(RoundedCornerShape(24.dp))
            .background(c.s2)
            .border(BorderStroke(1.dp, c.frame), RoundedCornerShape(24.dp)),
        contentAlignment = Alignment.Center,
    ) {
        SlouchcatIconImage(icon, contentDescription = null, size = 40.dp, tint = tint)
    }
}

private enum class HeroTone { Plain, Warn, Ok }

@Composable
private fun Title(text: String) {
    Text(
        text,
        style = SlouchcatType.pageTitle,
        color = SlouchcatTheme.colors.tx,
        textAlign = TextAlign.Center,
        modifier = Modifier.fillMaxWidth().semantics { heading() },
    )
}

@Composable
private fun Lede(text: String, modifier: Modifier = Modifier) {
    Text(text, style = SlouchcatType.bodySmall, color = SlouchcatTheme.colors.tx2, textAlign = TextAlign.Center, modifier = modifier.fillMaxWidth().padding(top = 8.dp))
}

@Composable
private fun Hint(text: String, modifier: Modifier = Modifier) {
    Text(text, style = SlouchcatType.caption, color = SlouchcatTheme.colors.tx3, textAlign = TextAlign.Center, modifier = modifier.fillMaxWidth().padding(top = 8.dp))
}

/** The page before the scan: what happens, «Scanner öffnen», the TVs that are connected. */
@Composable
private fun Intro(state: TvLinkUiState, scanner: CodeScanner, actions: TvLinkActions) {
    Hero(SlouchcatIcon.Tv)
    Title(stringResource(R.string.app_tv_intro_title))
    Lede(stringResource(R.string.app_tv_intro_lede))
    Column(Modifier.fillMaxWidth().padding(top = 20.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        IntroStep(1, SlouchcatIcon.Tv, stringResource(R.string.app_tv_intro_step1))
        IntroStep(2, SlouchcatIcon.Qr, stringResource(R.string.app_tv_intro_step2))
        IntroStep(3, SlouchcatIcon.Check, stringResource(R.string.app_tv_intro_step3))
    }
    state.scanProblem?.let { problem ->
        Note(problemText(problem), warning = true, modifier = Modifier.padding(top = 16.dp).semantics { liveRegion = LiveRegionMode.Polite })
    }
    if (state.primary) {
        Box(Modifier.fillMaxWidth().padding(top = 20.dp)) {
            SlouchcatButton(stringResource(R.string.app_tv_intro_scan), onClick = actions.onScan, kind = ButtonKind.Accent, leadingIcon = SlouchcatIcon.Qr)
        }
        Hint(stringResource(if (scanner.asksForCamera) R.string.app_tv_intro_cameraHint else R.string.app_tv_intro_googleHint))
    } else {
        Note(stringResource(R.string.app_tv_intro_primaryOnly), icon = SlouchcatIcon.Lock, modifier = Modifier.padding(top = 20.dp))
        SlouchcatButton(
            stringResource(R.string.app_more_switchProfile),
            onClick = actions.onSwitchProfile,
            kind = ButtonKind.Secondary,
            block = true,
            leadingIcon = SlouchcatIcon.Users,
            modifier = Modifier.padding(top = 12.dp),
        )
    }
    if (state.tvs.isNotEmpty()) {
        SectionTitle(stringResource(R.string.app_tv_intro_linked), inset = false, modifier = Modifier.fillMaxWidth().padding(top = 8.dp))
        RowGroup(rows = state.tvs.map { tv -> { TvRow(tv, actions.onDevices) } })
    }
    Row(Modifier.fillMaxWidth().padding(top = 20.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(
            stringResource(R.string.app_tv_relay_line, state.relay.host),
            style = SlouchcatType.caption,
            color = SlouchcatTheme.colors.tx3,
            modifier = Modifier.weight(1f),
        )
        SlouchcatButton(stringResource(R.string.app_tv_relay_change), onClick = { actions.onEditRelay(true) }, kind = ButtonKind.Ghost, small = true)
    }
}

/** `.su-steps li`: the number, the icon and the text in a framed row. */
@Composable
private fun IntroStep(n: Int, icon: SlouchcatIcon, text: String) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(16.dp)
    Row(
        Modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape)
            .heightIn(min = 56.dp)
            .padding(horizontal = 14.dp, vertical = 12.dp)
            .semantics(mergeDescendants = true) {},
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(Modifier.size(28.dp).clip(CircleShape).background(c.tx), contentAlignment = Alignment.Center) {
            Text(n.toString(), style = SlouchcatType.label, color = c.bg)
        }
        SlouchcatIconImage(icon, contentDescription = null, size = 20.dp, tint = c.tx2)
        Text(text, style = SlouchcatType.bodySmall, color = c.tx, modifier = Modifier.weight(1f))
    }
}

@Composable
private fun TvRow(tv: MyDevice, onDevices: () -> Unit) {
    val locale = LocalConfiguration.current.locales[0]
    val since = tv.signedInAt?.atZoneSameInstant(ZoneId.systemDefault())?.format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM).withLocale(locale))
    ListRow(
        title = tv.name?.takeIf { it.isNotBlank() } ?: stringResource(R.string.settings_devices_unnamed),
        icon = SlouchcatIcon.Tv,
        subtitle = since?.let { stringResource(R.string.app_tv_intro_since, it) },
        onClick = onDevices,
    )
}

@Composable
private fun problemText(problem: ScanProblem): String = when (problem) {
    ScanProblem.NotACode -> stringResource(R.string.app_tv_problem_notACode)
    ScanProblem.OtherVersion -> stringResource(R.string.app_tv_problem_otherVersion)
    is ScanProblem.OtherRelay -> stringResource(R.string.app_tv_problem_otherRelay, problem.host)
    ScanProblem.Unavailable -> stringResource(R.string.app_tv_problem_unavailable)
}

/**
 * The scanner (mockup `tvScan`): the camera area with the viewfinder, the close button and the
 * title on top, the hint below. With Google's scanner its own screen lies above all of this.
 */
@Composable
private fun ScanPage(scanner: CodeScanner, actions: TvLinkActions, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    BackHandler(onBack = actions.onScanClosed)
    Box(modifier.fillMaxSize().background(c.bg)) {
        scanner.Camera(
            onCode = actions.onScanned,
            onClose = actions.onScanClosed,
            onUnavailable = actions.onScannerUnavailable,
            modifier = Modifier.fillMaxSize(),
        )
        Viewfinder(Modifier.align(Alignment.Center))
        Row(
            Modifier.fillMaxWidth().padding(start = 4.dp, end = 8.dp).heightIn(min = 60.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            SlouchcatIconButton(SlouchcatIcon.Close, contentDescription = stringResource(R.string.app_tv_scan_close), onClick = actions.onScanClosed)
            Text(stringResource(R.string.app_tv_scan_title), style = SlouchcatType.barTitle, color = c.tx, modifier = Modifier.semantics { heading() })
        }
        Text(
            stringResource(R.string.app_tv_scan_aim),
            style = SlouchcatType.body,
            color = c.tx,
            textAlign = TextAlign.Center,
            modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().padding(horizontal = gutter(), vertical = 32.dp),
        )
    }
}

/** `.su-vf`: four corners around the place the code should be. */
@Composable
private fun Viewfinder(modifier: Modifier) {
    val c = SlouchcatTheme.colors
    Box(modifier.size(240.dp).border(BorderStroke(3.dp, c.tx), RoundedCornerShape(28.dp)))
}

/** The question before anything leaves the phone (K-5): account, server, who confirms. */
@Composable
private fun Confirm(step: TvLinkStep.Confirm, actions: TvLinkActions) {
    val c = SlouchcatTheme.colors
    Hero(SlouchcatIcon.Shield, HeroTone.Warn)
    Title(stringResource(R.string.app_tv_confirm_title))
    Lede(stringResource(R.string.app_tv_confirm_lede))
    val shape = RoundedCornerShape(16.dp)
    Column(
        Modifier
            .fillMaxWidth()
            .padding(top = 20.dp)
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape)
            .padding(horizontal = 16.dp, vertical = 4.dp),
    ) {
        Fact(stringResource(R.string.app_tv_confirm_account), step.account)
        HorizontalDivider(color = c.line)
        Fact(stringResource(R.string.app_tv_confirm_server), step.server)
        HorizontalDivider(color = c.line)
        Fact(
            stringResource(R.string.app_tv_confirm_by),
            stringResource(if (step.primary) R.string.app_tv_confirm_primary else R.string.app_tv_confirm_profile, step.profile),
        )
    }
    if (step.plainHttp) Note(stringResource(R.string.app_tv_confirm_plainHttp), warning = true, modifier = Modifier.padding(top = 12.dp))
    Hint(stringResource(R.string.app_tv_confirm_hint), Modifier.padding(top = 4.dp))
    Row(Modifier.fillMaxWidth().padding(top = 20.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        SlouchcatButton(stringResource(R.string.common_cancel), onClick = actions.onBackToIntro, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
        SlouchcatButton(stringResource(R.string.app_tv_confirm_go), onClick = actions.onConfirm, kind = ButtonKind.Accent, modifier = Modifier.weight(1f))
    }
}

/** `.su-dl div`: the term at the start, its value at the end, read as one. */
@Composable
private fun Fact(term: String, value: String) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier.fillMaxWidth().heightIn(min = 44.dp).padding(vertical = 8.dp).semantics(mergeDescendants = true) {},
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(term, style = SlouchcatType.bodySmall, color = c.tx2)
        Text(value, style = SlouchcatType.bodySmall.copy(fontWeight = FontWeight.Bold), color = c.tx, textAlign = TextAlign.End, modifier = Modifier.weight(1f))
    }
}

private val RUN_LINES = listOf(R.string.app_tv_run_voucher, R.string.app_tv_run_handedOver, R.string.app_tv_run_waiting)

/** `.su-run`: the three lines, done, busy, failed or waiting; [failedAt] marks the one that failed. */
@Composable
private fun Run(done: Int, failedAt: Int?) {
    val c = SlouchcatTheme.colors
    Hero(if (failedAt != null) SlouchcatIcon.Alert else SlouchcatIcon.Link, if (failedAt != null) HeroTone.Warn else HeroTone.Plain)
    Title(stringResource(if (failedAt != null) R.string.app_tv_run_failedTitle else R.string.app_tv_run_title))
    val shape = RoundedCornerShape(16.dp)
    Column(
        Modifier
            .fillMaxWidth()
            .padding(top = 20.dp)
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape)
            .padding(horizontal = 16.dp, vertical = 4.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
    ) {
        RUN_LINES.forEachIndexed { index, line ->
            if (index > 0) HorizontalDivider(color = c.line)
            val status = when {
                index == failedAt -> LineStatus.Failed
                index < done -> LineStatus.Done
                index == done && failedAt == null -> LineStatus.Busy
                else -> LineStatus.Pending
            }
            val text = stringResource(line)
            val said = stringResource(
                when (status) {
                    LineStatus.Done -> R.string.app_tv_run_done
                    LineStatus.Busy -> R.string.app_tv_run_busy
                    LineStatus.Failed -> R.string.app_tv_run_failed
                    LineStatus.Pending -> R.string.app_tv_run_pending
                },
            )
            Row(
                Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(vertical = 8.dp).semantics(mergeDescendants = true) { contentDescription = "$text: $said" },
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                // `.su-ri`: a filled circle when done or failed, a spinner while busy, a ring while waiting.
                val circle = Modifier.size(26.dp).clip(CircleShape)
                when (status) {
                    LineStatus.Done -> Box(circle.background(c.ok), contentAlignment = Alignment.Center) {
                        SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 16.dp, tint = c.bg)
                    }
                    LineStatus.Failed -> Box(circle.background(c.warn), contentAlignment = Alignment.Center) {
                        SlouchcatIconImage(SlouchcatIcon.Close, contentDescription = null, size = 16.dp, tint = c.bg)
                    }
                    LineStatus.Busy -> Box(Modifier.size(26.dp), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator(Modifier.size(20.dp), color = c.tx2, strokeWidth = 2.dp)
                    }
                    LineStatus.Pending -> Box(circle.border(BorderStroke(2.dp, c.ctl), CircleShape))
                }
                Text(
                    text,
                    style = if (status == LineStatus.Failed) SlouchcatType.body.copy(fontWeight = FontWeight.Bold) else SlouchcatType.body,
                    color = if (status == LineStatus.Pending) c.tx3 else c.tx,
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

private enum class LineStatus { Done, Busy, Failed, Pending }

/** A run that ended without «Verbunden»: the lines, the message, what the user can do now. */
@Composable
private fun Failure(step: TvLinkStep.Failed, actions: TvLinkActions) {
    Run(step.at, step.at)
    val failure = step.failure
    val (text, icon) = when (failure) {
        TvLinkFailure.Occupied -> stringResource(R.string.app_tv_fail_occupied) to SlouchcatIcon.Alert
        TvLinkFailure.Expired -> stringResource(R.string.app_tv_fail_expired) to SlouchcatIcon.Clock
        TvLinkFailure.TimedOut -> stringResource(R.string.app_tv_fail_timedOut) to SlouchcatIcon.Clock
        TvLinkFailure.RelayUnreachable -> stringResource(R.string.app_tv_fail_relay) to SlouchcatIcon.Alert
        TvLinkFailure.ServerOffline -> stringResource(R.string.app_tv_fail_server) to SlouchcatIcon.Alert
        TvLinkFailure.PrimaryOnly -> stringResource(R.string.app_tv_intro_primaryOnly) to SlouchcatIcon.Lock
        TvLinkFailure.NotAllowed -> stringResource(R.string.app_tv_fail_notAllowed) to SlouchcatIcon.Lock
        is TvLinkFailure.TooManyAttempts -> stringResource(R.string.errors_code_too_many_attempts) to SlouchcatIcon.Clock
        TvLinkFailure.ServerUnconfirmed -> stringResource(R.string.app_serverKey_unconfirmedBody) to SlouchcatIcon.Shield
        is TvLinkFailure.Left -> stringResource(if (failure.delivered) R.string.app_tv_fail_leftAfter else R.string.app_tv_fail_leftBefore) to SlouchcatIcon.Info
        TvLinkFailure.Failed -> stringResource(R.string.app_state_failed) to SlouchcatIcon.Alert
    }
    Note(text, warning = true, icon = icon, modifier = Modifier.padding(top = 16.dp))
    if (failure == TvLinkFailure.Occupied) Hint(stringResource(R.string.app_tv_fail_lapses))
    val again = failure in setOf(TvLinkFailure.Expired, TvLinkFailure.TimedOut, TvLinkFailure.ServerOffline) ||
        failure is TvLinkFailure.TooManyAttempts || failure == TvLinkFailure.Left(delivered = false)
    if (again) {
        Row(Modifier.fillMaxWidth().padding(top = 20.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = actions.onBackToIntro, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            SlouchcatButton(stringResource(R.string.app_tv_fail_again), onClick = actions.onScan, leadingIcon = SlouchcatIcon.Qr, modifier = Modifier.weight(1f))
        }
    } else {
        Row(Modifier.fillMaxWidth().padding(top = 20.dp), horizontalArrangement = Arrangement.Start) {
            if (failure == TvLinkFailure.Left(delivered = true)) {
                SlouchcatButton(stringResource(R.string.nav_devices), onClick = actions.onDevices, kind = ButtonKind.Ghost, modifier = Modifier.padding(end = 10.dp))
            }
            SlouchcatButton(stringResource(R.string.app_tv_fail_ok), onClick = actions.onBackToIntro)
        }
    }
}

@Composable
private fun Done(step: TvLinkStep.Done, actions: TvLinkActions) {
    val c = SlouchcatTheme.colors
    Hero(SlouchcatIcon.Check, HeroTone.Ok)
    Title(stringResource(R.string.app_tv_done_title))
    Row(Modifier.padding(top = 10.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        SlouchcatIconImage(SlouchcatIcon.Tv, contentDescription = null, size = 20.dp, tint = c.tx2)
        Text(step.name ?: stringResource(R.string.settings_devices_unnamed), style = SlouchcatType.strong, color = c.tx)
    }
    Lede(stringResource(R.string.app_tv_done_lede))
    Row(Modifier.fillMaxWidth().padding(top = 20.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        SlouchcatButton(stringResource(R.string.nav_devices), onClick = actions.onDevices, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
        SlouchcatButton(stringResource(R.string.app_tv_done_finish), onClick = actions.onFinish, modifier = Modifier.weight(1f))
    }
}

/** «Adresse der Vermittlung» (F-21): the same texts as on the TV; https only. */
@Composable
private fun RelaySheet(current: HttpUrl, actions: TvLinkActions) {
    var text by rememberSaveable { mutableStateOf(ServerAddress.display(current)) }
    var invalid by rememberSaveable { mutableStateOf(false) }
    val title = stringResource(R.string.tv_relay_title)
    SlouchcatSheet(title = title, onDismiss = { actions.onEditRelay(false) }) {
        SheetTitle(title)
        SheetText(stringResource(R.string.tv_relay_lead))
        SlouchcatTextField(
            value = text,
            onValueChange = {
                text = it
                invalid = false
            },
            label = stringResource(R.string.tv_relay_label),
            placeholder = RelaySettings.DEFAULT.host,
            error = if (invalid) stringResource(R.string.tv_relay_invalid) else null,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, autoCorrectEnabled = false),
            modifier = Modifier.padding(horizontal = gutter()).padding(top = 12.dp).imePadding(),
        )
        SheetActions {
            SlouchcatButton(stringResource(R.string.tv_relay_reset), onClick = { actions.onRelay(null) }, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            SlouchcatButton(
                stringResource(R.string.tv_relay_save),
                onClick = {
                    val relay = parseRelay(text)
                    if (relay == null) invalid = true else actions.onRelay(relay)
                },
                modifier = Modifier.weight(1f),
            )
        }
    }
}
