package com.slouchcat.lint

import com.android.tools.lint.client.api.UElementHandler
import com.android.tools.lint.detector.api.Category
import com.android.tools.lint.detector.api.ConstantEvaluator
import com.android.tools.lint.detector.api.Detector
import com.android.tools.lint.detector.api.Implementation
import com.android.tools.lint.detector.api.Issue
import com.android.tools.lint.detector.api.JavaContext
import com.android.tools.lint.detector.api.Scope
import com.android.tools.lint.detector.api.Severity
import com.intellij.psi.PsiMember
import org.jetbrains.uast.UCallExpression
import org.jetbrains.uast.UElement
import org.jetbrains.uast.ULiteralExpression
import org.jetbrains.uast.UQualifiedReferenceExpression
import org.jetbrains.uast.USimpleNameReferenceExpression

/**
 * Rule 9, colours: a colour comes from the generated tokens (`SlouchcatTokens`) or the theme. Reports
 * `Color(<constant>)`, `Color.hsl/hsv(…)`, the named constants `Color.Red` … of Compose,
 * `"#…".toColorInt()` and `android.graphics.Color`
 * built from literals (`parseColor("#…")`, `rgb(…)`, `argb(…)`). `Color.Transparent` and
 * `Color.Unspecified` are not colours and stay allowed.
 */
class HardcodedColorDetector : Detector(), Detector.UastScanner {
    override fun getApplicableUastTypes(): List<Class<out UElement>> =
        listOf(UCallExpression::class.java, UQualifiedReferenceExpression::class.java)

    override fun createUastHandler(context: JavaContext) = object : UElementHandler() {
        override fun visitCallExpression(node: UCallExpression) {
            if (context.isExempt()) return
            val name = node.methodName ?: return
            val owner = node.resolve()?.containingClass?.qualifiedName.orEmpty()
            // Literals and anything the compiler knows (`Color(ACCENT)`, `Color(-1)`).
            val constantArgs = node.valueArguments.isNotEmpty() &&
                node.valueArguments.all { it is ULiteralExpression || ConstantEvaluator.evaluate(context, it) != null }
            val compose = name in setOf("Color", "hsl", "hsv") && owner.startsWith(COMPOSE_GRAPHICS)
            val platform = owner == "android.graphics.Color" && name in setOf("parseColor", "rgb", "argb", "valueOf")
            // "#112233".toColorInt() (androidx.core.graphics)
            val colorInt = name == "toColorInt" && node.receiver?.let { ConstantEvaluator.evaluate(context, it) is String } == true
            if (((compose || platform) && constantArgs) || colorInt) {
                context.report(ISSUE, node, context.getLocation(node), MESSAGE)
            }
        }

        override fun visitQualifiedReferenceExpression(node: UQualifiedReferenceExpression) {
            if (context.isExempt()) return
            val receiver = node.receiver as? USimpleNameReferenceExpression ?: return
            if (receiver.identifier != "Color") return
            val selector = node.selector.asSourceString()
            if (selector.isEmpty() || !selector.first().isUpperCase() || selector in ALLOWED) return
            // Color.Red is a getter on the companion of an inline class; UAST does not always
            // resolve it, so the imports of the file decide when it does not.
            val owner = (node.resolve() as? PsiMember)?.containingClass?.qualifiedName
            val compose = owner?.startsWith("$COMPOSE_GRAPHICS.Color")
                ?: context.uastFile?.imports.orEmpty().any { it.asSourceString().endsWith("$COMPOSE_GRAPHICS.Color") }
            if (compose) context.report(ISSUE, node, context.getLocation(node), MESSAGE)
        }
    }

    companion object {
        private const val COMPOSE_GRAPHICS = "androidx.compose.ui.graphics"
        private const val MESSAGE = "Fester Farbwert: Farbe aus `SlouchcatTokens` oder dem Thema nehmen (Regel 9)"
        private val ALLOWED = setOf("Transparent", "Unspecified")

        val ISSUE: Issue = Issue.create(
            id = "SlouchcatHardcodedColor",
            briefDescription = "Feste Farbe in der Oberfläche",
            explanation = "Regel 9: Farben nur aus den Tokens (packages/design-tokens, erzeugt als " +
                "SlouchcatTokens in core/design) oder aus dem Thema der App.",
            category = Category.CORRECTNESS,
            priority = 8,
            severity = Severity.ERROR,
            implementation = Implementation(HardcodedColorDetector::class.java, Scope.JAVA_FILE_SCOPE),
        )
    }
}
