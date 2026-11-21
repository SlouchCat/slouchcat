package com.slouchcat.core.session

import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** What the user typed as the server address, read (concept 27, 6). */
sealed interface ParsedAddress {
    data object Empty : ParsedAddress

    data object Invalid : ParsedAddress

    /** [url] without query and fragment; a path stays (a server behind a proxy under /media). */
    data class Valid(val url: HttpUrl) : ParsedAddress {
        val encrypted: Boolean get() = url.isHttps
    }
}

object ServerAddress {
    /**
     * Reads an address as the web app's users type it: without a scheme it is `https://`, a slash
     * at the end goes. Only http and https with a host; nothing with spaces, user info, query or
     * fragment.
     */
    fun parse(input: String): ParsedAddress {
        var text = input.trim()
        if (text.isEmpty()) return ParsedAddress.Empty
        if (text.any { it.isWhitespace() }) return ParsedAddress.Invalid
        if (!text.contains("://")) text = "https://$text"
        val scheme = text.substringBefore("://").lowercase()
        if (scheme != "http" && scheme != "https") return ParsedAddress.Invalid
        val url = text.toHttpUrlOrNull() ?: return ParsedAddress.Invalid
        if (url.host.isEmpty() || url.username.isNotEmpty() || url.password.isNotEmpty()) return ParsedAddress.Invalid
        if (url.query != null || url.fragment != null) return ParsedAddress.Invalid
        // `http//host` or `https:/host`: a broken scheme read as a host.
        if (url.host == "http" || url.host == "https") return ParsedAddress.Invalid
        // A host name without a dot stays allowed: in a home network `http://nas:8096` is common.
        val segments = url.pathSegments.filter { it.isNotEmpty() }
        val clean = url.newBuilder().encodedPath("/").apply { segments.forEach { addPathSegment(it) } }.build()
        return ParsedAddress.Valid(clean)
    }

    /** The address as the app shows and stores it: no slash at the end. */
    fun display(url: HttpUrl): String = url.toString().trimEnd('/')
}
