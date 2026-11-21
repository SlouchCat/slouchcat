package com.slouchcat.app.mobile.common

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import com.slouchcat.core.api.SessionEvent
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.strings.R

/**
 * The one place that reacts to [SessionEvents] (concept 27, 11.5 and section 6):
 * - «App zu alt» shows the notice to update the app;
 * - «Sitzung weg» calls [onSignedOut] (forget the session, show the sign-in; the address stays);
 * - «Der Server hat einen neuen Schlüssel» explains it and offers to sign in again
 *   ([onConfirmKey]); «Später» keeps everything as it is, the gate keeps sending nothing.
 */
@Composable
fun SessionEventHandler(events: SessionEvents, onSignedOut: () -> Unit, onConfirmKey: () -> Unit) {
    var tooOld by rememberSaveable { mutableStateOf(false) }
    var newKey by rememberSaveable { mutableStateOf(false) }
    val signedOut by rememberUpdatedState(onSignedOut)
    LaunchedEffect(events) {
        events.events.collect { event ->
            when (event) {
                SessionEvent.AppTooOld -> tooOld = true
                SessionEvent.SignedOut -> signedOut()
                SessionEvent.ServerKeyChanged -> newKey = true
            }
        }
    }
    if (tooOld) {
        val title = stringResource(R.string.app_update_title)
        SlouchcatSheet(title = title, onDismiss = { tooOld = false }) {
            SheetHeadIcon(SlouchcatIcon.Alert, warning = true)
            SheetTitle(title, centered = true)
            SheetText(stringResource(R.string.app_update_body), centered = true)
            SheetActions {
                SlouchcatButton(stringResource(R.string.app_update_close), onClick = { tooOld = false }, modifier = Modifier.weight(1f))
            }
        }
    }
    if (newKey && !tooOld) {
        ServerKeySheet(onLater = { newKey = false }, onSignIn = {
            newKey = false
            onConfirmKey()
        })
    }
}

/** «Der Server hat einen neuen Schlüssel» (concept 27 section 6, ADR 0040). */
@Composable
fun ServerKeySheet(onLater: () -> Unit, onSignIn: () -> Unit) {
    val title = stringResource(R.string.app_serverKey_title)
    SlouchcatSheet(title = title, onDismiss = onLater) {
        SheetHeadIcon(SlouchcatIcon.Shield, warning = true)
        SheetTitle(title, centered = true)
        SheetText(stringResource(R.string.app_serverKey_body), centered = true)
        SheetActions {
            SlouchcatButton(stringResource(R.string.app_serverKey_later), onClick = onLater, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            SlouchcatButton(stringResource(R.string.app_serverKey_signIn), onClick = onSignIn, modifier = Modifier.weight(1f))
        }
    }
}
