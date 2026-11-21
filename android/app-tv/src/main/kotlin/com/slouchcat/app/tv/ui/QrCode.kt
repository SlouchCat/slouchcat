package com.slouchcat.app.tv.ui

import androidx.compose.foundation.Canvas
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.semantics.SemanticsPropertyKey
import androidx.compose.ui.semantics.SemanticsPropertyReceiver
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.common.BitMatrix
import com.google.zxing.qrcode.QRCodeWriter
import com.google.zxing.qrcode.decoder.ErrorCorrectionLevel
import com.slouchcat.core.design.SlouchcatTheme

/**
 * The text a QR code carries, in the semantics of its node: the emulator flow reads it (the code is
 * on the screen anyway), a screen reader does not (it reads [contentDescription]).
 */
val QrContent = SemanticsPropertyKey<String>("QrContent")
var SemanticsPropertyReceiver.qrContent by QrContent

/** The modules of a QR code for [text]: error correction M, no quiet zone (the box draws it). */
fun qrModules(text: String): BitMatrix = QRCodeWriter().encode(
    text,
    BarcodeFormat.QR_CODE,
    0,
    0,
    mapOf(EncodeHintType.ERROR_CORRECTION to ErrorCorrectionLevel.M, EncodeHintType.MARGIN to 0),
)

/**
 * A QR code drawn module by module (ZXing core only encodes; no bitmap, no Android parts): dark
 * modules in `--bg` on a ground of `--tx` (`--qr-fg`, `--qr-bg` of the mockup).
 */
@Composable
fun QrCode(text: String, description: String, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val matrix = remember(text) { qrModules(text) }
    Canvas(
        modifier.semantics {
            contentDescription = description
            qrContent = text
        },
    ) {
        drawRect(c.tx)
        val cell = minOf(size.width / matrix.width, size.height / matrix.height)
        val left = (size.width - cell * matrix.width) / 2
        val top = (size.height - cell * matrix.height) / 2
        for (y in 0 until matrix.height) {
            for (x in 0 until matrix.width) {
                // A hair larger than a module, so neighbours touch without gaps from rounding.
                if (matrix[x, y]) drawRect(c.bg, Offset(left + x * cell, top + y * cell), Size(cell + OVERLAP, cell + OVERLAP))
            }
        }
    }
}

private const val OVERLAP = 0.5f
