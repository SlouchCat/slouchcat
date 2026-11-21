package com.slouchcat.app.mobile.testing

import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.junit4.ComposeTestRule
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertTrue

/**
 * Rule 8 for the phone app, the part every screen test shares: every control on the screen (all
 * that can be tapped or typed into) has a name a screen reader says and a tap target of at least
 * 48 x 48 dp (K27 3.2). Call it in every `XScreenTest`, once per state and width.
 */
fun ComposeTestRule.assertEveryControlLabelledAndTappable() {
    val nodes = onAllNodes(hasClickAction() or hasSetTextAction()).fetchSemanticsNodes()
    assertTrue("no control on the screen", nodes.isNotEmpty())
    for (node in nodes) {
        val config = node.config
        val label = buildList {
            config.getOrNull(SemanticsProperties.ContentDescription)?.let { addAll(it) }
            config.getOrNull(SemanticsProperties.Text)?.let { texts -> addAll(texts.map { it.text }) }
        }.filter { it.isNotBlank() }
        assertTrue("control without a label: $config", label.isNotEmpty())
        val bounds = node.touchBoundsInRoot
        with(density) {
            val minimum = 48.dp.toPx() - 0.5f
            assertTrue("«${label.first()}» is ${bounds.width.toDp()} wide", bounds.width >= minimum)
            assertTrue("«${label.first()}» is ${bounds.height.toDp()} high", bounds.height >= minimum)
        }
    }
}
