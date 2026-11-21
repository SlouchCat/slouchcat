package com.slouchcat.core.media

import com.slouchcat.core.api.generated.models.ImageRef
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.SeriesRef
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** The addresses of the server's images and the card model made from a list entry. */
class ServerImagesTest {
    private val images = ServerImages("https://media.example.test/sub/".toHttpUrl())

    @Test
    fun aPathBelowTheApiLandsUnderTheServerAddressWithItsSubPath() {
        val url = images.url(ImageRef("/api/v1/items/A/images/poster/w400?v=h1&p=7", 400, 600))
        assertEquals("https://media.example.test/sub/api/v1/items/A/images/poster/w400?v=h1&p=7", url)
    }

    @Test
    fun aRelativePathCountsBelowTheApi() {
        assertEquals("https://media.example.test/sub/api/v1/people/3/image?p=1", images.url(ImageRef("people/3/image?p=1", 185, 278)))
    }

    @Test
    fun anAddressOfAnotherHostIsNotLoaded() {
        // The bearer goes only to the own server; an image elsewhere is not the server's.
        assertNull(images.url(ImageRef("https://elsewhere.example.test/x.webp", 10, 10)))
        assertEquals("https://media.example.test/a.webp", images.url(ImageRef("https://media.example.test/a.webp", 10, 10)))
    }

    @Test
    fun theProfileStaysInTheAddressSoEachProfileHasItsOwnCacheEntry() {
        val one = images.posters(posters("A", profile = 1)).pick(400)
        val two = images.posters(posters("A", profile = 2)).pick(400)
        assertTrue(one!!.endsWith("p=1"))
        assertNotEquals(one, two)
    }

    @Test
    fun theSizeFitsTheBox() {
        val set = images.posters(posters("A"))
        assertTrue(set.pick(150)!!.contains("/w200"))
        assertTrue(set.pick(300)!!.contains("/w400"))
        assertTrue(set.pick(500)!!.contains("/w800"))
        // Larger than the largest: the largest.
        assertTrue(set.pick(2000)!!.contains("/w800"))
        assertNull(ImageSet.NONE.pick(100))
    }

    @Test
    fun anEpisodeShowsItsSeriesAndAStartedTitleItsProgress() {
        val series = SeriesRef("S", "The Series", posters("S"))
        val episode = itemRef(1, title = "Pilot", kind = ItemKind.EPISODE, progress = progress(40, minutes = 50)).copy(seriesRef = series, seasonNumber = 1, episodeNumber = 2)
        val card = MediaItem.from(episode, images)
        assertEquals("The Series", card.seriesTitle)
        assertTrue(card.poster.pick(200)!!.contains("items/S/"))
        assertEquals(0.4f, card.progress!!, 0.001f)
        assertEquals(30, card.remainingMinutes)
        assertFalse(card.watched)
    }

    @Test
    fun audioIsSquareAndAFinishedFilmIsWatched() {
        assertEquals(CardShape.Square, MediaItem.from(itemRef(2, kind = ItemKind.AUDIOBOOK), images).defaultShape)
        assertEquals(CardShape.Poster, MediaItem.from(itemRef(3), images).defaultShape)
        assertTrue(MediaItem.from(itemRef(4, progress = progress(95, watched = true)), images).watched)
        // A next episode at position 0 counts as new, without a bar.
        assertNull(MediaItem.from(itemRef(5, progress = progress(0)), images).progress)
    }

    @Test
    fun aMissingImageIsAnEmptySet() {
        assertTrue(images.posters(null).isEmpty)
        assertTrue(images.single(image("people/1/image", 185)).sources.size == 1)
    }
}
