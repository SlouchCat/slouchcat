package com.slouchcat.app.tv.ui

import androidx.compose.ui.graphics.vector.ImageVector
import com.slouchcat.core.design.slouchcatIcon

/**
 * The icons of the TV mockup (`ICONS` in docs/planning/mockups/26-android-tv/js/core.js) that the
 * shared set `SlouchcatIcon` of the phone mockup lacks. A TV page that needs another one adds it
 * in a file of its own area, not here (parallel branches).
 */
object TvIcons {
    val Keyboard: ImageVector by lazy {
        slouchcatIcon("keyboard", """<rect x="2.5" y="6" width="19" height="12" rx="2"/><path d="M6 10h.01M9.5 10h.01M13 10h.01M16.5 10h.01M6 14h.01M18 14h.01M9 14h6"/>""")
    }
    val Unlock: ImageVector by lazy {
        slouchcatIcon("unlock", """<rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 7.5-1.9"/>""")
    }
    val Edit: ImageVector by lazy {
        slouchcatIcon("edit", """<path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13.5 6.5 4 4"/>""")
    }
    val Backspace: ImageVector by lazy {
        slouchcatIcon("backspace", """<path d="M21 5H9l-6 7 6 7h12z"/><path d="m12 9 6 6m0-6-6 6"/>""")
    }
    val Power: ImageVector by lazy {
        slouchcatIcon("power", """<path d="M12 3v9M6.3 6.3a8 8 0 1 0 11.4 0"/>""")
    }
}
