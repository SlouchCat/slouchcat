// Colours and measures from packages/design-tokens (concept 27, 5.5), the theme of the phone app and
// the shared building blocks of the mockup (docs/planning/mockups/27-android-handy, BAUKASTEN.md).
import com.slouchcat.tools.GenerateTokens

plugins {
    id("slouchcat.android.library")
    id("slouchcat.android.compose")
    id("slouchcat.generators")
}

android {
    namespace = "com.slouchcat.core.design"
}

val tokensPackage = rootProject.layout.projectDirectory.dir("../packages/design-tokens")
val generateTokens = tasks.register<GenerateTokens>("generateTokens") {
    script.set(tokensPackage.file("build-kotlin.mjs"))
    tokens.from(tokensPackage.dir("tokens").asFileTree.matching { include("**/*.json") })
    // The ten-foot tokens of the TV app (concept 26, 3.4): SlouchcatTokens.Tv.
    tokens.from(tokensPackage.dir("tokens-tv").asFileTree.matching { include("**/*.json") })
    packageDir.set(tokensPackage)
    lockfile.set(rootProject.layout.projectDirectory.file("../pnpm-lock.yaml"))
    node.set(providers.environmentVariable("SLOUCHCAT_NODE").orElse("node"))
    outputDir.set(layout.buildDirectory.dir("generated/slouchcat/tokens"))
}

androidComponents {
    onVariants { variant ->
        variant.sources.kotlin?.addGeneratedSourceDirectory(generateTokens, GenerateTokens::outputDir)
    }
}

dependencies {
    api(project(":core:strings"))
    testImplementation(libs.junit)
}
