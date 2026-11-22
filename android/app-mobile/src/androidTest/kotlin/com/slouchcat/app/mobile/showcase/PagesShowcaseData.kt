package com.slouchcat.app.mobile.showcase

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Account
import com.slouchcat.core.api.generated.models.AppLogin
import com.slouchcat.core.api.generated.models.CastMember
import com.slouchcat.core.api.generated.models.Collection
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.EffectiveLimits
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.ImageRef
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibrarySummary
import com.slouchcat.core.api.generated.models.MediaFileInfo
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.NextUp
import com.slouchcat.core.api.generated.models.PlaybackLimits
import com.slouchcat.core.api.generated.models.PlaybackPrefs
import com.slouchcat.core.api.generated.models.Person
import com.slouchcat.core.api.generated.models.PersonCreditsInner
import com.slouchcat.core.api.generated.models.PersonRef
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.generated.models.ProfileRequest
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.ProfileUpdate
import com.slouchcat.core.api.generated.models.RatingStep
import com.slouchcat.core.api.generated.models.RatingSystem
import com.slouchcat.core.api.generated.models.SearchResult
import com.slouchcat.core.api.generated.models.SessionInfo
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.api.generated.models.Trailer
import com.slouchcat.core.api.generated.models.UserPickResult
import com.slouchcat.core.api.generated.models.WatchedEntry
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.data.CollectionsRepository
import com.slouchcat.core.media.data.DetailRepository
import com.slouchcat.core.media.data.ItemRepository
import com.slouchcat.core.media.data.MyListsRepository
import com.slouchcat.core.media.data.PersonRepository
import com.slouchcat.core.media.data.ProfileSettingsRepository
import com.slouchcat.core.media.data.SearchQuery
import com.slouchcat.core.media.data.SearchRepository
import com.slouchcat.core.player.PlaybackPrefsRepository
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.DevicesRepository
import okhttp3.HttpUrl
import java.time.LocalDate
import java.time.OffsetDateTime

/**
 * Made-up data of the pages after the start (stage 4 and 5) for the showcase: a film with cast,
 * collection and two versions, a series with two seasons, its episodes, a person, the search,
 * «Merkliste», «Gesehen», the collections, the profiles of the account and the device. Invented
 * names, drawn pictures, no server ([data] answers everything from memory).
 */
object PagesShowcaseData {
    private val b = BrowseShowcaseData
    private val film: ItemRef = b.films[0]
    private val sequel: ItemRef = b.films[1]
    private val show: ItemRef = b.series[0]
    private val at = OffsetDateTime.parse("2026-11-20T20:00:00Z")

    // People: ids of their own, written on their portraits.
    private val people = listOf(101L to "Ilka Morand", 102L to "Tomas Brenner", 103L to "Selin Aras", 104L to "Jaro Wendt")

    private fun portrait(id: Long) = ImageRef("/api/v1/people/$id/image?v=showcase&p=${SHOWCASE_PROFILE.id}", 300, 450)

    private val cast = listOf(
        CastMember(101, "Ilka Morand", CastMember.Role.CAST, character = "Wera Lind", image = portrait(101)),
        CastMember(102, "Tomas Brenner", CastMember.Role.CAST, character = "Der Schaffner", image = portrait(102)),
        CastMember(103, "Selin Aras", CastMember.Role.CAST, character = "Nora", image = portrait(103)),
        CastMember(104, "Jaro Wendt", CastMember.Role.DIRECTOR, image = portrait(104)),
    )

    private val lumenCollection = CollectionRef(
        "01JSHOWCOLLECTION0000000001", "Lumen-Reihe", 2, cover = film, members = listOf(film, sequel), firstYear = 2025, lastYear = 2026,
    )
    private val collections = listOf(
        lumenCollection,
        CollectionRef("01JSHOWCOLLECTION0000000002", "Hafengeschichten", 3, cover = b.films[7], members = listOf(b.films[7], b.films[4], b.films[6])),
        CollectionRef("01JSHOWCOLLECTION0000000003", "Nordlicht-Universum", 2, cover = show, members = listOf(show, b.series[1]), seenCount = 1, started = true),
    )

    private fun file(id: String, label: String, height: Int) = MediaFileInfo(
        id = id, container = "mkv", sizeBytes = 4_200_000_000, available = true, videoCodec = "hevc", audioCodec = "eac3",
        width = height * 16 / 9, height = height, versionLabel = label,
    )

