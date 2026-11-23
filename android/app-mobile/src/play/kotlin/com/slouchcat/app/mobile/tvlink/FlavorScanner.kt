package com.slouchcat.app.mobile.tvlink

import android.content.Context
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import com.google.android.gms.common.ConnectionResult
import com.google.android.gms.common.GoogleApiAvailability
import com.google.android.gms.common.moduleinstall.ModuleInstall
import com.google.android.gms.common.moduleinstall.ModuleInstallRequest
import com.google.mlkit.common.MlKitException
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScanner
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning
import com.slouchcat.core.api.log.Log

/** The build `play`: Google's code scanner (concept 27 5.2). */
fun flavorScanner(): CodeScanner = GoogleCodeScanner

/**
 * Google's code scanner of the Play services: its own screen above the app, QR codes only, no
 * camera permission for the app (it never sees the camera). Behind it the page shows the empty
 * camera area; the result comes back once.
 *
 * The scanner is a module of the Play services that is downloaded on demand (the manifest of this
 * flavour asks for it at the install, `com.google.mlkit.vision.DEPENDENCIES`). When it is not there
 * yet, the page says «Scanner wird geladen» and the scan starts once it is (code review M-2).
 */
private object GoogleCodeScanner : CodeScanner {
    override val asksForCamera: Boolean = false

    @Composable
    override fun Camera(
        accepts: (String) -> Boolean,
        onCode: (String) -> Unit,
        onClose: () -> Unit,
        onLoading: (Boolean) -> Unit,
        onTrouble: (ScannerTrouble) -> Unit,
        modifier: Modifier,
    ) {
        val context = LocalContext.current
        val code by rememberUpdatedState(onCode)
        val close by rememberUpdatedState(onClose)
        val loading by rememberUpdatedState(onLoading)
        val trouble by rememberUpdatedState(onTrouble)
        LaunchedEffect(Unit) {
            if (GoogleApiAvailability.getInstance().isGooglePlayServicesAvailable(context) != ConnectionResult.SUCCESS) {
                trouble(ScannerTrouble.NoPlayServices)
                return@LaunchedEffect
            }
            val options = GmsBarcodeScannerOptions.Builder().setBarcodeFormats(Barcode.FORMAT_QR_CODE).build()
            val scanner = GmsBarcodeScanning.getClient(context, options)
            val start = { scan(context, scanner, { code(it) }, { close() }, { trouble(it) }) }
            val modules = ModuleInstall.getClient(context)
            modules.areModulesAvailable(scanner)
                .addOnSuccessListener { available ->
                    if (available.areModulesAvailable()) {
                        start()
                    } else {
                        loading(true)
                        modules.installModules(ModuleInstallRequest.newBuilder().addApi(scanner).build())
                            .addOnSuccessListener {
                                loading(false)
                                start()
                            }
                            .addOnFailureListener { e ->
                                Log.w(TAG, "scanner module not installed", error = e)
                                trouble(ScannerTrouble.Loading)
                            }
                    }
                }
                .addOnFailureListener { e -> trouble(troubleOf(context, e)) }
        }
        Box(modifier)
    }

    private fun scan(context: Context, scanner: GmsBarcodeScanner, onCode: (String) -> Unit, onClose: () -> Unit, onTrouble: (ScannerTrouble) -> Unit) {
        scanner.startScan()
            .addOnSuccessListener { barcode -> onCode(barcode.rawValue.orEmpty()) }
            .addOnCanceledListener { onClose() }
            .addOnFailureListener { e -> onTrouble(troubleOf(context, e)) }
    }

    /** Only a missing Play service is «no Play services»; a module still loading is «loading». */
    private fun troubleOf(context: Context, e: Exception): ScannerTrouble {
        Log.w(TAG, "code scanner failed", error = e)
        return when {
            GoogleApiAvailability.getInstance().isGooglePlayServicesAvailable(context) != ConnectionResult.SUCCESS -> ScannerTrouble.NoPlayServices
            e is MlKitException && e.errorCode == MlKitException.UNAVAILABLE -> ScannerTrouble.Loading
            else -> ScannerTrouble.Failed
        }
    }

    private const val TAG = "scanner"
}
