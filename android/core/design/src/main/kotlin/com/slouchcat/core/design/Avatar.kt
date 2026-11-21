package com.slouchcat.core.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** The fill and the text colour of a profile accent (`color.accent.*` of the tokens). */
data class AccentColors(val base: Color, val on: Color)

/** The profile accents of the contract (`Accent`: ember, coral, …) as tokens; unknown or none is ember. */
object ProfileAccents {
    fun of(name: String?): AccentColors = when (name) {
        "coral" -> AccentColors(SlouchcatTokens.Color.Accent.Coral.Base, SlouchcatTokens.Color.Accent.Coral.On)
        "amber" -> AccentColors(SlouchcatTokens.Color.Accent.Amber.Base, SlouchcatTokens.Color.Accent.Amber.On)
        "lime" -> AccentColors(SlouchcatTokens.Color.Accent.Lime.Base, SlouchcatTokens.Color.Accent.Lime.On)
        "teal" -> AccentColors(SlouchcatTokens.Color.Accent.Teal.Base, SlouchcatTokens.Color.Accent.Teal.On)
        "sky" -> AccentColors(SlouchcatTokens.Color.Accent.Sky.Base, SlouchcatTokens.Color.Accent.Sky.On)
        "violet" -> AccentColors(SlouchcatTokens.Color.Accent.Violet.Base, SlouchcatTokens.Color.Accent.Violet.On)
        "rose" -> AccentColors(SlouchcatTokens.Color.Accent.Rose.Base, SlouchcatTokens.Color.Accent.Rose.On)
        else -> AccentColors(SlouchcatTokens.Color.Accent.Ember.Base, SlouchcatTokens.Color.Accent.Ember.On)
    }
}

/**
 * `avatar(profil, grösse)`: the first letter of the profile's name on its accent, round. Decorative:
 * the name stands next to it, so it says nothing to a screen reader.
 */
@Composable
fun Avatar(name: String, accent: String?, modifier: Modifier = Modifier, size: Dp = 56.dp) {
    val colors = ProfileAccents.of(accent)
    val letter = name.trim().firstOrNull()?.uppercaseChar()?.toString().orEmpty()
    val fontSize = with(LocalDensity.current) { (size * 0.42f).toSp() / fontScale }
    Box(
        modifier
            .size(size)
            .clip(CircleShape)
            .background(colors.base),
        contentAlignment = Alignment.Center,
    ) {
        Text(letter, color = colors.on, fontSize = fontSize, fontWeight = FontWeight.Bold, maxLines = 1)
    }
}
