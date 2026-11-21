package com.slouchcat.lint

import com.android.tools.lint.client.api.IssueRegistry
import com.android.tools.lint.client.api.Vendor
import com.android.tools.lint.detector.api.CURRENT_API
import com.android.tools.lint.detector.api.JavaContext

/** The own rules of the Android apps; registered through the jar manifest (Lint-Registry-v2). */
class SlouchcatIssueRegistry : IssueRegistry() {
    override val issues = listOf(
        HardcodedTextDetector.ISSUE,
        HardcodedColorDetector.ISSUE,
        DirectLogDetector.ISSUE,
    )
    override val api = CURRENT_API
    override val minApi = 14
    override val vendor = Vendor(
        vendorName = "Slouchcat",
        identifier = "slouchcat",
        feedbackUrl = "https://github.com/SlouchCat/slouchcat/issues",
    )
}

/**
 * The rules judge the apps' own code (packages `com.slouchcat.*`). Code the generators write
 * (client, tokens, texts) is exempt.
 */
internal fun JavaContext.isExempt(): Boolean =
    "/build/generated/" in file.invariantSeparatorsPath ||
        uastFile?.packageName?.startsWith("com.slouchcat") != true

private val java.io.File.invariantSeparatorsPath: String get() = path.replace('\\', '/')
