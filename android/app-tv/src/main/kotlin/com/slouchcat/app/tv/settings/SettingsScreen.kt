package com.slouchcat.app.tv.settings

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.tv.material3.Text
import com.slouchcat.app.tv.setup.RelayMenu
import com.slouchcat.app.tv.shell.SettingsOrder
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.ui.ConnectionBadge
import com.slouchcat.app.tv.ui.DetailValue
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvActions
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDetailRows
import com.slouchcat.app.tv.ui.TvDialog
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvMenuItem
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.TvWordmark
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.strings.R

/** A row of «Einstellungen» as the screen draws it: built in, or from a feature ([com.slouchcat.app.tv.shell.SettingsEntry]). */
data class SettingsRow(
    val key: String,
    val order: Int,
    val title: String,
    val subtitle: String?,
    val icon: ImageVector,
    val onClick: () -> Unit,
    val pane: (@Composable ColumnScope.() -> Unit)? = null,
)


/** Which menu of the page is open. */
private enum class SettingsMenu { None, Server, SignOut, About, Relay }

/**
 * «Einstellungen» (mockup `/settings`, photo 36): the rows on the left, on the right a pane about the
 * row in focus. «Server und Konto» opens a menu from the right (photo 62) with «Diesen Fernseher
 * abmelden» and its question (photo 63), «Über» the version, licence and source (photo 64),
 * «Erweitert» the address of the relay. [extra]: the rows of later stages.
 */
@Composable
fun SettingsScreen(
    state: SettingsUiState,
    extra: List<SettingsRow>,
    onSwitchProfile: () -> Unit,
    onSignOut: () -> Unit,
    onRelay: (okhttp3.HttpUrl?) -> Unit,
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    var menu by rememberSaveable { mutableStateOf(SettingsMenu.None) }
    var paneKey by rememberSaveable { mutableStateOf("profile") }
    val server = state.server
    val host = server?.address?.let(ServerAddress::display).orEmpty()
    val plain = stringResource(R.string.tv_hand_notEncrypted)

    val builtIn = listOf(
        SettingsRow(
            "profile", SettingsOrder.SWITCH_PROFILE, stringResource(R.string.tv_shell_switchProfile),
            stringResource(R.string.tv_settings_active, state.profileName), SlouchcatIcon.Users.vector, onSwitchProfile,
        ) { ProfilePane(state) },
        SettingsRow(
            "server", SettingsOrder.SERVER, stringResource(R.string.tv_settings_server),
            if (server?.encrypted == false) "$host · $plain" else host, SlouchcatIcon.Server.vector, { menu = SettingsMenu.Server },
        ) {
            Text(stringResource(R.string.tv_settings_server), style = TvType.h2, modifier = Modifier.semantics { heading() })
            ServerDetails(state)
        },
        SettingsRow(
            "about", SettingsOrder.ABOUT, stringResource(R.string.tv_settings_about),
            stringResource(R.string.tv_settings_aboutVersion, state.appVersion), SlouchcatIcon.Info.vector, { menu = SettingsMenu.About },
        ) { AboutPane(state) },
        SettingsRow(
            "advanced", SettingsOrder.ADVANCED, stringResource(R.string.tv_settings_advanced),
            state.relay.host, SlouchcatIcon.Settings.vector, { menu = SettingsMenu.Relay },
        ) {
            Text(stringResource(R.string.tv_relay_title), style = TvType.h2, modifier = Modifier.semantics { heading() })
            Text(ServerAddress.display(state.relay), style = TvType.largeStrong, modifier = Modifier.padding(top = TvDimens.m))
            Text(stringResource(R.string.tv_relay_lead), style = TvType.body, color = c.tx2, modifier = Modifier.padding(top = TvDimens.s))
        },
    )
    val rows = (builtIn + extra).sortedBy { it.order }

    Column(modifier.fillMaxSize().tvPage()) {
        Text(stringResource(R.string.tv_settings_title), style = TvType.h1, modifier = Modifier.semantics { heading() })
        Row(Modifier.padding(top = TvDimens.Settings.Top), horizontalArrangement = Arrangement.spacedBy(TvDimens.Settings.Gap)) {
            Column(
                Modifier.width(TvDimens.Settings.List).verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(TvDimens.Menu.ListGap),
            ) {
                for (row in rows) {
                    TvMenuItem(
                        title = row.title,
                        subtitle = row.subtitle,
                        icon = row.icon,
                        onClick = row.onClick,
                        modifier = Modifier
                            .focusKey(focus, row.key)
                            .onFocusChanged { if (it.isFocused) paneKey = row.key },
                    )
                }
            }
            val pane = rows.firstOrNull { it.key == paneKey } ?: rows.first()
            Column(
                Modifier
                    .weight(1f)
                    .heightIn(min = TvDimens.Settings.Pane)
                    .background(c.s1, RoundedCornerShape(TvDimens.radiusPanel))
                    .border(BorderStroke(TvDimens.borderThin, c.line), RoundedCornerShape(TvDimens.radiusPanel))
                    .padding(horizontal = TvDimens.Settings.PanePadX, vertical = TvDimens.Settings.PanePadY)
                    .semantics { liveRegion = LiveRegionMode.Polite },
            ) {
                val content = pane.pane
                if (content != null) {
                    content()
                } else {
                    Text(pane.title, style = TvType.h2)
                    pane.subtitle?.let { Text(it, style = TvType.body, color = c.tx2, modifier = Modifier.padding(top = TvDimens.s)) }
                }
            }
        }
    }
    focus.StartFocus("profile")

    when (menu) {
        SettingsMenu.Server -> ServerMenu(state, onClose = { menu = SettingsMenu.None; focus.restore() }, onSignOut = { menu = SettingsMenu.SignOut })
        SettingsMenu.SignOut -> SignOutDialog(
            busy = state.signingOut,
            onConfirm = onSignOut,
            onDismiss = { menu = SettingsMenu.Server },
        )
        SettingsMenu.About -> AboutMenu(state, onClose = { menu = SettingsMenu.None; focus.restore() })
        SettingsMenu.Relay -> RelayMenu(
            current = state.relay,
            onSave = {
                onRelay(it)
                menu = SettingsMenu.None
                focus.restore()
            },
            onDismiss = { menu = SettingsMenu.None; focus.restore() },
        )
        SettingsMenu.None -> Unit
    }
}

