package com.slouchcat.app.tv.setup

import androidx.activity.compose.BackHandler
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
import androidx.tv.material3.Text
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.ui.ConnectionBadge
import com.slouchcat.app.tv.ui.DetailValue
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.QrCode
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvBusy
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDetailRows
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvIcons
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R

/**
 * «Mit dem Handy einrichten» (mockup `/setup/phone`, photo 03): steps, the QR code and its time
 * on the right; the page «Verbunden» with server and account (K-9); the problems with the way
 * by hand. [onRelay] opens the address of the relay (advanced, F-21).
 */
@Composable
fun PairScreen(
    state: PairUiState,
    onNewCode: () -> Unit,
    onHand: () -> Unit,
    onRelay: () -> Unit,
    onContinue: () -> Unit,
    onUndo: () -> Unit,
    onInput: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val focus = rememberFocusMemory()
    val phase = state.phase
    BackHandler(enabled = phase is PairPhase.Linked || phase is PairPhase.Problem) {
        if (phase is PairPhase.Linked) onContinue() else onNewCode()
    }
    Box(modifier.fillMaxSize().tvPage().onPreviewKeyEvent { onInput(); false }) {
        when (phase) {
            is PairPhase.Linked -> Linked(phase, state.storing, focus, onContinue, onUndo)
            is PairPhase.Problem -> Problem(phase, state.relay, focus, onHand, onNewCode, onRelay)
            else -> Waiting(phase, focus, onNewCode, onHand, onRelay)
        }
    }
    val start = when (phase) {
        is PairPhase.Linked -> "continue"
        is PairPhase.Problem -> "hand"
        else -> "new"
    }
    focus.StartFocus(start, phase::class)
}

@Composable
private fun Waiting(phase: PairPhase, focus: FocusMemory, onNewCode: () -> Unit, onHand: () -> Unit, onRelay: () -> Unit) {
    val c = SlouchcatTheme.colors
    val s = TvDimens.Setup
    val expired = phase == PairPhase.Expired
    Row(Modifier.fillMaxSize(), horizontalArrangement = Arrangement.spacedBy(s.ColsGap), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(stringResource(R.string.tv_pair_title), style = TvType.h1, modifier = Modifier.semantics { heading() })
            Column(Modifier.padding(top = s.StepsTop), verticalArrangement = Arrangement.spacedBy(s.StepsGap)) {
                Step(1, stringResource(R.string.tv_pair_step1))
                Step(2, stringResource(R.string.tv_pair_step2))
                Step(3, stringResource(R.string.tv_pair_step3))
            }
            Row(
                Modifier.padding(top = s.WaitTop).semantics(mergeDescendants = true) { liveRegion = LiveRegionMode.Polite },
                horizontalArrangement = Arrangement.spacedBy(TvDimens.m),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                when (phase) {
                    PairPhase.Expired -> {
                        TvIcon(SlouchcatIcon.Clock, TvDimens.Note.Icon, tint = c.warn)
                        Text(stringResource(R.string.tv_pair_expired), style = TvType.body, color = c.tx2)
                    }
                    PairPhase.Connecting -> TvBusy(stringResource(R.string.tv_pair_connecting))
                    PairPhase.Starting -> TvBusy(stringResource(R.string.tv_pair_creating))
                    else -> {
                        Pulse()
                        Text(stringResource(R.string.tv_pair_waiting), style = TvType.body, color = c.tx2)
                    }
                }
            }
            Row(Modifier.padding(top = s.ActionsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                TvButton(
                    stringResource(R.string.tv_pair_newCode),
                    onClick = onNewCode,
                    icon = SlouchcatIcon.Refresh.vector,
                    kind = if (expired) TvButtonKind.Primary else TvButtonKind.Standard,
                    enabled = phase != PairPhase.Connecting,
                    modifier = Modifier.focusKey(focus, "new"),
                )
                TvButton(stringResource(R.string.tv_pair_byHand), onClick = onHand, icon = TvIcons.Keyboard, kind = TvButtonKind.Ghost, modifier = Modifier.focusKey(focus, "hand"))
            }
            TvButton(
                stringResource(R.string.tv_pair_relay),
                onClick = onRelay,
                icon = SlouchcatIcon.Settings.vector,
                kind = TvButtonKind.Ghost,
                small = true,
                modifier = Modifier.padding(top = TvDimens.l).focusKey(focus, "relay"),
            )
        }
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(TvDimens.m)) {
            QrBox(phase)
            val time = when (phase) {
                is PairPhase.Waiting -> stringResource(R.string.tv_pair_validFor, minutesSeconds(phase.secondsLeft))
                PairPhase.Expired -> stringResource(R.string.tv_pair_showNew)
                else -> " "
            }
            Text(time, style = TvType.body, color = c.tx2)
        }
    }
}

