package com.slouchcat.app.mobile.tvlink

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.gutter
import com.slouchcat.core.strings.R
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

/** The build `foss`: the app's own scanner, without Google's libraries (F-33). */
fun flavorScanner(): CodeScanner = CameraCodeScanner

/**
 * The camera of the app (CameraX) and ZXing for the QR code. The camera permission is asked when
 * the scanner opens, not before; a refusal shows why and the way to the app's settings.
 */
private object CameraCodeScanner : CodeScanner {
    override val asksForCamera: Boolean = true

    @Composable
    override fun Camera(onCode: (String) -> Unit, onClose: () -> Unit, onUnavailable: () -> Unit, modifier: Modifier) {
        val context = LocalContext.current
        var granted by remember { mutableStateOf(hasCamera(context)) }
        var refused by remember { mutableStateOf(false) }
        val ask = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { ok ->
            granted = ok
            refused = !ok
        }
        LaunchedEffect(Unit) { if (!granted) ask.launch(Manifest.permission.CAMERA) }
        // Back from the app's settings: the permission may be there now.
        LifecycleResumeEffect(Unit) {
            if (!granted && hasCamera(context)) {
                granted = true
                refused = false
            }
            onPauseOrDispose {}
        }
        when {
            granted -> CameraPicture(onCode, onUnavailable, modifier)
            refused -> Refused(context, modifier)
            else -> Box(modifier)
        }
    }

    private fun hasCamera(context: Context) =
        ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED

    @Composable
    private fun Refused(context: Context, modifier: Modifier) {
        Box(modifier, contentAlignment = Alignment.Center) {
            Column(Modifier.fillMaxWidth().padding(horizontal = gutter()), verticalArrangement = Arrangement.spacedBy(12.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Note(stringResource(R.string.app_tv_scan_denied), warning = true, icon = SlouchcatIcon.Lock)
                SlouchcatButton(
                    stringResource(R.string.app_tv_scan_settings),
                    onClick = {
                        val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.fromParts("package", context.packageName, null))
                            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                        context.startActivity(intent)
                    },
                    kind = ButtonKind.Secondary,
                    leadingIcon = SlouchcatIcon.Settings,
                )
            }
        }
    }

    /** The picture of the back camera and the analysis of every frame until the first QR code. */
    @Composable
    private fun CameraPicture(onCode: (String) -> Unit, onUnavailable: () -> Unit, modifier: Modifier) {
        val context = LocalContext.current
        val owner = LocalLifecycleOwner.current
        val code by rememberUpdatedState(onCode)
        val unavailable by rememberUpdatedState(onUnavailable)
        val preview = remember { PreviewView(context) }
        DisposableEffect(owner) {
            val main = ContextCompat.getMainExecutor(context)
            val analyzer = Executors.newSingleThreadExecutor()
            val found = AtomicBoolean(false)
            val future = ProcessCameraProvider.getInstance(context)
            future.addListener({
                val provider = try {
                    future.get()
                } catch (e: ExecutionException) {
                    Log.w(TAG, "no camera provider", error = e)
                    unavailable()
                    return@addListener
                }
                val picture = Preview.Builder().build().also { it.setSurfaceProvider(preview.surfaceProvider) }
                val analysis = ImageAnalysis.Builder().setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST).build()
                analysis.setAnalyzer(analyzer) { image ->
                    val text = image.use { read(it) }
                    if (text != null && found.compareAndSet(false, true)) main.execute { code(text) }
                }
                try {
                    provider.unbindAll()
                    provider.bindToLifecycle(owner, CameraSelector.DEFAULT_BACK_CAMERA, picture, analysis)
                } catch (e: IllegalArgumentException) {
                    // No back camera on this device.
                    Log.w(TAG, "no back camera", error = e)
                    unavailable()
                } catch (e: IllegalStateException) {
                    Log.w(TAG, "camera not bound", error = e)
                    unavailable()
                }
            }, main)
            onDispose {
                if (future.isDone) runCatching { future.get().unbindAll() }
                analyzer.shutdown()
            }
        }
        AndroidView(factory = { preview }, modifier = modifier)
    }

    /** The luminance (Y plane) of a frame through ZXing. */
    private fun read(image: ImageProxy): String? {
        val plane = image.planes.firstOrNull() ?: return null
        val buffer = plane.buffer.duplicate().apply { rewind() }
        val bytes = ByteArray(buffer.remaining()).also { buffer.get(it) }
        return QrDecoder.decode(bytes, plane.rowStride, image.width, image.height)
    }

    private const val TAG = "scanner"
}