@Composable
private fun ProfilePane(state: SettingsUiState) {
    val c = SlouchcatTheme.colors
    Avatar(state.profileName, state.profileAccent, size = TvDimens.Settings.Avatar)
    Text(state.profileName, style = TvType.h2, modifier = Modifier.padding(top = TvDimens.gap))
    Text(stringResource(R.string.tv_settings_switchPane), style = TvType.body, color = c.tx2, modifier = Modifier.padding(top = TvDimens.s))
}

/** The details of server and account (`mdServerDl`): server, connection, account, this TV. */
@Composable
private fun ServerDetails(state: SettingsUiState) {
    val c = SlouchcatTheme.colors
    val server = state.server
    TvDetailRows(
        listOfNotNull(
            stringResource(R.string.tv_pair_server) to { DetailValue(server?.address?.let(ServerAddress::display).orEmpty()) },
            stringResource(R.string.tv_settings_connection) to {
                ConnectionBadge(server?.encrypted != false, stringResource(R.string.tv_hand_encrypted), stringResource(R.string.tv_hand_notEncrypted))
            },
            stringResource(R.string.tv_pair_account) to { DetailValue(state.email) },
            stringResource(R.string.tv_settings_thisTv) to { DetailValue(state.tvName) },
        ),
        keyWidth = TvDimens.Settings.Key,
        modifier = Modifier.padding(top = TvDimens.m),
    )
    Text(stringResource(R.string.tv_settings_tvHint), style = TvType.small, color = c.tx3, modifier = Modifier.padding(top = TvDimens.s))
    if (server?.encrypted == false) {
        TvNote(stringResource(R.string.tv_settings_insecure), tone = NoteTone.Warn, icon = com.slouchcat.app.tv.ui.TvIcons.Unlock, small = true, modifier = Modifier.padding(top = TvDimens.Note.Top))
    }
    if (state.unconfirmed) {
        TvNote(stringResource(R.string.tv_settings_unconfirmedBody), tone = NoteTone.Bad, icon = SlouchcatIcon.Shield.vector, small = true, modifier = Modifier.padding(top = TvDimens.Note.Top))
    }
}

