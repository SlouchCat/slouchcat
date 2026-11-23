package com.slouchcat.app.mobile.tvlink

import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning
import com.slouchcat.core.api.log.Log

/** The build `play`: Google's code scanner (concept 27 5.2). */
fun flavorScanner(): CodeScanner = GoogleCodeScanner

/**
 * Google's code scanner of the Play services: its own screen above the app, QR codes only, no
 * camera permission for the app (it never sees the camera). Behind it the page shows the empty
 * camera area; the result comes back once.
 */
private object GoogleCodeScanner : CodeScanner {
    override val asksForCamera: Boolean = false

    @Composable
    override fun Camera(onCode: (String) -> Unit, onClose: () -> Unit, onUnavailable: () -> Unit, modifier: Modifier) {
        val context = LocalContext.current
        val code by rememberUpdatedState(onCode)
        val close by rememberUpdatedState(onClose)
        val unavailable by rememberUpdatedState(onUnavailable)
        LaunchedEffect(Unit) {
            val options = GmsBarcodeScannerOptions.Builder().setBarcodeFormats(Barcode.FORMAT_QR_CODE).build()
            GmsBarcodeScanning.getClient(context, options).startScan()
                .addOnSuccessListener { barcode -> code(barcode.rawValue.orEmpty()) }
                .addOnCanceledListener { close() }
                .addOnFailureListener { e ->
                    // No Play services, or the scanner module could not be loaded.
                    Log.w(TAG, "code scanner unavailable", error = e)
                    unavailable()
                }
        }
        Box(modifier)
    }

    private const val TAG = "scanner"
}