    private fun detailImages(ref: ItemRef) = ref.images.copy(
        backdropLarge = b.image(ref.id, "backdrop", "w1920", 1920, 1080),
    )

    private val filmDetail = ItemDetail(
        id = film.id, kind = ItemKind.MOVIE, title = film.title, inWatchlist = true, images = detailImages(film),
        recommendedByMe = false, canRecommend = true, collections = listOf(lumenCollection), genres = listOf("Drama", "Abenteuer"),
        mediaFiles = listOf(file("f1", "Kinofassung", 2160), file("f2", "Langfassung", 1080)),
        year = 2026, rating = 12, runtimeMinutes = 145, tagline = "Jede Station ein neues Leben.",
        overview = "Wera Lind steigt in den letzten Nachtzug nach Lumen und merkt erst nach Mitternacht, dass jede Station " +
            "ein anderes Jahr ihres Lebens ist. Sie hat bis zum Morgen Zeit, den Halt zu finden, an dem alles anders kam, " +
            "und muss entscheiden, was sie dort zurücklässt.",
        progress = b.progress(35, 145), cast = cast, trailerCount = 1,
    )

    // A series with two seasons of four episodes; season 1 watched up to episode 2, episode 3 started.
    private val seasonIds = listOf(b.id(), b.id())
    private val episodeTitles = listOf(
        listOf("Das erste Licht", "Funkstille", "Unter dem Eis", "Die Karte"),
        listOf("Zurück in Kirkby", "Der Leuchtturm", "Polarnacht", "Was bleibt"),
    )
    private val episodes: List<List<ItemRef>> = episodeTitles.mapIndexed { s, titles ->
        titles.mapIndexed { e, title ->
            val id = b.id()
            val progress = when {
                s == 0 && e < 2 -> b.progress(100, 52).copy(completed = true, watchedAt = at)
                s == 0 && e == 2 -> b.progress(40, 52)
                else -> null
            }
            ItemRef(
                id, ItemKind.EPISODE, title, inWatchlist = false,
                images = ItemImages(backdropSmall = b.image(id, "backdrop", "w780", 780, 439), backdrop = b.image(id, "backdrop", "w1280", 1280, 720)),
                overview = "Die Akte ${s + 1}.${e + 1}: eine Spur führt weiter in den Norden.",
                runtimeMinutes = 52, progress = progress, parentId = seasonIds[s], seasonNumber = s + 1, episodeNumber = e + 1,
                seriesRef = com.slouchcat.core.api.generated.models.SeriesRef(show.id, show.title, show.images),
            )
        }
    }
    private val seasonRefs = seasonIds.mapIndexed { s, id ->
        ItemRef(id, ItemKind.SEASON, "Staffel ${s + 1}", inWatchlist = false, images = show.images, parentId = show.id, seasonNumber = s + 1, childCount = 4, watchedCount = if (s == 0) 2 else 0)
    }

    private val showDetail = ItemDetail(
        id = show.id, kind = ItemKind.SERIES, title = show.title, inWatchlist = false, images = detailImages(show),
        recommendedByMe = false, canRecommend = true, collections = listOf(collections[2]), genres = listOf("Krimi", "Thriller"),
        mediaFiles = emptyList(), year = 2026, rating = 12, watched = false, watchedCount = 2,
        overview = "In einem Dorf am Polarkreis verschwinden Funksignale. Eine Ermittlerin aus der Stadt folgt ihnen bis an den Rand der Karte.",
        seasons = seasonRefs, nextUp = NextUp(NextUp.Reason.RESUME, episodes[0][2]), cast = cast.take(3), trailerCount = 0,
    )

    private fun season(s: Int) = ItemDetail(
        id = seasonIds[s], kind = ItemKind.SEASON, title = seasonRefs[s].title, inWatchlist = false, images = show.images,
        recommendedByMe = false, canRecommend = false, collections = emptyList(), genres = emptyList(), mediaFiles = emptyList(),
        parentId = show.id, seasonNumber = s + 1, childCount = 4, watchedCount = seasonRefs[s].watchedCount, episodes = episodes[s],
        series = show, nextUp = if (s == 0) NextUp(NextUp.Reason.RESUME, episodes[0][2]) else NextUp(NextUp.Reason.START, episodes[1][0]),
    )

