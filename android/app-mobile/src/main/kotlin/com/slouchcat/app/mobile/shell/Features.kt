package com.slouchcat.app.mobile.shell

import com.slouchcat.app.mobile.more.MoreFeature

/**
 * Every feature of the phone app, one line each (concept 27, section 13: one or more per stage).
 * The order here does not matter: targets sort by `TopDestination.order`, rows by section and
 * order. A stage adds its feature here and nowhere else in the shell.
 */
val features: List<Feature> = listOf(
    MoreFeature,
)
