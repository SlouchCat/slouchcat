package com.slouchcat.app.mobile.browse

import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.core.media.MediaData

/**
 * The repositories and counts of this graph (`:core:media`): the server's unless a test put others
 * in place (`MediaData.install(graph, …)`). The counts forget themselves with the profile.
 */
val AppGraph.media: MediaData
    get() = MediaData.of(this) {
        MediaData.api(api, http).also { data -> hooks.onProfileOrSessionEnd { data.clearCounts() } }
    }