@Composable
private fun AboutPane(state: SettingsUiState) {
    TvWordmark(stringResource(R.string.app_name))
    Text(stringResource(R.string.tv_settings_aboutTitle), style = TvType.h2, modifier = Modifier.padding(top = TvDimens.gap))
    AboutDetails(state)
}

@Composable
private fun AboutDetails(state: SettingsUiState) {
    TvDetailRows(
        listOfNotNull(
            stringResource(R.string.tv_settings_appVersion) to { DetailValue(state.appVersion) },
            state.server?.version?.takeIf { it.isNotEmpty() }?.let { version -> stringResource(R.string.tv_pair_server) to { DetailValue(stringResource(R.string.app_setup_versionValue, version)) } },
            stringResource(R.string.tv_settings_license) to { DetailValue(stringResource(R.string.tv_settings_licenseValue)) },
            stringResource(R.string.tv_settings_source) to { DetailValue(stringResource(R.string.tv_settings_sourceValue)) },
        ),
        keyWidth = TvDimens.Settings.Key,
        modifier = Modifier.padding(top = TvDimens.m),
    )
}

/** `ACT.mdServer` (photo 62): the details and the way to sign this TV out. */
@Composable
private fun ServerMenu(state: SettingsUiState, onClose: () -> Unit, onSignOut: () -> Unit) {
    val close = remember { FocusRequester() }
    TvSideMenu(stringResource(R.string.tv_settings_server), onDismiss = onClose, initialFocus = close) {
        ServerDetails(state)
        TvActions(vertical = true) {
            TvButton(stringResource(R.string.common_close), onClick = onClose, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(close))
            TvButton(stringResource(R.string.tv_settings_signOut), onClick = onSignOut, icon = SlouchcatIcon.Logout.vector, kind = TvButtonKind.Ghost)
        }
    }
}

/** `ACT.mdLogout` (photo 63): the question; «Abbrechen» has the focus. */
@Composable
private fun SignOutDialog(busy: Boolean, onConfirm: () -> Unit, onDismiss: () -> Unit) {
    val c = SlouchcatTheme.colors
    val cancel = remember { FocusRequester() }
    TvDialog(stringResource(R.string.tv_settings_signOutTitle), onDismiss = onDismiss, initialFocus = cancel) {
        Text(stringResource(R.string.tv_settings_signOutBody), style = TvType.large, color = c.tx2)
        TvActions {
            TvButton(stringResource(R.string.tv_settings_signOutConfirm), onClick = onConfirm, icon = SlouchcatIcon.Logout.vector, enabled = !busy)
            TvButton(stringResource(R.string.common_cancel), onClick = onDismiss, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(cancel))
        }
    }
}

/** `ACT.mdAbout` (photo 64): app and server version, address, licence, source. */
@Composable
private fun AboutMenu(state: SettingsUiState, onClose: () -> Unit) {
    val close = remember { FocusRequester() }
    TvSideMenu(stringResource(R.string.tv_settings_about), onDismiss = onClose, initialFocus = close) {
        TvWordmark(stringResource(R.string.app_name))
        Text(stringResource(R.string.tv_settings_aboutTitle), style = TvType.large, modifier = Modifier.padding(top = TvDimens.xs).fillMaxWidth())
        AboutDetails(state)
        state.server?.let {
            TvDetailRows(
                listOf(stringResource(R.string.tv_settings_address) to { DetailValue(ServerAddress.display(it.address)) }),
                keyWidth = TvDimens.Settings.Key,
            )
        }
        TvActions {
            TvButton(stringResource(R.string.common_close), onClick = onClose, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(close))
        }
    }
}
