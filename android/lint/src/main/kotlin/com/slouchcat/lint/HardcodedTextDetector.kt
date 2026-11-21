package com.slouchcat.lint

import com.android.tools.lint.client.api.UElementHandler
import com.android.tools.lint.detector.api.Category
import com.android.tools.lint.detector.api.Detector
import com.android.tools.lint.detector.api.Implementation
import com.android.tools.lint.detector.api.Issue
import com.android.tools.lint.detector.api.JavaContext
import com.android.tools.lint.detector.api.Scope
import com.android.tools.lint.detector.api.Severity
import com.intellij.psi.PsiMethod
import org.jetbrains.uast.UCallExpression
import org.jetbrains.uast.UElement
import org.jetbrains.uast.UExpression
import org.jetbrains.uast.ULiteralExpression
import org.jetbrains.uast.UPolyadicExpression

/**
 * Rule 9, texts: a composable gets its text from the generated resources (core/strings), never as a
 * literal. Reports a string literal or template with a letter in it, passed to a text parameter
 * (`text`, `title`, `label`, `contentDescription` …) of a composable function, ours or Compose's.
 * Literals without a letter (" · ", "") are punctuation, not text.
 */
class HardcodedTextDetector : Detector(), Detector.UastScanner {
    override fun getApplicableUastTypes(): List<Class<out UElement>> = listOf(UCallExpression::class.java)

    override fun createUastHandler(context: JavaContext) = object : UElementHandler() {
        override fun visitCallExpression(node: UCallExpression) {
            if (context.isExempt()) return
            val method = node.resolve() ?: return
            if (!method.isComposable()) return
            for ((argument, parameter) in context.evaluator.computeArgumentMapping(node, method)) {
                if (parameter.name !in TEXT_PARAMETERS) continue
                if (!argument.isTextLiteral()) continue
                context.report(
                    ISSUE,
                    node,
                    context.getLocation(argument),
                    "Fester Text an `${parameter.name}` von `${method.name}`: Text aus den erzeugten Ressourcen nehmen (Regel 9)",
                )
            }
        }
    }

    companion object {
        private val TEXT_PARAMETERS = setOf(
            "text", "title", "subtitle", "label", "placeholder", "hint", "message", "body",
            "contentDescription", "supportingText", "actionLabel", "onClickLabel",
        )

        val ISSUE: Issue = Issue.create(
            id = "SlouchcatHardcodedText",
            briefDescription = "Fester Text in der Oberfläche",
            explanation = "Regel 9: Texte nur aus web/src/locales, als erzeugte Ressourcen aus " +
                "core/strings (stringResource, pluralStringResource).",
            category = Category.I18N,
            priority = 8,
            severity = Severity.ERROR,
            implementation = Implementation(HardcodedTextDetector::class.java, Scope.JAVA_FILE_SCOPE),
        )
    }
}

internal fun PsiMethod.isComposable(): Boolean =
    hasAnnotation("androidx.compose.runtime.Composable") ||
        containingClass?.qualifiedName.orEmpty().startsWith("androidx.compose.") && name.first().isUpperCase()

private fun UExpression.isTextLiteral(): Boolean = when (this) {
    is ULiteralExpression -> (value as? String)?.any(Char::isLetter) == true
    is UPolyadicExpression -> operands.any { it.isTextLiteral() }
    else -> false
}
