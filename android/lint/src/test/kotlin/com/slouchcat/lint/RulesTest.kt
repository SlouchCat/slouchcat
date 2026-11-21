package com.slouchcat.lint

import com.android.tools.lint.checks.infrastructure.LintDetectorTest
import com.android.tools.lint.checks.infrastructure.TestMode
import com.android.tools.lint.detector.api.Detector
import com.android.tools.lint.detector.api.Issue

/** LintDetectorTest is JUnit 3: test methods are found by their `test` prefix. */
class RulesTest : LintDetectorTest() {
    override fun getDetector(): Detector = HardcodedTextDetector()

    override fun getIssues(): List<Issue> = SlouchcatIssueRegistry().issues

    private fun check(vararg files: com.android.tools.lint.checks.infrastructure.TestFile) =
        lint()
            .files(
                Stubs.composable, Stubs.color, Stubs.text, Stubs.stringResource, Stubs.tokens,
                Stubs.androidLog, Stubs.logSink, *Stubs.more.toTypedArray(), *files,
            )
            .issues(*getIssues().toTypedArray())
            .testModes(TestMode.DEFAULT)
            .allowMissingSdk()
            .run()

    fun testFixedTextInComposablesIsAnError() {
        check(
            kotlin(
                """
                package com.slouchcat.app
                import androidx.compose.material3.Icon
                import androidx.compose.material3.Text
                import androidx.compose.runtime.Composable
                import androidx.compose.ui.res.stringResource

                @Composable
                fun Own(title: String) {}

                @Composable
                fun Page(name: String) {
                    Text("Hallo Welt")
                    Text(text = "Hallo ${'$'}name")
                    Icon(contentDescription = "Server")
                    Own(title = "Fester Titel")
                    Text(stringResource(1))
                    Text(" · ")
                    Text(name)
                    Icon(contentDescription = null)
                }
                """,
            ).indented(),
        ).expectErrorCount(4)
            .expectContains("Fester Text an text von Text")
            .expectContains("Fester Text an contentDescription von Icon")
            .expectContains("Fester Text an title von Own")
    }

    fun testFixedColoursAreAnError() {
        check(
            kotlin(
                """
                package com.slouchcat.app
                import androidx.compose.material3.Text
                import androidx.compose.runtime.Composable
                import androidx.compose.ui.graphics.Color
                import androidx.compose.ui.res.stringResource
                import com.slouchcat.core.design.SlouchcatTokens

                @Composable
                fun Page() {
                    Text(stringResource(1), color = Color(0xFF112233))
                    Text(stringResource(1), color = Color.Red)
                    Text(stringResource(1), color = SlouchcatTokens.Text.Primary)
                    Text(stringResource(1), color = Color.Transparent)
                }
                """,
            ).indented(),
        ).expectErrorCount(2)
            .expectContains("SlouchcatHardcodedColor")
    }

    fun testGeneratedCodeIsExempt() {
        check(
            kotlin(
                "build/generated/slouchcat/tokens/com/slouchcat/core/design/Generated.kt",
                """
                package com.slouchcat.core.design
                import androidx.compose.material3.Text
                import androidx.compose.runtime.Composable
                import androidx.compose.ui.graphics.Color
                object Generated { val Accent = Color(0xFFE8794A) }
                @Composable
                fun Sample() { Text("Generated") }
                """,
            ).indented(),
        ).expectClean()
    }

    fun testLogCallsPastTheLoggerAreAnError() {
        check(
            kotlin(
                """
                package com.slouchcat.app
                import android.util.Log
                import com.slouchcat.core.api.log.LogSink

                fun leak(token: String, error: Throwable) {
                    Log.d("auth", "token " + token)
                    println(token)
                    error.printStackTrace()
                }

                class SecondSink : LogSink {
                    override fun write(priority: Int, tag: String, message: String) {
                        Log.println(priority, tag, message)
                    }
                }
                """,
            ).indented(),
            kotlin(
                """
                package com.slouchcat.app.mobile
                import android.util.Log
                import com.slouchcat.core.api.log.LogSink

                class AndroidLogSink : LogSink {
                    override fun write(priority: Int, tag: String, message: String) {
                        Log.println(priority, tag, message)
                    }
                }
                """,
            ).indented(),
        ).expectErrorCount(4)
            .expectContains("android.util.Log.d umgeht den Logger")
            .expectContains("android.util.Log.println umgeht den Logger")
            .expectContains("println umgeht den Logger")
            .expectContains("printStackTrace umgeht den Logger")
    }

    fun testTextInLabelsSemanticsAndBranches() {
        check(
            kotlin(
                """
                package com.slouchcat.app
                import androidx.compose.material3.SnackbarHostState
                import androidx.compose.material3.Text
                import androidx.compose.runtime.Composable
                import androidx.compose.ui.Modifier
                import androidx.compose.ui.semantics.semantics
                import androidx.compose.foundation.clickable
                import androidx.compose.ui.tooling.preview.Preview

                @Composable
                fun Page(on: Boolean, host: SnackbarHostState) {
                    Text(if (on) "An" else "Aus")
                    Text(when { on -> "Ja"; else -> "Nein" })
                    val m = Modifier.clickable(onClickLabel = "Öffnen") {}.semantics { contentDescription = "Bild" }
                    suspend fun tell() = host.showSnackbar("Gespeichert")
                }

                @Preview
                @Composable
                fun PagePreview() { Text("Vorschau") }
                """,
            ).indented(),
        ).expectErrorCount(5)
            .expectContains("Fester Text an onClickLabel von clickable")
            .expectContains("Fester Text in contentDescription")
            .expectContains("Fester Text an message von showSnackbar")
    }

    fun testColourConstantsAndOtherForms() {
        check(
            kotlin(
                """
                package com.slouchcat.app
                import androidx.compose.ui.graphics.Color
                import androidx.core.graphics.toColorInt

                const val ACCENT = 0xFFE8794A
                val a = Color(ACCENT)
                val b = Color(-1L)
                val c = Color.hsl(20f, 0.5f, 0.5f)
                val d = "#112233".toColorInt()
                """,
            ).indented(),
        ).expectErrorCount(4)
    }
}
