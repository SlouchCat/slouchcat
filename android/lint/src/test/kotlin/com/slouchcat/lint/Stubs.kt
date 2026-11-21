package com.slouchcat.lint

import com.android.tools.lint.checks.infrastructure.LintDetectorTest.kotlin
import com.android.tools.lint.checks.infrastructure.TestFile

/** Just enough of Compose, Android and the own logger for the rules to resolve calls. */
object Stubs {
    val composable: TestFile = kotlin(
        """
        package androidx.compose.runtime
        annotation class Composable
        """,
    ).indented()

    val color: TestFile = kotlin(
        """
        package androidx.compose.ui.graphics
        @JvmInline
        value class Color(val value: ULong) {
            companion object {
                val Red = Color(0xFFFF0000UL)
                val Transparent = Color(0UL)
                val Unspecified = Color(16UL)
                fun hsl(hue: Float, saturation: Float, lightness: Float): Color = Color(0UL)
            }
        }
        fun Color(color: Long): Color = Color(color.toULong())
        """,
    ).indented()

    val text: TestFile = kotlin(
        """
        package androidx.compose.material3
        import androidx.compose.runtime.Composable
        import androidx.compose.ui.graphics.Color
        @Composable
        fun Text(text: String, color: Color = Color.Unspecified) {}
        @Composable
        fun Icon(contentDescription: String?) {}
        """,
    ).indented()

    val stringResource: TestFile = kotlin(
        """
        package androidx.compose.ui.res
        import androidx.compose.runtime.Composable
        @Composable
        fun stringResource(id: Int): String = ""
        """,
    ).indented()

    val tokens: TestFile = kotlin(
        """
        package com.slouchcat.core.design
        object SlouchcatTokens {
            object Text { val Primary = androidx.compose.ui.graphics.Color.Transparent }
        }
        """,
    ).indented()

    val androidLog: TestFile = kotlin(
        """
        package android.util
        object Log {
            @JvmStatic fun d(tag: String, msg: String): Int = 0
            @JvmStatic fun println(priority: Int, tag: String, msg: String): Int = 0
        }
        """,
    ).indented()

    /** Modifier with clickable and semantics, the snackbar host, @Preview and toColorInt. */
    val modifier: TestFile = kotlin(
        """
        package androidx.compose.ui
        interface Modifier { companion object : Modifier }
        """,
    ).indented()

    val more: List<TestFile> by lazy { listOf(
        modifier,
        kotlin(
            """
            package androidx.compose.foundation
            import androidx.compose.ui.Modifier
            fun Modifier.clickable(onClickLabel: String? = null, onClick: () -> Unit): Modifier = this
            """,
        ).indented(),
        kotlin(
            """
            package androidx.compose.ui.semantics
            import androidx.compose.ui.Modifier
            interface SemanticsPropertyReceiver { var contentDescription: String }
            fun Modifier.semantics(properties: SemanticsPropertyReceiver.() -> Unit): Modifier = this
            """,
        ).indented(),
        kotlin(
            """
            package androidx.compose.material3
            class SnackbarHostState { suspend fun showSnackbar(message: String, actionLabel: String? = null): Unit = Unit }
            """,
        ).indented(),
        kotlin(
            """
            package androidx.compose.ui.tooling.preview
            annotation class Preview
            """,
        ).indented(),
        kotlin(
            """
            package androidx.core.graphics
            fun String.toColorInt(): Int = 0
            """,
        ).indented(),
    ) }

    val logSink: TestFile = kotlin(
        """
        package com.slouchcat.core.api.log
        interface LogSink { fun write(priority: Int, tag: String, message: String) }
        """,
    ).indented()
}
