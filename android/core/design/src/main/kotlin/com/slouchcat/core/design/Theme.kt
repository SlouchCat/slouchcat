package com.slouchcat.core.design

import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp

/**
 * The colours of the phone app under the names of the mockup (`css/app.css`, `:root`), each one a
 * token of packages/design-tokens. Read them as `SlouchcatTheme.colors.tx2`.
 */
@Immutable
data class SlouchcatColors(
    /** `--bg`: the background of every page. */
    val bg: Color = SlouchcatTokens.Color.Bg.Base,
    /** `--elev`: bars, sheets, the drawer. */
    val elev: Color = SlouchcatTokens.Color.Bg.Elevated,
    val s1: Color = SlouchcatTokens.Color.Surface.S1,
    val s2: Color = SlouchcatTokens.Color.Surface.S2,
    val s3: Color = SlouchcatTokens.Color.Surface.S3,
    /** `--tx`: titles and body text. */
    val tx: Color = SlouchcatTokens.Color.Text.Primary,
    /** `--tx2`: supporting text. */
    val tx2: Color = SlouchcatTokens.Color.Text.Secondary,
    /** `--tx3`: hints, captions; at least 8:1 on bg, elev, s1 and s2 (K27 3.2). */
    val tx3: Color = SlouchcatTokens.Color.Text.Muted,
    /** `--line`: dividers. */
    val line: Color = SlouchcatTokens.Color.Border.Subtle,
    /** `--frame`: frames of panels and notes, at least 3:1. */
    val frame: Color = SlouchcatTokens.Color.Border.Frame,
    /** `--ctl`: outlines of controls, at least 3:1. */
    val ctl: Color = SlouchcatTokens.Color.Border.Control,
    val danger: Color = SlouchcatTokens.Color.State.Danger,
    val ok: Color = SlouchcatTokens.Color.State.Success,
    val warn: Color = SlouchcatTokens.Color.State.Warning,
    val info: Color = SlouchcatTokens.Color.Accent.Sky.Base,
    /** The accent of the active profile; ember until a profile is chosen. */
    val accent: Color = SlouchcatTokens.Color.Accent.Ember.Base,
    val onAccent: Color = SlouchcatTokens.Color.Accent.Ember.On,
    /** `--scrim`: behind sheets and the drawer. */
    val scrim: Color = SlouchcatTokens.Color.Bg.Overlay,
    val focus: Color = SlouchcatTokens.Color.State.FocusRing,
)

/** Measures of the mockup (`css/app.css`), for the building blocks and the pages. */
object SlouchcatDimens {
    /** `--gut`: side margin of a page; 24 dp from [WIDE]. */
    val gutter: Dp = SlouchcatTokens.Space.S4
    val gutterWide: Dp = SlouchcatTokens.Space.S6

    /** From this width on a page uses its wide layout (`@container app (min-width: 700px)`). */
    val WIDE: Dp = 700.dp

    /** Smallest tap target (K27 3.2). */
    val touch: Dp = SlouchcatTokens.Space.S12
    val barHeight: Dp = 60.dp
    val fieldHeight: Dp = 54.dp
    val rowHeight: Dp = 60.dp

    /** `--r`: corners of cards, notes and grouped lists. */
    val radius: Dp = 14.dp
    val radiusField: Dp = SlouchcatTokens.Radius.Lg

    /** Width of a form column on wide screens (BAUKASTEN rule 6: at most 560, the setup pane 460). */
    val formMax: Dp = 460.dp
}

