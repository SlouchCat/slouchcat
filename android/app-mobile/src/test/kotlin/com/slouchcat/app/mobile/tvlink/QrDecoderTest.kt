package com.slouchcat.app.mobile.tvlink

import com.google.zxing.BarcodeFormat
import com.google.zxing.qrcode.QRCodeWriter
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.pairing.PairingCode
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/** The QR decoding of the own scanner (build `foss`) on frames as the camera delivers them; and the relay address. */
class QrDecoderTest {
    private val code = PairingCode("https://pair.slouchcat.com".toHttpUrl(), Base64Url.encode(ByteArray(16) { 4 }), Base64Url.encode(ByteArray(32) { 6 }))

    /** A camera frame: the code in the middle, grey around it, [stride] bytes per row (a padded Y plane). */
    private fun frame(text: String, width: Int, height: Int, stride: Int, truncateLastRow: Boolean = false): ByteArray {
        val size = minOf(width, height) * 2 / 3
        val matrix = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, size, size)
        val left = (width - size) / 2
        val top = (height - size) / 2
        val bytes = ByteArray(stride * height) { 0x90.toByte() }
        for (y in 0 until size) for (x in 0 until size) {
            bytes[(top + y) * stride + left + x] = if (matrix[x, y]) 0x10 else 0xF0.toByte()
        }
        // The Y plane of a camera often ends after the last pixel, not after the last row's padding.
        return if (truncateLastRow) bytes.copyOf(stride * (height - 1) + width) else bytes
    }

    @Test
    fun theCodeOfTheTvReadsFromAFrame() {
        assertEquals(code.text, QrDecoder.decode(frame(code.text, 640, 480, 640), 640, 640, 480))
    }

    @Test
    fun aPaddedPlaneWithAShortLastRowReadsAsWell() {
        assertEquals(code.text, QrDecoder.decode(frame(code.text, 640, 480, 704, truncateLastRow = true), 704, 640, 480))
    }

    @Test
    fun aFrameWithoutCodeOrOfTheWrongSizeIsNothing() {
        assertNull(QrDecoder.decode(ByteArray(640 * 480) { 0x80.toByte() }, 640, 640, 480))
        assertNull(QrDecoder.decode(ByteArray(10), 640, 640, 480))
        assertNull(QrDecoder.decode(ByteArray(640 * 480), 320, 640, 480))
        assertNull(QrDecoder.decode(ByteArray(0), 0, 0, 0))
    }

    @Test
    fun aRelayIsHttpsUnlessTheBuildAllowsPlainHttp() {
        assertEquals("https://pair.example.test/".toHttpUrl(), parseRelay("pair.example.test", allowPlainHttp = false))
        assertEquals("https://pair.example.test/sub".toHttpUrl(), parseRelay("https://pair.example.test/sub/", allowPlainHttp = false))
        assertNull(parseRelay("http://pair.example.test", allowPlainHttp = false))
        assertEquals("http://10.0.2.2:9780/".toHttpUrl(), parseRelay("http://10.0.2.2:9780", allowPlainHttp = true))
        assertNull(parseRelay("", allowPlainHttp = true))
        assertNull(parseRelay("ftp://pair.example.test", allowPlainHttp = true))
        assertNull(parseRelay("https://pair.example.test/tv#v=1", allowPlainHttp = true))
    }
}
