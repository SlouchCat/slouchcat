package com.slouchcat.app.mobile.shell

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * In-app jumps (code review M-4): a notification (stage 8) or a page asks for a page by its
 * address; the jump waits until a profile is chosen, then [Navigator.open] builds the stack under
 * its target and marks it. Nothing outside the app reaches it: the manifest exports no deep link.
 *
 * ```
 * graph.links.request("item/42")      // the feature whose link() knows the address
 * graph.links.request(AppLink("start", listOf(ItemRoute(42))))
 * ```
 */
class AppLinks(private val registry: ShellRegistry) {
    private val _pending = MutableStateFlow<AppLink?>(null)
    val pending: StateFlow<AppLink?> = _pending.asStateFlow()

    fun request(link: AppLink) {
        _pending.value = link
    }

    /** `false` when no feature knows [address]. */
    fun request(address: String): Boolean {
        val link = registry.link(address) ?: return false
        request(link)
        return true
    }

    fun consume() {
        _pending.value = null
    }
}