    private fun episode(s: Int, e: Int): ItemDetail {
        val ref = episodes[s][e]
        return ItemDetail(
            id = ref.id, kind = ItemKind.EPISODE, title = ref.title, inWatchlist = false, images = ref.images,
            recommendedByMe = false, canRecommend = false, collections = emptyList(), genres = emptyList(),
            mediaFiles = listOf(file("e-$s-$e", "", 1080).copy(versionLabel = null)), overview = ref.overview, runtimeMinutes = 52,
            progress = ref.progress, parentId = ref.parentId, seasonNumber = s + 1, episodeNumber = e + 1, seriesRef = ref.seriesRef,
            series = show, nextEpisode = episodes[s].getOrNull(e + 1) ?: episodes.getOrNull(s + 1)?.firstOrNull(),
        )
    }

    /** The ids the showcase opens. */
    val filmId: String = film.id
    val seriesId: String = show.id
    val seasonId: String = seasonIds[0]
    val episodeId: String = episodes[0][2].id
    const val PERSON_ID = 101L
    val collectionId: String = lumenCollection.id

    private val details: Map<String, ItemDetail> by lazy {
        buildMap {
            put(filmDetail.id, filmDetail)
            put(showDetail.id, showDetail)
            seasonIds.indices.forEach { put(seasonIds[it], season(it)) }
            episodes.forEachIndexed { s, list -> list.indices.forEach { e -> put(list[e].id, episode(s, e)) } }
        }
    }

    private val person = Person(
        id = PERSON_ID, name = "Ilka Morand", image = portrait(PERSON_ID), knownForDepartment = "Acting",
        birthday = LocalDate.of(1988, 3, 14), placeOfBirth = "Seewil",
        biography = "Ilka Morand wuchs an einem See auf und stand mit zwölf zum ersten Mal auf einer Bühne. Nach der Schauspielschule " +
            "spielte sie lange Theater, bevor sie mit «Kalter Mond» bekannt wurde. Sie lebt mit ihrer Familie in den Bergen.",
        credits = listOf(
            PersonCreditsInner(film, PersonCreditsInner.Role.CAST, character = "Wera Lind"),
            PersonCreditsInner(b.films[2], PersonCreditsInner.Role.CAST, character = "Ada"),
            PersonCreditsInner(show, PersonCreditsInner.Role.CAST, character = "Kommissarin Hale"),
            PersonCreditsInner(b.films[5], PersonCreditsInner.Role.CAST, character = "Die Fährfrau"),
            PersonCreditsInner(b.films[8], PersonCreditsInner.Role.CAST, character = "Lotte"),
        ),
    )

    private val watchlist: List<ItemRef> = listOf(film, b.series[1], b.films[4], b.books[0], b.films[8]).map { it.copy(inWatchlist = true) }
    private val watched: List<WatchedEntry> = listOf(
        WatchedEntry(b.films[3], at.minusDays(1)),
        WatchedEntry(b.series[2], at.minusDays(3), episodeCount = 8),
        WatchedEntry(b.films[6], at.minusDays(9)),
        WatchedEntry(b.books[1], at.minusDays(20)),
        WatchedEntry(b.films[7], at.minusDays(40)),
    )

    private val profiles = listOf(
        profile(1, "Mara", primary = true, rating = 18, accent = Accent.EMBER, hasPin = true),
        profile(2, "Jonas", rating = 16, accent = Accent.SKY, hasPin = true),
        profile(3, "Kinder", rating = 6, accent = Accent.LIME, allLibraries = false, libraryIds = listOf(1, 2)),
    )

    private fun profile(id: Long, name: String, primary: Boolean = false, rating: Int, accent: Accent, hasPin: Boolean = false, allLibraries: Boolean = true, libraryIds: List<Long> = emptyList()) = Profile(
        id = id, name = name, maxRating = rating, language = Language.DE, allLibraries = allLibraries, libraryIds = libraryIds,
        hasPin = hasPin, hasOfflinePin = false, isPrimary = primary, limits = PlaybackLimits(maxStreams = 2), effectiveLimits = EffectiveLimits(),
        audioLanguages = setOf("de"), subtitleMode = SubtitleMode.FORCED, subtitleLanguages = setOf("de"), accent = accent,
    )

