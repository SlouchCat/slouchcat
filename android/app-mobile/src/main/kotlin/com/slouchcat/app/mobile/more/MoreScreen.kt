package com.slouchcat.app.mobile.more

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.DetailRows
import com.slouchcat.app.mobile.common.KeyCheckSheet
import com.slouchcat.app.mobile.common.ServerUnconfirmedNote
import com.slouchcat.app.mobile.common.RowGroup
import com.slouchcat.app.mobile.common.StaticRow
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.app.mobile.shell.ShellTopBar
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.ListRow
import com.slouchcat.core.design.SectionTitle
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
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.strings.R

/** A row of «Mehr» from a feature, resolved for the page. */
data class MoreRow(
    val key: String,
    val section: MoreSection,
    val icon: SlouchcatIcon,
    val title: String,
    val subtitle: String?,
    val onClick: () -> Unit,
)

/**
 * «Mehr» (mockup `/more`, reference photos 05-more, 70, 71): profile, the warning about an
 * unencrypted connection, the features' rows by section, server and connection, «Abmelden».
 * Two columns from 700 dp as in the mockup (`.su-cols`).
 */
@Composable
fun MoreScreen(
    state: MoreUiState,
    rows: List<MoreRow>,
    appVersion: String,
    onSwitchProfile: () -> Unit,
    onServer: () -> Unit,
    onSignOutAsk: () -> Unit,
    onChoice: (Int, Boolean) -> Unit,
    onSignOut: () -> Unit,
    onDismissSheet: () -> Unit,
    modifier: Modifier = Modifier,
    onChangeServer: () -> Unit = {},
    onKeyCheck: () -> Unit = {},
    onOwnNetwork: (Boolean) -> Unit = {},
    onConfirmKey: () -> Unit = {},
) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    Column(modifier.fillMaxSize().background(c.bg)) {
        ShellTopBar(stringResource(R.string.app_more_title))
        Column(
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = gutter(), vertical = 8.dp),
        ) {
            ProfileCard(state, onSwitchProfile)
            if (state.newKey != null) ServerUnconfirmedNote(onCheck = onKeyCheck, modifier = Modifier.padding(top = 14.dp))
            if (state.server?.encrypted == false) InsecureNote(Modifier.padding(top = 14.dp))
            val library = rowsOf(rows, MoreSection.Library)
            val devices = rowsOf(rows, MoreSection.Devices)
            val settings = rowsOf(rows, MoreSection.Settings)
            if (wide) {
                Row(Modifier.fillMaxWidth().padding(top = 4.dp), horizontalArrangement = Arrangement.spacedBy(40.dp)) {
                    Column(Modifier.weight(1f)) {
                        Section(R.string.app_more_section_library, library)
                        ServerSection(state, appVersion, onServer, onSignOutAsk)
                    }
                    Column(Modifier.weight(1f)) {
                        Section(R.string.app_more_section_devices, devices)
                        Section(R.string.app_more_section_settings, settings)
                    }
                }
            } else {
                Section(R.string.app_more_section_library, library)
                Section(R.string.app_more_section_devices, devices)
                Section(R.string.app_more_section_settings, settings)
                ServerSection(state, appVersion, onServer, onSignOutAsk)
            }
        }
    }
    when (state.sheet) {
        MoreSheet.Server -> ServerSheet(state, onDismissSheet, onChange = onChangeServer)
        MoreSheet.SignOut -> SignOutSheet(state, onChoice, onSignOut, onDismissSheet)
        MoreSheet.KeyCheck -> state.newKey?.let { presented ->
            KeyCheckSheet(
                remembered = state.rememberedKey,
                presented = presented,
                ownNetwork = state.ownNetwork,
                onOwnNetwork = onOwnNetwork,
                confirmLabel = stringResource(R.string.app_serverKey_signIn),
                onConfirm = onConfirmKey,
                onDismiss = onDismissSheet,
            )
        }
        null -> Unit
    }
}

private fun rowsOf(rows: List<MoreRow>, section: MoreSection) = rows.filter { it.section == section }

@Composable
private fun Section(title: Int, rows: List<MoreRow>) {
    if (rows.isEmpty()) return
    SectionTitle(stringResource(title), inset = false)
    RowGroup(rows = rows.map { row -> { ListRow(row.title, icon = row.icon, subtitle = row.subtitle, onClick = row.onClick) } })
}

/** `.su-me`: avatar, name, role and account, «Profil wechseln». */
@Composable
private fun ProfileCard(state: MoreUiState, onSwitchProfile: () -> Unit) {
    val c = SlouchcatTheme.colors
    val profile = state.profile ?: return
    val shape = RoundedCornerShape(18.dp)
    val role = stringResource(if (profile.isPrimary) R.string.profiles_picker_primary else R.string.nav_profileRole_other)
    Row(
        Modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape)
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Avatar(profile.name, profile.accent, size = 56.dp)
        Column(Modifier.weight(1f).semantics(mergeDescendants = true) {}) {
            Text(profile.name, style = SlouchcatType.strong.copy(fontSize = SlouchcatType.barTitle.fontSize), color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Text(stringResource(R.string.app_more_profileLine, role, state.email), style = SlouchcatType.caption, color = c.tx2, maxLines = 2, overflow = TextOverflow.Ellipsis)
        }
        SlouchcatButton(
            stringResource(R.string.app_more_switchProfile),
            onClick = onSwitchProfile,
            kind = ButtonKind.Ghost,
            small = true,
            leadingIcon = SlouchcatIcon.Users,
        )
    }
}

