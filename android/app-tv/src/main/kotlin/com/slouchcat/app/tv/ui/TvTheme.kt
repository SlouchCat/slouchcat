package com.slouchcat.app.tv.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.em
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.darkColorScheme
import com.slouchcat.core.design.SlouchcatColors
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens

/**
 * The ten-foot measures of the TV mockup (`css/app.css` of docs/planning/mockups/26-android-tv),
 * each a token of packages/design-tokens/tokens-tv (half of the mockup's 1920 × 1080 canvas, on
 * the TV's 960 × 540 dp). No number in a page: read them here.
 */
object TvDimens {

    /** `--safe-x`, `--safe-y`: the safe margin of about 5 %. */
    val safeX: Dp = SlouchcatTokens.Tv.Space.SafeX
    val safeY: Dp = SlouchcatTokens.Tv.Space.SafeY

    /** `--cx`: where the content of a page with the rail begins. */
    val contentStart: Dp = SlouchcatTokens.Tv.Space.ContentStart
    val gap: Dp = SlouchcatTokens.Tv.Space.Gap
    val xs: Dp = SlouchcatTokens.Tv.Space.Xs
    val s: Dp = SlouchcatTokens.Tv.Space.S
    val m: Dp = SlouchcatTokens.Tv.Space.M
    val l: Dp = SlouchcatTokens.Tv.Space.L
    val xl: Dp = SlouchcatTokens.Tv.Space.Xl
    val xxl: Dp = SlouchcatTokens.Tv.Space.Xxl
    val xxxl: Dp = SlouchcatTokens.Tv.Space.Xxxl

    val radiusCard: Dp = SlouchcatTokens.Tv.Radius.Card
    val radiusField: Dp = SlouchcatTokens.Tv.Radius.Field
    val radiusMenuItem: Dp = SlouchcatTokens.Tv.Radius.MenuItem
    val radiusIconBox: Dp = SlouchcatTokens.Tv.Radius.IconBox
    val radiusPanel: Dp = SlouchcatTokens.Tv.Radius.Panel
    val radiusDialog: Dp = SlouchcatTokens.Tv.Radius.Dialog
    val radiusBadge: Dp = SlouchcatTokens.Tv.Radius.Badge
    val radiusToast: Dp = SlouchcatTokens.Tv.Radius.Toast
    val radiusRailItem: Dp = SlouchcatTokens.Tv.Radius.RailItem
    val radiusLogo: Dp = SlouchcatTokens.Tv.Radius.Logo

    val borderThin: Dp = SlouchcatTokens.Tv.Border.Thin
    val borderThick: Dp = SlouchcatTokens.Tv.Border.Thick
    val borderFocus: Dp = SlouchcatTokens.Tv.Border.Focus

    val Rail = SlouchcatTokens.Tv.Size.Rail
    val Button = SlouchcatTokens.Tv.Size.Button
    val Field = SlouchcatTokens.Tv.Size.Field
    val Switch = SlouchcatTokens.Tv.Size.Switch
    val Note = SlouchcatTokens.Tv.Size.Note
    val Badge = SlouchcatTokens.Tv.Size.Badge
    val Menu = SlouchcatTokens.Tv.Size.Menu
    val Dialog = SlouchcatTokens.Tv.Size.Dialog
    val Toast = SlouchcatTokens.Tv.Size.Toast
    val Setup = SlouchcatTokens.Tv.Size.Setup
    val Profiles = SlouchcatTokens.Tv.Size.Profiles
    val Settings = SlouchcatTokens.Tv.Size.Settings
    val Card = SlouchcatTokens.Tv.Size.Card
    val Browse = SlouchcatTokens.Tv.Size.Browse
}

/** Scales, opacities and times of the mockup (`--focus-scale`, `--t` …). */
object TvMotion {
    val focusScale: Float = SlouchcatTokens.Tv.Scale.Focus
    val focusScaleSmall: Float = SlouchcatTokens.Tv.Scale.FocusSmall
    val focusScaleRail: Float = SlouchcatTokens.Tv.Scale.FocusRail
    val focusScaleCard: Float = SlouchcatTokens.Tv.Scale.FocusCard
    val railScrim: Float = SlouchcatTokens.Tv.Opacity.ScrimRail
    val dim: Float = SlouchcatTokens.Tv.Opacity.Dim
    val disabled: Float = SlouchcatTokens.Tv.Opacity.Disabled
    val profileRest: Float = SlouchcatTokens.Tv.Opacity.ProfileRest
    val rowRest: Float = SlouchcatTokens.Tv.Opacity.RowRest
    val FOCUS_MS: Int = SlouchcatTokens.Tv.Motion.Focus
    val RAIL_MS: Int = SlouchcatTokens.Tv.Motion.Rail
    val TOAST_MS: Int = SlouchcatTokens.Tv.Motion.Toast
    val EXIT_MS: Int = SlouchcatTokens.Tv.Motion.Exit
}

