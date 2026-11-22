package com.slouchcat.app.mobile.downloads

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalConfiguration
import java.text.NumberFormat
import java.util.Locale

/**
 * A size in the largest fitting unit, as the web app writes it (`formatBytes` of web/src/lib/format.ts):
 * powers of 1000, one decimal from GB on, the number in the format of the app's language («4,2 GB»
 * in German, «4.2 GB» in English). The language without its region, as the web app formats with its
 * language tag: Swiss German writes the decimal comma there too.
 */
fun formatBytes(bytes: Long, locale: Locale): String {
    val units = listOf("B", "kB", "MB", "GB", "TB")
    var value = bytes.coerceAtLeast(0).toDouble()
    var unit = 0
    while (value >= 1000 && unit < units.lastIndex) {
        value /= 1000
        unit++
    }
    val number = NumberFormat.getNumberInstance(Locale.forLanguageTag(locale.language)).apply {
        maximumFractionDigits = if (unit >= 3) 1 else 0
        minimumFractionDigits = 0
    }
    return "${number.format(value)} ${units[unit]}"
}

/** [formatBytes] in the language of the app. */
@Composable
fun fileSize(bytes: Long): String = formatBytes(bytes, LocalConfiguration.current.locales[0])
