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
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.strings.R

/**
 * The one place that reacts to [SessionEvents] (concept 27, 11.5): «App zu alt» shows the notice
 * to update the app, «Sitzung weg» calls [onSignedOut] with the session epoch of the request that
 * met the 401. A new key of the server is no event: it shows quietly where the server's data
 * would be, and only «Mehr › Server» offers to confirm it (ADR 0040).
 */
@Composable
fun SessionEventHandler(events: SessionEvents, onSignedOut: (epoch: Long) -> Unit) {
    var tooOld by rememberSaveable { mutableStateOf(false) }
    val signedOut by rememberUpdatedState(onSignedOut)
    LaunchedEffect(events) {
        events.events.collect { reported ->
            when (reported.event) {
                SessionEvent.AppTooOld -> tooOld = true
                SessionEvent.SignedOut -> signedOut(reported.epoch)
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
}