/** Text styles of the mockup for sofa distance (`--fs-s` 26 px … `--fs-xl` 84 px on the canvas). */
object TvType {
    private val S = SlouchcatTokens.Tv.Font.Size

    /** `--fs-s`: the smallest text on the TV (BAUKASTEN rule 5). */
    val small = TextStyle(fontSize = S.S, lineHeight = S.S * LINE)

    /** `--fs`: body text. */
    val body = TextStyle(fontSize = S.Body, lineHeight = S.Body * LINE)
    val bodyStrong = body.copy(fontWeight = FontWeight.SemiBold)

    /** `--fs-l`: lead texts, menu titles. */
    val large = TextStyle(fontSize = S.L, lineHeight = S.L * LINE)
    val largeStrong = large.copy(fontWeight = FontWeight.SemiBold)
    val h2 = TextStyle(fontSize = S.H2, fontWeight = FontWeight.Bold, lineHeight = S.H2 * TITLE_LINE)
    val h1 = TextStyle(fontSize = S.H1, fontWeight = FontWeight.ExtraBold, lineHeight = S.H1 * TITLE_LINE, letterSpacing = (-0.02).em)
    val xl = TextStyle(fontSize = S.Xl, fontWeight = FontWeight.ExtraBold, lineHeight = S.Xl * TITLE_LINE)
    val button = TextStyle(fontSize = S.Button, fontWeight = FontWeight.SemiBold, lineHeight = S.Button * LINE)
    val buttonSmall = TextStyle(fontSize = S.S, fontWeight = FontWeight.SemiBold, lineHeight = S.S * LINE)
    val field = TextStyle(fontSize = S.Field, lineHeight = S.Field * LINE)
    val label = TextStyle(fontSize = S.S, fontWeight = FontWeight.SemiBold, lineHeight = S.S * LINE)
    val badge = TextStyle(fontSize = S.Badge, fontWeight = FontWeight.SemiBold, lineHeight = S.Badge * LINE)
    val wordmark = TextStyle(fontSize = S.Wordmark, fontWeight = FontWeight.ExtraBold, letterSpacing = (-0.02).em, lineHeight = S.Wordmark * TITLE_LINE)
    val railWordmark = wordmark.copy(fontSize = S.RailWordmark, lineHeight = S.RailWordmark * TITLE_LINE)
    val logo = TextStyle(fontSize = S.Logo, fontWeight = FontWeight.ExtraBold, lineHeight = S.Logo * TITLE_LINE)
    val section = TextStyle(fontSize = S.Section, fontWeight = FontWeight.Bold, letterSpacing = 0.08.em, lineHeight = S.Section * LINE)
    val pin = TextStyle(fontSize = S.Pin, fontWeight = FontWeight.SemiBold, lineHeight = S.Pin * TITLE_LINE)

    private const val LINE = 1.4f
    private const val TITLE_LINE = 1.1f
}

/** The colours of Compose for TV from the same tokens as the phone app (one dark theme). */
private fun tvColorScheme(c: SlouchcatColors) = darkColorScheme(
    primary = c.tx,
    onPrimary = c.bg,
    secondary = c.accent,
    onSecondary = c.onAccent,
    background = c.bg,
    onBackground = c.tx,
    surface = c.bg,
    onSurface = c.tx,
    surfaceVariant = c.s2,
    onSurfaceVariant = c.tx2,
    inverseSurface = c.tx,
    inverseOnSurface = c.bg,
    border = c.frame,
    borderVariant = c.line,
    error = c.danger,
    onError = c.bg,
    scrim = c.scrim,
)

/**
 * The theme of the TV app: the colours of [SlouchcatTheme] (tokens, accent of the profile) and
 * Compose for TV around them. Every TV screen and every screen test is wrapped in it. Text outside
 * a Surface gets `--tx` (K26 stage 0: without it, TV text is black on the dark ground).
 */
@Composable
fun TvTheme(accent: String? = null, content: @Composable () -> Unit) {
    SlouchcatTheme(accent = accent) {
        val colors = SlouchcatTheme.colors
        MaterialTheme(colorScheme = tvColorScheme(colors)) {
            CompositionLocalProvider(LocalContentColor provides colors.tx, content = content)
        }
    }
}
