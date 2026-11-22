package com.slouchcat.app.mobile.tvlink

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.PlanarYUVLuminanceSource
import com.google.zxing.ReaderException
import com.google.zxing.common.HybridBinarizer
import com.google.zxing.qrcode.QRCodeReader

/**
 * The scanner of the QR code on the TV (concept 27 section 10 step 1; F-33): the build `play` uses
 * Google's code scanner (its own screen, no camera permission), the build `foss` its own camera page
 * (CameraX and ZXing, with the camera permission). Each flavour brings [flavorScanner]; tests and
 * the emulator flow pass their own through the feature's constructor. The app takes a code only from
 * here, never as a link (K-5): the manifest exports no deep link.
 */
interface CodeScanner {
    /** Whether this scanner asks for the camera permission (the hint under «Scanner öffnen»). */
    val asksForCamera: Boolean

    /**
     * Draws what the scanner shows inside the camera area of the page (the camera picture, or nothing
     * while Google's scanner covers the screen) and reports once: [onCode] with the text read,
     * [onClose] when the user closed the scanner, [onUnavailable] when there is no scanner (no camera,
     * no Google Play services).
     */
    @Composable
    fun Camera(onCode: (String) -> Unit, onClose: () -> Unit, onUnavailable: () -> Unit, modifier: Modifier)
}

/**
 * QR codes from a camera frame (ZXing core, Apache-2.0, compatible with the AGPL): the luminance of
 * the frame, row by row with [rowStride] bytes per row. `null` when there is no code in it.
 */
object QrDecoder {
    private val hints = mapOf(DecodeHintType.TRY_HARDER to true)

    fun decode(luminance: ByteArray, rowStride: Int, width: Int, height: Int): String? {
        if (width <= 0 || height <= 0 || rowStride < width || luminance.size < rowStride * (height - 1) + width) return null
        val rows = luminance.size / rowStride
        val data = if (rows >= height) luminance else luminance.copyOf(rowStride * height)
        val source = PlanarYUVLuminanceSource(data, rowStride, maxOf(rows, height), 0, 0, width, height, false)
        return try {
            QRCodeReader().decode(BinaryBitmap(HybridBinarizer(source)), hints).text
        } catch (e: ReaderException) {
            null
        }
    }
}
