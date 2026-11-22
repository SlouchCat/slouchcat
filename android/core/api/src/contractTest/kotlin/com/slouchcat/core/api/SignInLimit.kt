package com.slouchcat.core.api

import com.slouchcat.core.api.generated.infrastructure.ClientException

/** The server's pot of sign-ins: 10 per minute and address (`too_many_attempts`). */
private const val LIMIT_WINDOW_MS = 61_000L

/**
 * A sign-in of a contract test. All classes share the one server of the task and its pot of sign-ins;
 * with the player's test (K27 stage 5) they need more than ten. A 429 waits once for the minute to
 * pass, as the sign-in flow in the emulator does.
 */
fun <T> withinTheSignInLimit(block: () -> T): T = try {
    block()
} catch (e: RuntimeException) {
    val limited = (e as? ClientException)?.statusCode == 429 || e.message.orEmpty().contains(": 429 ")
    if (!limited) throw e
    Thread.sleep(LIMIT_WINDOW_MS)
    block()
}
