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
import org.jetbrains.uast.UBinaryExpression
import org.jetbrains.uast.UCallExpression
import org.jetbrains.uast.UElement
import org.jetbrains.uast.UExpression
import org.jetbrains.uast.UIfExpression
import org.jetbrains.uast.ULiteralExpression
import org.jetbrains.uast.UMethod
import org.jetbrains.uast.UParenthesizedExpression
import org.jetbrains.uast.UPolyadicExpression
import org.jetbrains.uast.USimpleNameReferenceExpression
import org.jetbrains.uast.USwitchClauseExpressionWithBody
import org.jetbrains.uast.USwitchExpression
import org.jetbrains.uast.UastBinaryOperator
import org.jetbrains.uast.getParentOfType

/**
 * Rule 9, texts: the apps take their texts from the generated resources (core/strings), never as a
 * literal. Reports a string literal or template with a letter in it (also inside `if` and `when`)
 * - passed to a text parameter (`text`, `title`, `label`, `contentDescription` …) of a composable,
 *   ours or Compose's;
 * - passed to a label of any call (`onClickLabel` of `Modifier.clickable`, `actionLabel`, the
 *   `message` of `showSnackbar`);
 * - assigned to a semantics property (`contentDescription = "…"` in `Modifier.semantics { }`).
 * Literals without a letter (" · ", "") are punctuation, not text. `@Preview` functions are exempt:
 * their sample data never reaches a user.
 */
class HardcodedTextDetector : Detector(), Detector.UastScanner {
    override fun getApplicableUastTypes(): List<Class<out UElement>> =
        listOf(UCallExpression::class.java, UBinaryExpression::class.java)

    override fun createUastHandler(context: JavaContext) = object : UElementHandler() {
        override fun visitCallExpression(node: UCallExpression) {
            if (context.isExempt() || node.inPreview()) return
            val method = node.resolve() ?: return
            val composable = method.isComposable()
            for ((argument, parameter) in context.evaluator.computeArgumentMapping(node, method)) {
                val name = parameter.name
                val textParameter = (composable && name in TEXT_PARAMETERS) || name in LABEL_PARAMETERS ||
                    (method.name == "showSnackbar" && name == "message")
                if (!textParameter || !argument.isTextLiteral()) continue
                context.report(
                    ISSUE,
                    node,
                    context.getLocation(argument),
                    "Fester Text an `$name` von `${method.name}`: Text aus den erzeugten Ressourcen nehmen (Regel 9)",
                )
            }
        }

        override fun visitBinaryExpression(node: UBinaryExpression) {
            if (context.isExempt() || node.inPreview()) return
            if (node.operator != UastBinaryOperator.ASSIGN) return
            val target = (node.leftOperand as? USimpleNameReferenceExpression)?.identifier ?: return
            if (target !in SEMANTIC_PROPERTIES || !node.rightOperand.isTextLiteral()) return
            context.report(
                ISSUE,
                node,
                context.getLocation(node.rightOperand),
                "Fester Text in `$target`: Text aus den erzeugten Ressourcen nehmen (Regel 9)",
            )
        }
    }

    companion object {
        private val TEXT_PARAMETERS = setOf(
            "text", "title", "subtitle", "label", "placeholder", "hint", "message", "body",
            "contentDescription", "supportingText", "actionLabel", "onClickLabel",
        )

        /** Labels a screen reader says, on any function (Modifier.clickable, toggleable …). */
        private val LABEL_PARAMETERS = setOf("onClickLabel", "onLongClickLabel", "actionLabel")

        private val SEMANTIC_PROPERTIES = setOf("contentDescription", "stateDescription", "paneTitle")

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

private fun UElement.inPreview(): Boolean =
    getParentOfType<UMethod>(UMethod::class.java)?.uAnnotations.orEmpty()
        .any { it.qualifiedName == "androidx.compose.ui.tooling.preview.Preview" }

private fun UExpression.isTextLiteral(): Boolean = when (this) {
    is ULiteralExpression -> (value as? String)?.any(Char::isLetter) == true
    is UPolyadicExpression -> operands.any { it.isTextLiteral() }
    is UParenthesizedExpression -> expression.isTextLiteral()
    is UIfExpression -> thenExpression?.isTextLiteral() == true || elseExpression?.isTextLiteral() == true
    is USwitchExpression -> body.expressions.filterIsInstance<USwitchClauseExpressionWithBody>()
        .any { clause -> clause.body.expressions.lastOrNull()?.lastValue()?.isTextLiteral() == true }
    else -> false
}

/** The value of the last expression of a `when` branch (`yieldExpression` or the expression itself). */
private fun UExpression.lastValue(): UExpression = when (this) {
    is org.jetbrains.uast.UYieldExpression -> expression ?: this
    is org.jetbrains.uast.UReturnExpression -> returnExpression ?: this
    else -> this
}
