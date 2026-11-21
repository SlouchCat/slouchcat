package com.slouchcat.core.api.log

/** Where log lines end up; the app installs one that writes to logcat. */
interface LogSink {
    /** [priority] as `android.util.Log` counts it (2 verbose … 6 error). */
    fun write(priority: Int, tag: String, message: String)
}

/**
 * The one logger of the apps (rule 10, concept 27 5.6). Token, voucher, device id and stream tokens
 * never reach a log line: values of fields with such names are replaced, and the message and the
 * text of an error are scrubbed of the forms those values travel in (Bearer header, `dt`/`st`/`rt`
 * in an address, the device header). The lint rule `SlouchcatDirectLog` reports every other way to
 * logcat.
 *
 * ```
 * Log.i("setup", "server found", "version" to info.version)
 * Log.w("setup", "probe failed", error = e)
 * ```
 * Put values into fields, not into the message: a field is redacted by its name.
 */
object Log {
    const val VERBOSE = 2
    const val DEBUG = 3
    const val INFO = 4
    const val WARN = 5
    const val ERROR = 6

    @Volatile
    var sink: LogSink = object : LogSink {
        override fun write(priority: Int, tag: String, message: String) = Unit
    }

    fun d(tag: String, message: String, vararg fields: Pair<String, Any?>, error: Throwable? = null) =
        write(DEBUG, tag, message, fields, error)

    fun i(tag: String, message: String, vararg fields: Pair<String, Any?>, error: Throwable? = null) =
        write(INFO, tag, message, fields, error)

    fun w(tag: String, message: String, vararg fields: Pair<String, Any?>, error: Throwable? = null) =
        write(WARN, tag, message, fields, error)

    fun e(tag: String, message: String, vararg fields: Pair<String, Any?>, error: Throwable? = null) =
        write(ERROR, tag, message, fields, error)

    private fun write(priority: Int, tag: String, message: String, fields: Array<out Pair<String, Any?>>, error: Throwable?) {
        sink.write(priority, tag, Redaction.line(message, fields.toList(), error))
    }
}

/** The rules of [Log], separate so they can be tested on their own. */
object Redaction {
    const val REDACTED = "[redacted]"

    /**
     * Field names whose values never appear (compared without case, `-` and `_`). The same list as
     * the server's redaction handler for the values an app holds (CLAUDE.md rule 10).
     */
    val SECRET_KEYS = setOf(
        "token", "bearer", "bearertoken", "authorization", "cookie", "setcookie", "password",
        "newpassword", "pin", "voucher", "pairingvoucher", "deviceid", "device", "xslouchcatdevice",
        "pickup", "pickupsecret", "sealedtoken", "fcmtoken", "googletoken", "secret", "dt", "st", "rt",
    )

    private val patterns = listOf(
        Regex("""(?i)(Bearer\s+)[^\s,;"']+""") to "$1$REDACTED",
        Regex("""(?i)(Pickup\s+)[^\s,;"']+""") to "$1$REDACTED",
        Regex("""(?i)(X-Slouchcat-Device\s*[:=]\s*)[^\s,;"']+""") to "$1$REDACTED",
        Regex("""(?i)([?&](?:dt|st|rt|token|voucher)=)[^&\s#"']+""") to "$1$REDACTED",
    )

    fun isSecret(key: String): Boolean = key.lowercase().replace("-", "").replace("_", "") in SECRET_KEYS

    fun scrub(text: String): String = patterns.fold(text) { acc, (regex, replacement) -> regex.replace(acc, replacement) }

    fun line(message: String, fields: List<Pair<String, Any?>>, error: Throwable?): String = buildString {
        append(scrub(message))
        for ((key, value) in fields) {
            append(' ').append(key).append('=')
            append(if (isSecret(key)) REDACTED else scrub(value.toString()))
        }
        // The class and the scrubbed message of the error, no stack trace: the messages of OkHttp
        // and of the generated client carry addresses, and a stack adds nothing a user could act on.
        var cause = error
        var depth = 0
        while (cause != null && depth < 4) {
            append(if (depth == 0) " error=" else " cause=")
            append(cause.javaClass.name)
            cause.message?.let { append(": ").append(scrub(it)) }
            cause = cause.cause
            depth++
        }
    }
}
