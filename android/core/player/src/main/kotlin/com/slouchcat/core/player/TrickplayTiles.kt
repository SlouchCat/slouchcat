package com.slouchcat.core.player

import com.slouchcat.core.api.generated.models.Trickplay

/** One preview tile: the sheet to load (absolute address) and the tile's rectangle in it. */
data class TrickplayTile(val sheetUrl: String, val x: Int, val y: Int, val width: Int, val height: Int, val timeMs: Long) {
    /** Without the address: a sheet's address may carry a token (rule 10). */
    override fun toString() = "TrickplayTile(x=$x, y=$y, width=$width, height=$height, timeMs=$timeMs)"
}

/**
 * The preview tiles of the seek bar (concept 18 part A, `getTrickplay`): tile `k` shows
 * `pointsMs[k]` and lies on sheet `k / (columns * rows)` in reading order. The app loads a sheet
 * through its image loader (the one HTTP client) and draws the rectangle.
 */
class TrickplayTiles(private val set: Trickplay, private val absolute: (String) -> String?) {
    val tileWidth: Int get() = set.tileWidth
    val tileHeight: Int get() = set.tileHeight

    /** The tile nearest before [positionMs]; `null` before the first point or while its sheet does not exist. */
    fun at(positionMs: Long): TrickplayTile? {
        val points = set.pointsMs
        if (points.isEmpty()) return null
        var k = points.binarySearch(positionMs)
        if (k < 0) k = -k - 2
        if (k < 0) k = 0
        val perSheet = set.columns * set.rows
        if (perSheet <= 0) return null
        val sheet = set.sheets.firstOrNull { it.index == k / perSheet } ?: return null
        val url = absolute(sheet.url) ?: return null
        val inSheet = k % perSheet
        return TrickplayTile(
            sheetUrl = url,
            x = (inSheet % set.columns) * set.tileWidth,
            y = (inSheet / set.columns) * set.tileHeight,
            width = set.tileWidth,
            height = set.tileHeight,
            timeMs = points[k],
        )
    }
}
