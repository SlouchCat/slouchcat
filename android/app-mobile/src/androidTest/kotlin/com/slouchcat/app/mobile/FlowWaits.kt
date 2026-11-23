package com.slouchcat.app.mobile

import android.graphics.Bitmap
import androidx.compose.ui.semantics.SemanticsNode
import androidx.compose.ui.test.ComposeTimeoutException
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.isRoot
import androidx.compose.ui.test.junit4.ComposeTestRule
import androidx.compose.ui.test.printToString
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File

/*
 * Waiting in the emulator flows: when a state does not come, the failure says what the screen
 * showed instead (the semantics tree in the message) and keeps a photo of it in the app's files
 * (`failures/`, the e2e script copies it to app-mobile/build/e2e-logs). No longer limits: the
 * wait is the same, only the report is better.
 */

/**
 * The nodes of [matcher] now; `null` while no Compose hierarchy is there at all. That is a moment,
 * not a state: an activity without Compose lies in front (the tap on a notification passes through
 * `PushTapActivity` before `MainActivity` is back), and `fetchSemanticsNodes` throws then instead
 * of answering «none». Only that one exception counts as «not yet»; every other one goes on.
 */
fun ComposeTestRule.nodesOrNull(matcher: SemanticsMatcher, unmerged: Boolean = false): List<SemanticsNode>? = try {
    onAllNodes(matcher, useUnmergedTree = unmerged).fetchSemanticsNodes()
} catch (e: IllegalStateException) {
    if (e.message?.startsWith(NO_HIERARCHY) == true) null else throw e
}

/** A node of [matcher] is there now (no Compose hierarchy: not yet). */
fun ComposeTestRule.isShown(matcher: SemanticsMatcher, unmerged: Boolean = false): Boolean = nodesOrNull(matcher, unmerged)?.isNotEmpty() == true

/** No node of [matcher] is there now, in a Compose hierarchy that is there. */
fun ComposeTestRule.isGone(matcher: SemanticsMatcher, unmerged: Boolean = false): Boolean = nodesOrNull(matcher, unmerged)?.isEmpty() == true

private const val NO_HIERARCHY = "No compose hierarchies found in the app"

/** Waits until a node of [matcher] is there; at the timeout fails with the screen's tree. */
fun ComposeTestRule.waitForNode(matcher: SemanticsMatcher, timeout: Long, what: String = matcher.description) {
    try {
        waitUntil(timeout) { isShown(matcher, unmerged = true) }
    } catch (e: ComposeTimeoutException) {
        throw AssertionError("not shown within $timeout ms: $what\n${screenReport(what)}", e)
    }
}

/** Waits until no node of [matcher] is there; at the timeout fails with the screen's tree. */
fun ComposeTestRule.waitForNoNode(matcher: SemanticsMatcher, timeout: Long, what: String = matcher.description) {
    try {
        waitUntil(timeout) { isGone(matcher) }
    } catch (e: ComposeTimeoutException) {
        throw AssertionError("still shown after $timeout ms: $what\n${screenReport(what)}", e)
    }
}

/** The semantics tree of every window and a photo of the screen, named after [what]. */
fun ComposeTestRule.screenReport(what: String): String {
    val tree = runCatching { onAllNodes(isRoot(), useUnmergedTree = true).printToString(maxDepth = Int.MAX_VALUE) }
        .getOrElse { "no tree: ${it.javaClass.simpleName}" }
    val photo = runCatching {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val bitmap = instrumentation.uiAutomation.takeScreenshot() ?: return@runCatching "no photo"
        val dir = File(instrumentation.targetContext.getExternalFilesDir(null), "failures").apply { mkdirs() }
        val file = File(dir, what.replace(Regex("[^A-Za-z0-9]+"), "-").take(60) + "-" + System.currentTimeMillis() + ".png")
        file.outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
        file.name
    }.getOrElse { "no photo: ${it.javaClass.simpleName}" }
    return "photo: $photo\n$tree"
}