/** Text styles of the mockup; the font is the system's (Roboto), as the tokens leave it open. */
object SlouchcatType {
    val wordmark = TextStyle(fontSize = 40.sp, fontWeight = FontWeight.ExtraBold, letterSpacing = (-0.02).em, lineHeight = 40.sp)
    val wordmarkWide = wordmark.copy(fontSize = 46.sp, lineHeight = 46.sp)
    val pageTitle = TextStyle(fontSize = 22.sp, fontWeight = FontWeight.Bold, lineHeight = 28.sp)
    val barTitle = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.SemiBold, lineHeight = 26.sp)
    val sheetTitle = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.Bold, lineHeight = 26.sp)
    val body = TextStyle(fontSize = 16.sp, lineHeight = 23.sp)
    val bodySmall = TextStyle(fontSize = 14.5.sp, lineHeight = 21.sp)
    val lede = TextStyle(fontSize = 15.5.sp, lineHeight = 23.sp)
    val label = TextStyle(fontSize = 13.sp, fontWeight = FontWeight.SemiBold, lineHeight = 18.sp)
    val caption = TextStyle(fontSize = 13.sp, lineHeight = 18.sp)
    val button = TextStyle(fontSize = 15.sp, fontWeight = FontWeight.SemiBold, lineHeight = 20.sp)
    val buttonSmall = button.copy(fontSize = 14.sp)
    val rowTitle = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.SemiBold, lineHeight = 22.sp)
    val section = TextStyle(fontSize = 13.sp, fontWeight = FontWeight.Bold, letterSpacing = 0.06.em, lineHeight = 18.sp)
    val badge = TextStyle(fontSize = 12.sp, fontWeight = FontWeight.SemiBold, lineHeight = 16.sp)
    val chip = TextStyle(fontSize = 14.sp, fontWeight = FontWeight.Medium, lineHeight = 18.sp)
    val strong = TextStyle(fontSize = 16.5.sp, fontWeight = FontWeight.Bold, lineHeight = 22.sp)
}

private val LocalSlouchcatColors = staticCompositionLocalOf { SlouchcatColors() }

object SlouchcatTheme {
    val colors: SlouchcatColors
        @Composable get() = LocalSlouchcatColors.current
}

/** Material 3 colours from the tokens: one theme, dark, no colours from the wallpaper (K27 3.2). */
fun slouchcatColorScheme(c: SlouchcatColors = SlouchcatColors()): ColorScheme = darkColorScheme(
    primary = c.tx,
    onPrimary = c.bg,
    primaryContainer = c.s3,
    onPrimaryContainer = c.tx,
    secondary = c.accent,
    onSecondary = c.onAccent,
    secondaryContainer = c.accent,
    onSecondaryContainer = c.onAccent,
    tertiary = c.info,
    background = c.bg,
    onBackground = c.tx,
    surface = c.bg,
    onSurface = c.tx,
    surfaceVariant = c.s2,
    onSurfaceVariant = c.tx2,
    surfaceContainerLowest = c.bg,
    surfaceContainerLow = c.s1,
    surfaceContainer = c.elev,
    surfaceContainerHigh = c.elev,
    surfaceContainerHighest = c.s3,
    inverseSurface = c.tx,
    inverseOnSurface = c.bg,
    error = c.danger,
    onError = c.bg,
    outline = c.ctl,
    outlineVariant = c.line,
    scrim = c.scrim,
)

private val typography = Typography(
    bodyLarge = SlouchcatType.body,
    bodyMedium = SlouchcatType.bodySmall,
    bodySmall = SlouchcatType.caption,
    labelLarge = SlouchcatType.button,
    titleLarge = SlouchcatType.barTitle,
    headlineSmall = SlouchcatType.pageTitle,
)

/** The theme of the phone app. Wrap every screen (and every screen test) in it. */
@Composable
fun SlouchcatTheme(content: @Composable () -> Unit) {
    val colors = SlouchcatColors()
    CompositionLocalProvider(LocalSlouchcatColors provides colors) {
        MaterialTheme(colorScheme = slouchcatColorScheme(colors), typography = typography, content = content)
    }
}

/** Whether the window is wide enough for the two-column layouts of the mockup (from 700 dp). */
@Composable
fun isWideLayout(): Boolean =
    with(LocalDensity.current) { LocalWindowInfo.current.containerSize.width.toDp() } >= SlouchcatDimens.WIDE

/** Side margin of a page for the current width. */
@Composable
fun gutter(): Dp = if (isWideLayout()) SlouchcatDimens.gutterWide else SlouchcatDimens.gutter