/** `.note.warn.su-http`: stays as long as the connection is unencrypted (concept 27 section 6). */
@Composable
private fun InsecureNote(modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    val title = stringResource(R.string.app_more_insecureTitle)
    val body = stringResource(R.string.app_more_insecureBody)
    Row(
        modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.warn), shape)
            .padding(14.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        SlouchcatIconImage(SlouchcatIcon.Alert, contentDescription = null, size = 20.dp, tint = c.warn, modifier = Modifier.padding(top = 1.dp))
        Text(
            buildAnnotatedString {
                withStyle(SpanStyle(fontWeight = FontWeight.Bold, color = c.tx)) { append(title) }
                append(" ")
                append(body)
            },
            style = SlouchcatType.bodySmall,
            color = c.tx2,
        )
    }
}

/** «Server»: the server (opens its sheet) and the connection, then «Abmelden» and the version of the app. */
@Composable
private fun ServerSection(state: MoreUiState, appVersion: String, onServer: () -> Unit, onSignOutAsk: () -> Unit) {
    val c = SlouchcatTheme.colors
    val server = state.server ?: return
    val address = ServerAddress.display(server.address)
    SectionTitle(stringResource(R.string.app_more_section_server), inset = false)
    val connection = when {
        !server.encrypted -> stringResource(R.string.app_setup_notEncrypted)
        server.userCertificate -> stringResource(R.string.app_tls_userCertificate)
        else -> stringResource(R.string.app_setup_encrypted)
    }
    RowGroup(
        rows = listOf(
            { ListRow(server.address.host, icon = SlouchcatIcon.Server, subtitle = stringResource(R.string.app_more_serverLine, server.version, address), onClick = onServer) },
            {
                StaticRow(
                    connection,
                    icon = if (server.encrypted) SlouchcatIcon.Lock else SlouchcatIcon.Alert,
                    subtitle = stringResource(if (server.encrypted) R.string.app_more_https else R.string.app_more_plain),
                    iconTint = if (server.encrypted && !server.userCertificate) c.ok else c.warn,
                )
            },
        ),
    )
    SlouchcatButton(
        stringResource(R.string.nav_logout),
        onClick = onSignOutAsk,
        kind = ButtonKind.Danger,
        block = true,
        leadingIcon = SlouchcatIcon.Logout,
        modifier = Modifier.padding(top = 22.dp),
    )
    Text(
        stringResource(R.string.app_more_footer, appVersion),
        style = SlouchcatType.caption,
        color = c.tx2,
        modifier = Modifier.padding(top = 10.dp, bottom = 24.dp).fillMaxWidth(),
    )
}

/** The sheet «Server» (reference photo 71): address, version, connection, account. */
@Composable
private fun ServerSheet(state: MoreUiState, onClose: () -> Unit, onChange: () -> Unit) {
    val server = state.server ?: return
    val title = stringResource(R.string.app_more_serverSheet_title)
    SlouchcatSheet(title = title, onDismiss = onClose) {
        SheetTitle(title)
        Column(Modifier.padding(horizontal = gutter())) {
            DetailRows(
                listOf(
                    stringResource(R.string.app_setup_address) to ServerAddress.display(server.address),
                    stringResource(R.string.app_setup_version) to stringResource(R.string.app_setup_versionValue, server.version),
                    stringResource(R.string.app_more_serverSheet_connection) to
                        stringResource(if (server.encrypted) R.string.app_more_serverSheet_encrypted else R.string.app_more_serverSheet_notEncrypted),
                    stringResource(R.string.nav_account) to state.email,
                ),
            )
        }
        SheetText(stringResource(R.string.app_more_serverSheet_hint))
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_close), onClick = onClose, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            SlouchcatButton(stringResource(R.string.app_more_serverSheet_change), onClick = onChange, kind = ButtonKind.Secondary, modifier = Modifier.weight(1f))
        }
    }
}

/** The sheet «Abmelden?» (reference photo 70); later stages add their questions as switches. */
@Composable
private fun SignOutSheet(state: MoreUiState, onChoice: (Int, Boolean) -> Unit, onSignOut: () -> Unit, onClose: () -> Unit) {
    val title = stringResource(R.string.app_signOut_title)
    SlouchcatSheet(title = title, onDismiss = onClose) {
        SheetHeadIcon(SlouchcatIcon.Logout)
        SheetTitle(title, centered = true)
        SheetText(stringResource(R.string.app_signOut_body), centered = true)
        state.choices.forEachIndexed { index, choice ->
            SwitchRow(
                stringResource(choice.question.title),
                checked = choice.answer,
                onCheckedChange = { onChoice(index, it) },
                subtitle = choice.question.subtitle,
                enabled = !state.signingOut,
            )
        }
        Text(
            stringResource(R.string.app_signOut_hint),
            style = SlouchcatType.caption,
            color = SlouchcatTheme.colors.tx2,
            modifier = Modifier.padding(start = gutter(), end = gutter(), top = 10.dp),
        )
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = onClose, kind = ButtonKind.Ghost, enabled = !state.signingOut, modifier = Modifier.weight(1f))
            SlouchcatButton(
                stringResource(if (state.signingOut) R.string.app_signOut_busy else R.string.nav_logout),
                onClick = onSignOut,
                kind = ButtonKind.Danger,
                busy = state.signingOut,
                modifier = Modifier.weight(1f),
            )
        }
    }
}