    private val summaries = profiles.map { ProfileSummary(it.id, it.name, it.hasPin, it.isPrimary, accent = it.accent) }

    private val sessionInfo = SessionInfo(
        Account(1, "mara@example.test", Account.Role.USER, isOwner = true, maxRating = 18, allLibraries = true, libraryIds = emptyList()),
        ratingSystem = RatingSystem("CH", listOf(0, 6, 12, 16, 18).map { RatingStep(it, "ab $it") }, name = "Schweiz"),
    )

    /** What the pictures write on the ids of this file (portraits, episodes). */
    val names: Map<String, String> by lazy {
        people.associate { (id, name) -> id.toString() to name } +
            episodes.flatten().associate { it.id to it.title } + collections.associate { it.id to it.name }
    }

    private fun <T> page(list: List<T>, offset: Int, limit: Int) = ApiResult.Ok(Page(list.drop(offset).take(limit), list.size))

    private val detail = object : DetailRepository {
        override suspend fun item(server: HttpUrl, id: String) = details[id]?.let { ApiResult.Ok(it) } ?: ApiResult.NotFound
        override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean) = ApiResult.Ok(Unit)
        override suspend fun trailers(server: HttpUrl, id: String) =
            ApiResult.Ok(listOf(Trailer(1, "de", "abcDEF12345", java.net.URI("https://www.youtube-nocookie.com/embed/abcDEF12345"), Trailer.Source.TMDB, official = true)))
        override suspend fun setWatched(server: HttpUrl, id: String, on: Boolean) = ApiResult.Ok(Unit)
        override suspend fun recommend(server: HttpUrl, id: String): ApiResult<UserPickResult> = ApiResult.NotFound
        override suspend fun unrecommend(server: HttpUrl, id: String) = ApiResult.Ok(Unit)
    }

    private val search = object : SearchRepository {
        override suspend fun search(server: HttpUrl, query: SearchQuery, offset: Int, limit: Int): ApiResult<SearchResult> {
            val text = query.text.orEmpty().lowercase()
            val items = (b.all).filter { text.isNotEmpty() && text in it.title.lowercase() || (query.genre != null && it.kind == ItemKind.MOVIE) }
            val people = people.filter { text.isNotEmpty() && text in it.second.lowercase() }.map { (id, name) -> PersonRef(id, name, portrait(id)) }
            val found = collections.filter { text.isNotEmpty() && text in it.name.lowercase() }
            return ApiResult.Ok(SearchResult(items.drop(offset).take(limit), items.size, offset, limit, people, found))
        }
        override suspend fun genres(server: HttpUrl) = ApiResult.Ok(listOf(GenreRef("Drama", 5), GenreRef("Science Fiction", 4), GenreRef("Komödie", 4), GenreRef("Thriller", 3), GenreRef("Abenteuer", 3), GenreRef("Krimi", 2)))
    }

    private val myLists = object : MyListsRepository {
        override suspend fun watchlist(server: HttpUrl, offset: Int, limit: Int) = page(watchlist, offset, limit)
        override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean) = ApiResult.Ok(Unit)
        override suspend fun watched(server: HttpUrl, offset: Int, limit: Int) = page(watched, offset, limit)
        override suspend fun setWatched(server: HttpUrl, id: String, watched: Boolean) = ApiResult.Ok(Unit)
    }

    private val collectionsRepository = object : CollectionsRepository {
        override suspend fun collections(server: HttpUrl) = ApiResult.Ok(collections)
        override suspend fun collection(server: HttpUrl, id: String, offset: Int, limit: Int): ApiResult<Collection> {
            val ref = collections.firstOrNull { it.id == id } ?: return ApiResult.NotFound
            val items = ref.members.orEmpty()
            return ApiResult.Ok(
                Collection(
                    id = id, name = ref.name, items = items.drop(offset).take(limit), total = items.size, offset = offset, limit = limit,
                    overview = "Zwei Reisen mit dem Nachtzug, zwei Jahrzehnte auseinander.", firstYear = ref.firstYear, lastYear = ref.lastYear,
                    runtimeMinutes = 257, seenCount = 0, started = false, next = items.firstOrNull(), nextPart = 1,
                ),
            )
        }
        override suspend fun setWatched(server: HttpUrl, id: String, watched: Boolean) = ApiResult.Ok(Unit)
        override suspend fun setWatchlist(server: HttpUrl, itemId: String, on: Boolean) = ApiResult.Ok(Unit)
    }

    private val profilesRepository = object : ProfileSettingsRepository {
        override suspend fun profiles(server: HttpUrl) = ApiResult.Ok(profiles)
        override suspend fun summaries(server: HttpUrl) = ApiResult.Ok(summaries)
        override suspend fun libraries(server: HttpUrl) = ApiResult.Ok(b.libraries.map { LibrarySummary(it.id, it.name, it.kind) })
        override suspend fun session(server: HttpUrl) = ApiResult.Ok(sessionInfo)
        override suspend fun create(server: HttpUrl, request: ProfileRequest): ApiResult<Profile> = ApiResult.NotFound
        override suspend fun update(server: HttpUrl, id: Long, update: ProfileUpdate): ApiResult<Profile> = ApiResult.NotFound
        override suspend fun delete(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)
        override suspend fun setPin(server: HttpUrl, id: Long, pin: String) = ApiResult.Ok(Unit)
        override suspend fun removePin(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)
        override suspend fun setOfflinePin(server: HttpUrl, id: Long, pin: String) = ApiResult.Ok(Unit)
        override suspend fun removeOfflinePin(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)
    }

    /** Every repository of the pages over this data, the start page and libraries from [BrowseShowcaseData]. */
    fun data(base: MediaData): MediaData {
        val browse = b.data(base)
        val items: ItemRepository = object : ItemRepository by browse.items {
            override suspend fun item(server: HttpUrl, id: String) = details[id]?.let { ApiResult.Ok(it) } ?: browse.items.item(server, id)
        }
        return browse.copy(
            items = items,
            detail = detail,
            person = object : PersonRepository {
                override suspend fun person(server: HttpUrl, id: Long) = if (id == PERSON_ID) ApiResult.Ok(person) else ApiResult.NotFound
            },
            search = search,
            myLists = myLists,
            collections = collectionsRepository,
            profiles = profilesRepository,
        )
    }

    /** The account's side for the profile choice and «Mehr»: profiles, session, nothing that signs in. */
    val auth = object : AuthRepository {
        override suspend fun appLogin(server: HttpUrl, email: String, password: String, device: DeviceDescription, deviceId: String?): ApiResult<AppLogin> = ApiResult.Offline
        override suspend fun session(server: HttpUrl) = ApiResult.Ok(sessionInfo)
        override suspend fun profiles(server: HttpUrl) = ApiResult.Ok(summaries)
        override suspend fun selectProfile(server: HttpUrl, id: Long, pin: String?): ApiResult<SessionInfo> = ApiResult.Offline
        override suspend fun logout(server: HttpUrl) = ApiResult.Ok(Unit)
        override suspend fun updateAccount(server: HttpUrl, currentPassword: String, email: String?, newPassword: String?): ApiResult<SessionInfo> = ApiResult.Offline
    }

    /** «Meine Geräte»: this phone and a TV. */
    val devices = object : DevicesRepository {
        override suspend fun devices(server: HttpUrl) = ApiResult.Ok(
            listOf(
                MyDevice("showcase-device", at, 0, current = true, name = "Pixel · Android 16", type = MyDevice.Type.PHONE),
                MyDevice("tv-1", at.minusDays(2), 0, current = false, name = "Wohnzimmer-TV", type = MyDevice.Type.TV),
            ),
        )
        override suspend fun signOut(server: HttpUrl, deviceId: String) = ApiResult.Ok(Unit)
    }

    /** The profile's languages for «Einstellungen › Wiedergabe». */
    val playbackPrefs = object : PlaybackPrefsRepository {
        private var prefs = PlaybackPrefs(audioLanguages = setOf("de", "en"), subtitleMode = SubtitleMode.FORCED, subtitleLanguages = setOf("de"))
        override suspend fun prefs() = ApiResult.Ok(prefs)
        override suspend fun save(prefs: PlaybackPrefs): ApiResult<PlaybackPrefs> {
            this.prefs = prefs
            return ApiResult.Ok(prefs)
        }
        override suspend fun resetSeries() = ApiResult.Ok(Unit)
    }
}
