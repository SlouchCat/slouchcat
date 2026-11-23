package com.slouchcat.app.mobile.tvlink

/**
 * The camera of one opening of the scanner (code review M-1). The camera provider arrives later
 * than the page asks for it; when the scanner is closed first, nothing is bound any more. Closing
 * unbinds only the scanner's own use cases (never another part of the app), and only then releases
 * the analysis thread. All calls on the main thread.
 *
 * [bind] binds the use cases with the provider and says whether it did; [unbind] undoes exactly
 * that; [release] stops what the frames were analysed on.
 */
class CameraLease<P : Any>(
    private val bind: (P) -> Boolean,
    private val unbind: (P) -> Unit,
    private val release: () -> Unit,
) {
    private var closed = false
    private var bound: P? = null

    /** The provider is there: binds unless the scanner is closed already. */
    fun onProvider(provider: P) {
        if (closed || bound != null) return
        if (bind(provider)) bound = provider
    }

    /** The scanner is closed: the camera is free from here, whatever arrives later. */
    fun close() {
        if (closed) return
        closed = true
        bound?.let(unbind)
        bound = null
        release()
    }

    val isBound: Boolean get() = bound != null
}
