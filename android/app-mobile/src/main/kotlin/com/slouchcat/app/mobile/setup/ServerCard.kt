package com.slouchcat.app.mobile.setup

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.core.design.Badge
import com.slouchcat.core.design.BadgeTone
import com.slouchcat.core.design.SlouchcatDimens
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl

/**
 * The server as the app found it (`.su-result` in the mockup): host and version, the address,
 * the version again as a row, and whether the connection is encrypted. [found] adds the badge.
 */
@Composable
fun ServerCard(address: HttpUrl, version: String, encrypted: Boolean, found: Boolean, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    val versionText = stringResource(R.string.app_setup_versionValue, version)
    Column(
        modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape)
            .padding(16.dp)
            .semantics(mergeDescendants = true) {},
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Box(Modifier.size(44.dp).clip(RoundedCornerShape(12.dp)).background(c.s3), contentAlignment = Alignment.Center) {
                SlouchcatIconImage(SlouchcatIcon.Server, contentDescription = null, size = 22.dp, tint = c.tx)
            }
            Column(Modifier.weight(1f)) {
                Text(address.host, style = SlouchcatType.strong, color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(versionText, style = SlouchcatType.caption, color = c.tx3)
            }
            if (found) Badge(stringResource(R.string.app_setup_found), tone = BadgeTone.Ok, icon = SlouchcatIcon.Check)
        }
        Spacer(Modifier.height(14.dp))
        DetailRow(stringResource(R.string.app_setup_address), ServerAddress.display(address))
        DetailRow(stringResource(R.string.app_setup_version), versionText)
        HorizontalDivider(color = c.line)
        Row(
            Modifier.padding(top = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            val tint = if (encrypted) c.ok else c.warn
            SlouchcatIconImage(if (encrypted) SlouchcatIcon.Lock else SlouchcatIcon.Alert, contentDescription = null, size = 18.dp, tint = tint)
            Text(
                stringResource(if (encrypted) R.string.app_setup_encrypted else R.string.app_setup_notEncrypted),
                style = SlouchcatType.bodySmall.copy(fontWeight = SlouchcatType.strong.fontWeight),
                color = tint,
            )
        }
    }
}

/** `.su-dl` row: name left, value right, a line above. */
@Composable
private fun DetailRow(name: String, value: String) {
    val c = SlouchcatTheme.colors
    HorizontalDivider(color = c.line)
    Row(Modifier.fillMaxWidth().padding(vertical = 10.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
        Text(name, style = SlouchcatType.bodySmall, color = c.tx3)
        Text(
            value,
            style = SlouchcatType.bodySmall.copy(fontWeight = SlouchcatType.button.fontWeight),
            color = c.tx,
            textAlign = TextAlign.End,
            modifier = Modifier.weight(1f),
        )
    }
}