/** `.su-qrbox`: the code on its light ground; dimmed with «Abgelaufen» once it ran out. */
@Composable
private fun QrBox(phase: PairPhase) {
    val c = SlouchcatTheme.colors
    val s = TvDimens.Setup
    Box(
        Modifier.size(s.Qr).clip(RoundedCornerShape(TvDimens.radiusDialog)).background(c.tx).padding(s.QrPad),
        contentAlignment = Alignment.Center,
    ) {
        when (phase) {
            is PairPhase.Waiting -> QrCode(phase.code, stringResource(R.string.tv_pair_qrLabel), Modifier.fillMaxSize())
            PairPhase.Expired -> Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(TvDimens.s)) {
                TvIcon(SlouchcatIcon.Clock, TvDimens.Menu.IconBox, tint = c.bg)
                Text(stringResource(R.string.tv_pair_expiredShort), style = TvType.largeStrong, color = c.bg)
            }
            else -> androidx.compose.material3.CircularProgressIndicator(color = c.bg, trackColor = c.s3, modifier = Modifier.size(TvDimens.Setup.ChoiceIcon))
        }
    }
}

@Composable
private fun Step(number: Int, text: String) {
    val c = SlouchcatTheme.colors
    Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.gap)) {
        Box(Modifier.size(TvDimens.Setup.Step).clip(CircleShape).background(c.s3), contentAlignment = Alignment.Center) {
            Text(number.toString(), style = TvType.bodyStrong)
        }
        Text(text, style = TvType.large, color = c.tx, modifier = Modifier.widthIn(max = TvDimens.Setup.ListWidth / 2))
    }
}

/** `.su-pulse`: the dot that breathes while the TV waits. */
@Composable
private fun Pulse() {
    val c = SlouchcatTheme.colors
    val transition = rememberInfiniteTransition()
    val alpha = transition.animateFloat(TvMotion.disabled, 1f, infiniteRepeatable(tween(PULSE_MS), RepeatMode.Reverse))
    Box(Modifier.size(TvDimens.Setup.Pulse).alpha(alpha.value).clip(CircleShape).background(c.accent))
}

