package com.slouchcat.tools

import org.gradle.api.DefaultTask
import org.gradle.api.GradleException
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.provider.Property
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.Internal
import org.gradle.api.tasks.OutputDirectory
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction
import org.gradle.process.ExecOperations
import java.io.ByteArrayOutputStream
import javax.inject.Inject

/**
 * Runs the Kotlin output of packages/design-tokens (concept 27, 5.5): `node build-kotlin.mjs <dir>`
 * writes `SlouchcatTokens.kt` into [outputDir]. Needs Node and the workspace installed
 * (`pnpm install` at the repository root); the CI job `android` sets both up.
 */
@CacheableTask
abstract class GenerateTokens : DefaultTask() {
    @get:Inject
    abstract val exec: ExecOperations

    /** packages/design-tokens/build-kotlin.mjs */
    @get:InputFile
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val script: RegularFileProperty

    /** The JSON files of packages/design-tokens/tokens. */
    @get:InputFiles
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val tokens: ConfigurableFileCollection

    /** The package folder, for the check that its dependencies are installed. */
    @get:Internal
    abstract val packageDir: DirectoryProperty

    /** The Node binary: `node` from PATH unless SLOUCHCAT_NODE names another one. */
    @get:Input
    abstract val node: Property<String>

    @get:OutputDirectory
    abstract val outputDir: DirectoryProperty

    @TaskAction
    fun generate() {
        val pkg = packageDir.get().asFile
        if (!pkg.resolve("node_modules/style-dictionary").exists()) {
            throw GradleException(
                "GenerateTokens: style-dictionary is not installed in ${pkg.path}; run `pnpm install` at the repository root",
            )
        }
        val out = outputDir.get().asFile
        out.deleteRecursively()
        out.mkdirs()
        val log = ByteArrayOutputStream()
        val result = exec.exec {
            commandLine(node.get(), script.get().asFile.path, out.path)
            workingDir = pkg
            standardOutput = log
            errorOutput = log
            isIgnoreExitValue = true
        }
        if (result.exitValue != 0) {
            throw GradleException("GenerateTokens: node build-kotlin.mjs failed:\n$log")
        }
        logger.info(log.toString())
    }
}
