package com.slouchcat.lint

import com.android.tools.lint.detector.api.Category
import com.android.tools.lint.detector.api.Detector
import com.android.tools.lint.detector.api.Implementation
import com.android.tools.lint.detector.api.Issue
import com.android.tools.lint.detector.api.JavaContext
import com.android.tools.lint.detector.api.Scope
import com.android.tools.lint.detector.api.Severity
import com.intellij.psi.PsiMethod
import org.jetbrains.uast.UCallExpression
import org.jetbrains.uast.UClass
import org.jetbrains.uast.getParentOfType

/**
 * Rule 10 for the apps (concept 27, 5.6): token, voucher and device id never reach a log. Every log
 * line goes through the redacting logger `com.slouchcat.core.api.log.Log`; this rule reports the
 * ways around it: `android.util.Log`, `println`/`print` (System.out lands in logcat) and
 * `printStackTrace`. The one place that may call `android.util.Log` is a `LogSink`: it receives
 * lines the logger has already redacted.
 */
class DirectLogDetector : Detector(), Detector.UastScanner {
    override fun getApplicableMethodNames() =
        listOf("v", "d", "i", "w", "e", "wtf", "println", "print", "printStackTrace")

    override fun visitMethodCall(context: JavaContext, node: UCallExpression, method: PsiMethod) {
        if (context.isExempt()) return
        val evaluator = context.evaluator
        val direct = when {
            evaluator.isMemberInClass(method, "android.util.Log") -> "android.util.Log.${method.name}"
            method.name in setOf("println", "print") &&
                (evaluator.isMemberInClass(method, "kotlin.io.ConsoleKt") || evaluator.isMemberInClass(method, "java.io.PrintStream")) ->
                method.name
            method.name == "printStackTrace" && node.valueArgumentCount == 0 -> "printStackTrace"
            else -> return
        }
        val owner = node.getParentOfType<UClass>(UClass::class.java)
        if (owner != null && evaluator.inheritsFrom(owner.javaPsi, LOG_SINK, false)) return
        context.report(
            ISSUE,
            node,
            context.getLocation(node),
            "`$direct` umgeht den Logger: `com.slouchcat.core.api.log.Log` nehmen, er entfernt Token, Gutschein und Gerätekennung (Regel 10)",
        )
    }

    companion object {
        private const val LOG_SINK = "com.slouchcat.core.api.log.LogSink"

        val ISSUE: Issue = Issue.create(
            id = "SlouchcatDirectLog",
            briefDescription = "Log-Aufruf am Logger vorbei",
            explanation = "Regel 10: Token, Gutschein und Gerätekennung erscheinen in keinem Log der App. " +
                "Geloggt wird nur über com.slouchcat.core.api.log.Log, das diese Werte entfernt; " +
                "android.util.Log ruft nur ein LogSink auf.",
            category = Category.SECURITY,
            priority = 9,
            severity = Severity.ERROR,
            implementation = Implementation(DirectLogDetector::class.java, Scope.JAVA_FILE_SCOPE),
        )
    }
}
