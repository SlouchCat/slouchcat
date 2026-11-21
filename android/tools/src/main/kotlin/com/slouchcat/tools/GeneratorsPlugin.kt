package com.slouchcat.tools

import org.gradle.api.Plugin
import org.gradle.api.Project

/**
 * Makes the generator tasks of this build available to a module. It registers nothing itself: each
 * module wires its generator into its own sources (core/api, core/strings, core/design).
 */
class GeneratorsPlugin : Plugin<Project> {
    override fun apply(target: Project) = Unit
}
