package com.slouchcat.app.mobile.common

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.slouchcat.core.api.crypto.ServerProof
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatDimens
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.SwitchRow
import com.slouchcat.core.design.gutter
import com.slouchcat.core.strings.R

/**
 * «Server nicht bestätigt» (ADR 0040, decision of the build): a quiet notice where the server's
 * data would be, no dialog, no request to sign in. [onCheck] opens [KeyCheckSheet]; only «Mehr ›
 * Server» and, when signed out, the sign-in page offer it.
 */
@Composable
fun ServerUnconfirmedNote(onCheck: (() -> Unit)?, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    Column(
        modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.warn), shape)
            .padding(14.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            SlouchcatIconImage(SlouchcatIcon.Shield, contentDescription = null, size = 20.dp, tint = c.warn, modifier = Modifier.padding(top = 1.dp))
            Column(Modifier.weight(1f).semantics(mergeDescendants = true) {}) {
                Text(stringResource(R.string.app_serverKey_unconfirmed), style = SlouchcatType.rowTitle, color = c.tx, modifier = Modifier.semantics { heading() })
                Text(stringResource(R.string.app_serverKey_unconfirmedBody), style = SlouchcatType.bodySmall, color = c.tx2)
            }
        }
        if (onCheck != null) {
            SlouchcatButton(stringResource(R.string.app_serverKey_check), onClick = onCheck, kind = ButtonKind.Secondary, small = true)
        }
    }
}

/**
 * The explicit confirmation of a new key (section 6): both fingerprints, the two cases, the warning
 * that e-mail and password go to the device with the new key, and the switch «Ich bin in meinem
 * eigenen Netz» before [onConfirm] can be pressed.
 */
@Composable
fun KeyCheckSheet(
    remembered: String?,
    presented: String,
    ownNetwork: Boolean,
    onOwnNetwork: (Boolean) -> Unit,
    confirmLabel: String,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    val title = stringResource(R.string.app_serverKey_title)
    SlouchcatSheet(title = title, onDismiss = onDismiss) {
        SheetHeadIcon(SlouchcatIcon.Shield, warning = true)
        SheetTitle(title, centered = true)
        SheetText(stringResource(R.string.app_serverKey_cases))
        Column(Modifier.padding(horizontal = gutter(), vertical = 12.dp)) {
            DetailRows(
                listOfNotNull(
                    remembered?.let { stringResource(R.string.app_serverKey_remembered) to (ServerProof.fingerprint(it) ?: it) },
                    stringResource(R.string.app_serverKey_presented) to (ServerProof.fingerprint(presented) ?: presented),
                ),
            )
        }
        Note(stringResource(R.string.app_serverKey_passwordGoes), warning = true, modifier = Modifier.padding(horizontal = gutter()))
        SwitchRow(
            stringResource(R.string.app_serverKey_ownNetwork),
            checked = ownNetwork,
            onCheckedChange = onOwnNetwork,
            subtitle = stringResource(R.string.app_serverKey_ownNetworkHint),
        )
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = onDismiss, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            SlouchcatButton(confirmLabel, onClick = onConfirm, kind = ButtonKind.Danger, enabled = ownNetwork, modifier = Modifier.weight(1f))
        }
    }
}
