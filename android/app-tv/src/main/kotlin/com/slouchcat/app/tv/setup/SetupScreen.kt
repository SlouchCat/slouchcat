package com.slouchcat.app.tv.setup

import androidx.compose.foundation.BorderStroke
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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.ui.BadgeTone
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvBadge
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvIcons
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.TvWordmark
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R

/**
 * «Willkommen bei Slouchcat» (mockup `/setup`, photo 02): two ways, with the phone (recommended,
 * the start focus) or by hand. On the first page of the app Back counts towards leaving it.
 */
@Composable
fun SetupScreen(onPhone: () -> Unit, onHand: () -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    Box(modifier.fillMaxSize().tvPage()) {
        TvWordmark(stringResource(R.string.app_name), modifier = Modifier.align(Alignment.TopStart))
        Column(Modifier.align(Alignment.Center), horizontalAlignment = Alignment.CenterHorizontally) {
            Text(stringResource(R.string.tv_setup_welcome), style = TvType.h1, textAlign = TextAlign.Center, modifier = Modifier.semantics { heading() })
            Text(
                stringResource(R.string.tv_setup_lead),
                style = TvType.large,
                color = c.tx2,
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(top = TvDimens.Setup.LeadTop).widthIn(max = TvDimens.Setup.LeadMax),
            )
            Row(Modifier.padding(top = TvDimens.Setup.ChoicesTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.Setup.ChoicesGap)) {
                Choice(
                    icon = SlouchcatIcon.Phone.vector,
                    badge = stringResource(R.string.tv_setup_recommended),
                    recommended = true,
                    title = stringResource(R.string.tv_setup_phoneTitle),
                    text = stringResource(R.string.tv_setup_phoneText),
                    onClick = onPhone,
                    modifier = Modifier.focusKey(focus, "phone"),
                )
                Choice(
                    icon = TvIcons.Keyboard,
                    badge = stringResource(R.string.tv_setup_withoutPhone),
                    recommended = false,
                    title = stringResource(R.string.tv_setup_handTitle),
                    text = stringResource(R.string.tv_setup_handText),
                    onClick = onHand,
                    modifier = Modifier.focusKey(focus, "hand"),
                )
            }
        }
    }
    focus.StartFocus("phone")
}

/** `.su-choice`: a large card; in focus framed light, its icon on the accent. */
@Composable
private fun Choice(icon: ImageVector, badge: String, recommended: Boolean, title: String, text: String, onClick: () -> Unit, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusDialog)
    val s = TvDimens.Setup
    val source = remember { MutableInteractionSource() }
    val focused by source.collectIsFocusedAsState()
    Surface(
        onClick = onClick,
        modifier = modifier.width(s.ChoiceWidth).heightIn(min = s.ChoiceHeight),
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = c.s1,
            contentColor = c.tx,
            focusedContainerColor = c.s2,
            focusedContentColor = c.tx,
            pressedContainerColor = c.s2,
            pressedContentColor = c.tx,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleCard),
        interactionSource = source,
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, c.line), shape = shape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
        ),
    ) {
        Column(
            Modifier.fillMaxWidth().padding(horizontal = s.ChoicePadX, vertical = s.ChoicePadY),
            verticalArrangement = Arrangement.spacedBy(TvDimens.m),
        ) {
            ChoiceIcon(icon, focused)
            TvBadge(badge, tone = if (recommended) BadgeTone.Accent else BadgeTone.Plain)
            Text(title, style = TvType.h2, modifier = Modifier.padding(top = TvDimens.xs / 2))
            Text(text, style = TvType.body, color = c.tx2)
        }
    }
}

@Composable
private fun ChoiceIcon(icon: ImageVector, focused: Boolean) {
    val c = SlouchcatTheme.colors
    Box(
        Modifier.size(TvDimens.Setup.ChoiceIconBox).clip(CircleShape).background(if (focused) c.accent else c.s3),
        contentAlignment = Alignment.Center,
    ) { TvIcon(icon, TvDimens.Setup.ChoiceIcon, tint = if (focused) c.onAccent else c.tx) }
}
