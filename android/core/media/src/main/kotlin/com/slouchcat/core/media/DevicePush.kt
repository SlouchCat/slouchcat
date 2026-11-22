package com.slouchcat.core.media

import kotlinx.coroutines.flow.StateFlow

/**
 * Where the push subscription of this device stands (concept 27, 7.3; stage 8). The phone app
 * of the bundle `play` has push through the push relay; the bundle `foss`, a `play` build without
 * Firebase values and the TV app have none ([NotInBuild]).
 */
sealed interface DevicePushState {
    /** This build has no push: the page offers nothing that cannot work. */
    data object NotInBuild : DevicePushState

    /** The server has no push set up (`getPushPublicKey` 404). */
    data object ServerOff : DevicePushState

    /** Switched off on this device; the server keeps no subscription of it. */
    data object Off : DevicePushState

    /**
     * Switched off, but the server has not confirmed the removal yet (no connection): the device
     * can no longer read a message, the app removes the subscription at the next contact.
     */
    data object OffPending : DevicePushState

    /** Registering or removing the subscription. */
    data object Busy : DevicePushState

    /** Subscribed at the server through the push relay. */
    data object On : DevicePushState

    /** Wanted, but it did not work; the app tries again at the next start. */
    data class Failed(val failure: PushFailure) : DevicePushState
}

/** Why the subscription of this device failed. */
enum class PushFailure {
    /** No answer from the server. */
    Offline,

    /** The push relay did not answer or refused (concept 27, 7.7: the list in the app stays). */
    Relay,

    /** Google's push token could not be read (no Play services, no network). */
    Google,

    /** The server refused the subscription. */
    Server,
}

/** The push subscription of this device, as the page «Einstellungen › Benachrichtigungen» uses it. */
interface DevicePush {
    val state: StateFlow<DevicePushState>

    /** Switches push on for this device and subscribes (the app asks for the system permission first). */
    suspend fun enable()

    /** Switches push off for this device and removes its subscription at the server. */
    suspend fun disable()

    /** Checks the subscription again (the page opens, the server's key changed). */
    suspend fun refresh()
}
