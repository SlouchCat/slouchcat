package com.slouchcat.app.tv.showcase

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.LinearGradient
import android.graphics.Paint
import android.graphics.Shader
import android.graphics.Typeface
import android.text.Layout
import android.text.StaticLayout
import android.text.TextPaint
import coil3.ImageLoader
import coil3.asImage
import coil3.intercept.Interceptor
import coil3.request.ImageResult
import coil3.request.SuccessResult
import kotlin.math.abs

/**
 * Pictures made in the test for the showcase (no files, nothing from the network, no real films):
 * a poster 2:3, a backdrop 16:9, a square cover, a portrait or a title logo, each a colour
 * gradient with the title written on it. The colour follows the id, so a title looks the same on
 * every page. [names] maps an id (a title, a person, a collection) to what is written.
 */
class ShowcaseImages(private val names: Map<String, String>) {
    /** An image loader that answers every request of the app from [draw]: no network. */
    fun loader(context: Context): ImageLoader = ImageLoader.Builder(context)
        .components {
            add(
                Interceptor { chain ->
                    val url = chain.request.data.toString()
                    SuccessResult(draw(url).asImage(), chain.request) as ImageResult
                },
            )
        }
        .build()

    /**
     * The picture behind an address of the server (`/api/v1/items/<id>/images/<kind>/<size>`,
     * `/people/<id>/image`; a title outside the library `/api/v1/titles/<media>/<tmdbId>/images/<kind>/<size>`,
     * named as `<media>-<tmdbId>`).
     */
    fun draw(url: String): Bitmap {
        val path = url.substringAfter("/api/v1/").substringBefore('?').split('/')
        val outside = path.firstOrNull() == "titles"
        val id = if (outside) "${path.getOrElse(1) { "" }}-${path.getOrElse(2) { "" }}" else path.getOrElse(1) { url }
        val kind = when {
            outside -> path.getOrElse(4) { "poster" }
            path.firstOrNull() == "people" -> "person"
            path.firstOrNull() == "collections" -> "poster"
            else -> path.getOrElse(3) { "poster" }
        }
        val name = names[id] ?: id
        val square = id in squareIds
        return when (kind) {
            "backdrop" -> picture(1280, 720, id, name, small = true)
            "logo" -> logo(name)
            else -> if (square) picture(600, 600, id, name, small = false) else picture(400, 600, id, name, small = false)
        }
    }

    /** Ids drawn square (audiobooks and podcasts). */
    var squareIds: Set<String> = emptySet()

    // Ids differ in their last digits only: spread them over the circle.
    private fun hue(id: String): Float = (abs(id.hashCode() * 47L) % 360).toFloat()

    private fun colour(hue: Float, saturation: Float, value: Float) = android.graphics.Color.HSVToColor(floatArrayOf(hue, saturation, value))

    private fun picture(width: Int, height: Int, id: String, name: String, small: Boolean): Bitmap {
        val bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        val h = hue(id)
        val paint = Paint(Paint.ANTI_ALIAS_FLAG)
        paint.shader = LinearGradient(0f, 0f, width.toFloat(), height.toFloat(), colour(h, 0.65f, 0.85f), colour((h + 60f) % 360f, 0.8f, 0.25f), Shader.TileMode.CLAMP)
        canvas.drawRect(0f, 0f, width.toFloat(), height.toFloat(), paint)
        // Some shapes, so a backdrop is no plain area under the scrims of the stage.
        paint.shader = null
        paint.color = colour((h + 180f) % 360f, 0.5f, 0.95f)
        paint.alpha = 70
        canvas.drawCircle(width * 0.72f, height * 0.38f, height * 0.32f, paint)
        paint.alpha = 50
        canvas.drawCircle(width * 0.2f, height * 0.8f, height * 0.2f, paint)
        if (!small) {
            val text = TextPaint(Paint.ANTI_ALIAS_FLAG).apply {
                color = android.graphics.Color.WHITE
                textSize = width / 9f
                typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
            }
            val layout = StaticLayout.Builder.obtain(name.uppercase(), 0, name.length, text, (width * 0.84f).toInt())
                .setAlignment(Layout.Alignment.ALIGN_CENTER)
                .build()
            canvas.save()
            canvas.translate(width * 0.08f, height * 0.72f - layout.height / 2f)
            layout.draw(canvas)
            canvas.restore()
        }
        return bitmap
    }

    private fun logo(name: String): Bitmap {
        val bitmap = Bitmap.createBitmap(800, 300, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        val text = TextPaint(Paint.ANTI_ALIAS_FLAG).apply {
            color = android.graphics.Color.WHITE
            textSize = 96f
            typeface = Typeface.create(Typeface.SERIF, Typeface.BOLD_ITALIC)
            setShadowLayer(12f, 0f, 4f, android.graphics.Color.BLACK)
        }
        val layout = StaticLayout.Builder.obtain(name.uppercase(), 0, name.length, text, 780)
            .setAlignment(Layout.Alignment.ALIGN_CENTER)
            .setMaxLines(2)
            .build()
        canvas.translate(10f, (300 - layout.height) / 2f)
        layout.draw(canvas)
        return bitmap
    }
}