@Composable
private fun Linked(phase: PairPhase.Linked, storing: Boolean, focus: FocusMemory, onContinue: () -> Unit, onUndo: () -> Unit) {
    val c = SlouchcatTheme.colors
    val s = TvDimens.Setup
    Column(Modifier.fillMaxSize(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
        Box(Modifier.size(s.Check).clip(CircleShape).background(c.ok), contentAlignment = Alignment.Center) {
            TvIcon(SlouchcatIcon.Check, s.CheckIcon, tint = c.bg)
        }
        Text(stringResource(R.string.tv_pair_linked), style = TvType.h1, modifier = Modifier.padding(top = TvDimens.xl).semantics { heading() })
        Text(stringResource(R.string.tv_pair_linkedLead), style = TvType.large, color = c.tx2, textAlign = TextAlign.Center, modifier = Modifier.padding(top = s.LeadTop))
        TvDetailRows(
            listOf(
                stringResource(R.string.tv_pair_server) to {
                    DetailValue(phase.host)
                    ConnectionBadge(phase.encrypted, stringResource(R.string.tv_hand_encrypted), stringResource(R.string.tv_hand_notEncrypted))
                },
                stringResource(R.string.tv_pair_account) to { DetailValue(phase.account) },
                stringResource(R.string.tv_pair_tvName) to { DetailValue(phase.tvName) },
            ),
            keyWidth = s.ListKey,
            rowHeight = s.ListRow,
            modifier = Modifier
                .padding(top = TvDimens.l)
                .widthIn(min = s.ListWidth, max = s.ListWidth)
                .background(c.s1, RoundedCornerShape(TvDimens.radiusPanel))
                .border(BorderStroke(TvDimens.borderThin, c.line), RoundedCornerShape(TvDimens.radiusPanel))
                .padding(horizontal = TvDimens.l, vertical = TvDimens.m),
        )
        Text(stringResource(R.string.tv_pair_notMine), style = TvType.body, color = c.tx3, textAlign = TextAlign.Center, modifier = Modifier.padding(top = TvDimens.m).widthIn(max = TvDimens.Note.Max))
        Row(Modifier.padding(top = s.ActionsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            val label = phase.autoSeconds?.let { stringResource(R.string.tv_pair_continueIn, it) } ?: stringResource(R.string.tv_pair_continue)
            TvButton(label, onClick = onContinue, icon = SlouchcatIcon.Users.vector, kind = TvButtonKind.Primary, enabled = !storing, modifier = Modifier.focusKey(focus, "continue"))
            TvButton(stringResource(R.string.tv_pair_undo), onClick = onUndo, icon = SlouchcatIcon.Close.vector, kind = TvButtonKind.Ghost, enabled = !storing, modifier = Modifier.focusKey(focus, "undo"))
        }
    }
}

@Composable
private fun Problem(phase: PairPhase.Problem, relay: String, focus: FocusMemory, onHand: () -> Unit, onNewCode: () -> Unit, onRelay: () -> Unit) {
    val c = SlouchcatTheme.colors
    val s = TvDimens.Setup
    val relayProblem = phase.kind == PairProblem.RelayUnreachable || phase.kind == PairProblem.RelayBusy
    Column(Modifier.fillMaxSize(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
        Box(Modifier.size(s.Check).clip(CircleShape).background(c.s3), contentAlignment = Alignment.Center) {
            TvIcon(SlouchcatIcon.Alert, s.CheckIcon, tint = c.warn)
        }
        Text(problemTitle(phase.kind), style = TvType.h1, textAlign = TextAlign.Center, modifier = Modifier.padding(top = TvDimens.xl).semantics { heading() })
        val lead = TvType.large
        val leadModifier = Modifier.padding(top = s.LeadTop).widthIn(max = s.LeadMax)
        when (phase.kind) {
            PairProblem.ServerUnreachable -> {
                Text(stringResource(R.string.tv_pair_noServerSent), style = lead, color = c.tx2, textAlign = TextAlign.Center, modifier = leadModifier)
                Text(
                    phase.address.orEmpty(),
                    style = TvType.h2.copy(fontFamily = FontFamily.Monospace),
                    modifier = Modifier
                        .padding(top = TvDimens.l)
                        .background(c.s1, RoundedCornerShape(TvDimens.radiusField))
                        .border(BorderStroke(TvDimens.borderThin, c.frame), RoundedCornerShape(TvDimens.radiusField))
                        .padding(horizontal = TvDimens.xl / 2, vertical = TvDimens.s),
                )
                Text(stringResource(R.string.tv_pair_noServerLead), style = lead, color = c.tx2, textAlign = TextAlign.Center, modifier = leadModifier)
            }
            else -> Text(problemLead(phase, relay), style = lead, color = c.tx2, textAlign = TextAlign.Center, modifier = leadModifier)
        }
        Row(Modifier.padding(top = s.ActionsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            TvButton(stringResource(R.string.tv_pair_byHand), onClick = onHand, icon = TvIcons.Keyboard, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, "hand"))
            TvButton(
                stringResource(if (relayProblem) R.string.tv_pair_retry else R.string.tv_pair_newCode),
                onClick = onNewCode,
                icon = SlouchcatIcon.Refresh.vector,
                kind = TvButtonKind.Ghost,
                modifier = Modifier.focusKey(focus, "new"),
            )
            if (relayProblem) {
                TvButton(stringResource(R.string.tv_pair_relay), onClick = onRelay, icon = SlouchcatIcon.Settings.vector, kind = TvButtonKind.Ghost, modifier = Modifier.focusKey(focus, "relay"))
            }
        }
    }
}

@Composable
private fun problemTitle(kind: PairProblem): String = stringResource(
    when (kind) {
        PairProblem.RelayUnreachable, PairProblem.RelayBusy -> R.string.tv_pair_noRelayTitle
        PairProblem.ServerUnreachable -> R.string.tv_pair_noServerTitle
        PairProblem.ServerTooOld -> R.string.tv_pair_tooOldTitle
        PairProblem.Untrusted -> R.string.tv_pair_untrustedTitle
        PairProblem.AppTooOld -> R.string.app_update_title
        PairProblem.TooManyAttempts, PairProblem.Failed -> R.string.tv_pair_failedTitle
    },
)

@Composable
private fun problemLead(phase: PairPhase.Problem, relay: String): String = when (phase.kind) {
    PairProblem.RelayUnreachable -> stringResource(R.string.tv_pair_noRelayLead, relay)
    PairProblem.RelayBusy -> stringResource(R.string.tv_pair_busyLead)
    PairProblem.ServerTooOld -> stringResource(R.string.app_setup_error_tooOld, phase.serverVersion.orEmpty(), phase.requiredVersion.orEmpty())
    PairProblem.Untrusted -> stringResource(R.string.tv_pair_untrustedLead)
    PairProblem.AppTooOld -> stringResource(R.string.app_update_body)
    PairProblem.TooManyAttempts -> stringResource(R.string.app_signIn_error_tooMany, phase.seconds ?: 0L)
    PairProblem.Failed, PairProblem.ServerUnreachable -> stringResource(R.string.app_state_failed)
}

/** `m:ss` of the time left (`mmss` of the mockup). */
fun minutesSeconds(seconds: Long): String = String.format(java.util.Locale.ROOT, "%d:%02d", seconds / SECONDS_PER_MINUTE, seconds % SECONDS_PER_MINUTE)

private const val SECONDS_PER_MINUTE = 60
private const val PULSE_MS = 1100
