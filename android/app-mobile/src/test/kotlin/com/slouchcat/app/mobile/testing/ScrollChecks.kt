package com.slouchcat.app.mobile.testing

import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.junit4.ComposeTestRule
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertTrue

/**
 * [assertEveryControlLabelledAndTappable] for a page longer than the screen (a detail page): each
 * control is brought into view first, so one below the fold is measured where the reader meets it,
 * not clipped to nothing by the scroll container. The page's vertical scroll moves first, then the
 * row the control lies in (`performScrollTo` alone moves only the closest scroll parent).
 */
fun ComposeTestRule.assertEveryControlLabelledAndTappableWhenScrolled() {
    val matcher = hasClickAction() or hasSetTextAction()
    val count = onAllNodes(matcher).fetchSemanticsNodes().size
    assertTrue("no control on the screen", count > 0)
    for (i in 0 until count) {
        val node = reveal(onAllNodes(matcher)[i])
        val config = node.fetchSemanticsNode().config
        val label = buildList {
            config.getOrNull(SemanticsProperties.ContentDescription)?.let { addAll(it) }
            config.getOrNull(SemanticsProperties.Text)?.let { texts -> addAll(texts.map { it.text }) }
        }.filter { it.isNotBlank() }
        assertTrue("control without a label: $config", label.isNotEmpty())
        val bounds = node.fetchSemanticsNode().touchBoundsInRoot
        with(density) {
            val minimum = 48.dp.toPx() - 0.5f
            assertTrue("«${label.first()}» is ${bounds.width.toDp()} wide", bounds.width >= minimum)
            assertTrue("«${label.first()}» is ${bounds.height.toDp()} high", bounds.height >= minimum)
        }
    }
}

private val vertical = SemanticsMatcher.keyIsDefined(SemanticsProperties.VerticalScrollAxisRange) and SemanticsMatcher.keyIsDefined(SemanticsActions.ScrollBy)

/**
 * Brings [node] into view: the tallest vertical scroll of the screen (the page) moves it to the
 * middle, then the row it lies in. Returns [node] for the next action.
 */
fun ComposeTestRule.reveal(node: SemanticsNodeInteraction): SemanticsNodeInteraction {
    val pages = onAllNodes(vertical).fetchSemanticsNodes()
    val page = pages.indices.maxByOrNull { pages[it].size.height }
    if (page != null) {
        val target = node.fetchSemanticsNode()
        val middle = pages[page].boundsInRoot.top + pages[page].size.height / 2f
        val offset = target.positionInRoot.y + target.size.height / 2f - middle
        onAllNodes(vertical)[page].performSemanticsAction(SemanticsActions.ScrollBy) { it(0f, offset) }
    }
    // A control outside any row stays where it is.
    runCatching { node.performScrollTo() }
    return node
}
