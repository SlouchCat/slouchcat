package com.slouchcat.core.design

import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.addPathNodes
import androidx.compose.ui.unit.dp

/**
 * The icons of the mockup (docs/planning/mockups/27-android-handy/js/core.js, `ICONS`): 24 x 24,
 * stroke 2, round caps and joins. Each entry keeps the SVG of the mockup; [vector] turns it into an
 * ImageVector once. Draw one with [SlouchcatIconImage]; the tint comes from the caller.
 */
@Suppress("SpellCheckingInspection")
enum class SlouchcatIcon(private val svg: String) {
    Home("""<path d="M3 10.5 12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z"/>"""),
    Search("""<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>"""),
    Download("""<path d="M12 3v12m0 0-4.5-4.5M12 15l4.5-4.5M4 20h16"/>"""),
    Wish("""<path d="M12 3l2.6 5.4 5.9.8-4.3 4.1 1 5.9L12 16.4 6.8 19.2l1-5.9L3.5 9.2l5.9-.8z"/>"""),
    More("""<path d="M4 6h16M4 12h16M4 18h16"/>"""),
    Play("""<path d="M7 4.5v15l12-7.5z" fill="currentColor"/>"""),
    Pause("""<path d="M7 5v14M17 5v14" stroke-width="3"/>"""),
    Back("""<path d="M19 12H5m0 0 6-6m-6 6 6 6"/>"""),
    Close("""<path d="M6 6l12 12M18 6 6 18"/>"""),
    Check("""<path d="m5 12.5 4.5 4.5L19 7.5"/>"""),
    Plus("""<path d="M12 5v14M5 12h14"/>"""),
    Bookmark("""<path d="M6 4h12v17l-6-4.5L6 21z"/>"""),
    BookmarkOn("""<path d="M6 4h12v17l-6-4.5L6 21z" fill="currentColor"/>"""),
    Right("""<path d="m9 5 7 7-7 7"/>"""),
    Down("""<path d="m5 9 7 7 7-7"/>"""),
    Up("""<path d="m5 15 7-7 7 7"/>"""),
    Settings("""<circle cx="12" cy="12" r="3"/><path d="M12 2v3m0 14v3M2 12h3m14 0h3M4.9 4.9l2.1 2.1m10 10 2.1 2.1M19.1 4.9 17 7M7 17l-2.1 2.1"/>"""),
    User("""<circle cx="12" cy="8" r="4"/><path d="M4 21c1.2-4 4.3-6 8-6s6.800 2 8 6"/>"""),
    Users("""<circle cx="9" cy="8" r="3.5"/><path d="M2.5 20c.9-3.300 3.400-5 6.500-5s5.600 1.700 6.500 5M16 4.800a3.500 3.500 0 0 1 0 6.400M18 15.300c1.800.700 3 2.300 3.500 4.700"/>"""),
    Tv("""<rect x="2.500" y="4" width="19" height="13" rx="2"/><path d="M8 21h8"/>"""),
    Qr("""<path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h2v2h-2zM18 14h2M14 18h2m2 0h2v2"/>"""),
    Radio("""<rect x="3" y="9" width="18" height="11" rx="2"/><path d="m7 9 10-5"/><circle cx="8.500" cy="14.500" r="2.500"/><path d="M15 13h3m-3 3h3"/>"""),
    Headphones("""<path d="M4 15v-3a8 8 0 0 1 16 0v3"/><rect x="3" y="14" width="4" height="6" rx="1.500"/><rect x="17" y="14" width="4" height="6" rx="1.500"/>"""),
    Mic("""<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/>"""),
    Bell("""<path d="M6 16V11a6 6 0 0 1 12 0v5l2 2H4zM10 20a2 2 0 0 0 4 0"/>"""),
    Filter("""<path d="M4 6h16M7 12h10M10 18h4"/>"""),
    Sort("""<path d="M7 5v14m0 0-3-3m3 3 3-3M17 19V5m0 0-3 3m3-3 3 3"/>"""),
    Star("""<path d="M12 3l2.600 5.400 5.900.800-4.300 4.100 1 5.900L12 16.400 6.800 19.200l1-5.900L3.500 9.200l5.900-.800z" fill="currentColor"/>"""),
    Info("""<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-9.500v.500"/>"""),
    Lock("""<rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>"""),
    Trash("""<path d="M4 7h16M9 7V4h6v3m-8 0 1 13h8l1-13"/>"""),
    Wifi("""<path d="M2.500 9a14 14 0 0 1 19 0M6 12.500a9 9 0 0 1 12 0M9.500 16a4 4 0 0 1 5 0"/><circle cx="12" cy="19.500" r="1" fill="currentColor"/>"""),
    Subs("""<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M7 14h4m3 0h3M7 10.500h2m3 0h5"/>"""),
    Audio("""<path d="M4 9v6h4l5 4V5L8 9zM16.500 8.500a5 5 0 0 1 0 7M19 6a9 9 0 0 1 0 12"/>"""),
    Fwd("""<path d="M12 5a7 7 0 1 1-6.600 4.700M12 5l-3-3m3 3-3 3"/>"""),
    Rew("""<path d="M12 5a7 7 0 1 0 6.600 4.700M12 5l3-3m-3 3 3 3"/>"""),
    Next("""<path d="M6 5v14l10-7z" fill="currentColor"/><path d="M19 5v14"/>"""),
    Prev("""<path d="M18 5v14L8 12z" fill="currentColor"/><path d="M5 5v14"/>"""),
    List("""<path d="M8 6h12M8 12h12M8 18h12M4 6h.010M4 12h.010M4 18h.010"/>"""),
    Grid("""<rect x="4" y="4" width="7" height="7" rx="1"/><rect x="13" y="4" width="7" height="7" rx="1"/><rect x="4" y="13" width="7" height="7" rx="1"/><rect x="13" y="13" width="7" height="7" rx="1"/>"""),
    Film("""<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M8 4v16M16 4v16M3 9h5m8 0h5M3 15h5m8 0h5"/>"""),
    Book("""<path d="M5 4h11a3 3 0 0 1 3 3v13H8a3 3 0 0 1-3-3zM5 17a3 3 0 0 1 3-3h11"/>"""),
    Eye("""<path d="M2 12s3.600-7 10-7 10 7 10 7-3.600 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>"""),
    Clock("""<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.500 2"/>"""),
    Alert("""<path d="M12 3 2 20h20zM12 10v4m0 3v.500"/>"""),
    Logout("""<path d="M10 4H5v16h5M14 8l4 4-4 4m4-4H9"/>"""),
    Phone("""<rect x="7" y="2.500" width="10" height="19" rx="2.500"/><path d="M11 18.500h2"/>"""),
    Tablet("""<rect x="3" y="5" width="18" height="14" rx="2.500"/>"""),
    Server("""<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.500h.010M7 16.500h.010"/>"""),
    Link("""<path d="M10 14a4 4 0 0 0 6 0l3-3a4.200 4.200 0 0 0-6-6l-1 1M14 10a4 4 0 0 0-6 0l-3 3a4.200 4.200 0 0 0 6 6l1-1"/>"""),
    Refresh("""<path d="M20 5v5h-5M4 19v-5h5M19 10a7.500 7.500 0 0 0-13-2.500M5 14a7.500 7.500 0 0 0 13 2.500"/>"""),
    Heart("""<path d="M12 20s-7.500-4.600-7.500-10A4.300 4.300 0 0 1 12 7.400 4.300 4.300 0 0 1 19.500 10c0 5.400-7.500 10-7.500 10z"/>"""),
    Share("""<path d="M12 15V4m0 0L8 8m4-4 4 4M5 13v6h14v-6"/>"""),
    Dots("""<circle cx="12" cy="5" r="1.600" fill="currentColor"/><circle cx="12" cy="12" r="1.600" fill="currentColor"/><circle cx="12" cy="19" r="1.600" fill="currentColor"/>"""),
    Hd("""<rect x="3" y="6" width="18" height="12" rx="2"/><path d="M7 9.500v5m0-2.500h3m0-2.500v5M14 9.500v5h1.500a2.500 2.500 0 0 0 0-5z"/>"""),
    Speed("""<path d="M5 17a8 8 0 1 1 14 0M12 13l4-4"/>"""),
    Moon("""<path d="M20 14.500A8.500 8.500 0 0 1 9.500 4 8.500 8.500 0 1 0 20 14.500z"/>"""),
    Key("""<circle cx="8" cy="15" r="4"/><path d="m11 12 9-9m-4 4 3 3"/>"""),
    Globe("""<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3.300 3 14.700 0 18M12 3c-3 3.300-3 14.700 0 18"/>"""),
    Shield("""<path d="M12 3 5 6v6c0 4.500 3 7.700 7 9 4-1.300 7-4.500 7-9V6z"/>"""),
    Storage("""<ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v12c0 1.700 3.600 3 8 3s8-1.300 8-3V6M4 12c0 1.700 3.600 3 8 3s8-1.300 8-3"/>"""),

    /** The crossed eye of the password field (`eyeOff` in setup.js). */
    EyeOff("""<path d="M2 12s3.6-7 10-7c2 0 3.7.7 5.1 1.6M22 12s-3.6 7-10 7c-2 0-3.7-.7-5.1-1.6"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2M3 3l18 18"/>""");

    /** The icon as an ImageVector, built on first use. */
    val vector: ImageVector by lazy { SvgIcon.build(name, svg) }
}

/** Reads the small SVG subset of the mockup's icons: path, rect, circle, ellipse. */
/**
 * An icon of a page that the mockup draws inline (not in `ICONS` of core.js): the same SVG subset,
 * built once. Keep a page's icons in a file of its area (`object PlayerIcons { val Cast by lazy {
 * slouchcatIcon("cast", "<path …/>") } }`), not in [SlouchcatIcon]: parallel branches then never
 * touch the same enum. Draw it with `SlouchcatIconImage(vector, …)`.
 */
fun slouchcatIcon(name: String, svg: String): ImageVector = SvgIcon.build(name, svg)

internal object SvgIcon {
    private val element = Regex("""<(path|rect|circle|ellipse)\b([^>]*)/>""")
    private val attribute = Regex("""([a-zA-Z-]+)="([^"]*)"""")

    fun build(name: String, svg: String): ImageVector {
        val builder = ImageVector.Builder(name, 24.dp, 24.dp, 24f, 24f)
        for (match in element.findAll(svg)) {
            val attrs = attribute.findAll(match.groupValues[2]).associate { it.groupValues[1] to it.groupValues[2] }
            val d = when (match.groupValues[1]) {
                "path" -> attrs.getValue("d")
                "rect" -> rect(attrs)
                "circle" -> ellipse(attrs.f("cx"), attrs.f("cy"), attrs.f("r"), attrs.f("r"))
                else -> ellipse(attrs.f("cx"), attrs.f("cy"), attrs.f("rx"), attrs.f("ry"))
            }
            val filled = attrs["fill"] == "currentColor"
            builder.addPath(
                pathData = addPathNodes(d),
                fill = if (filled) INK else null,
                stroke = INK,
                strokeLineWidth = attrs["stroke-width"]?.toFloat() ?: 2f,
                strokeLineCap = StrokeCap.Round,
                strokeLineJoin = StrokeJoin.Round,
            )
        }
        return builder.build()
    }

    // Any opaque colour: Icon() tints the whole vector with the colour its caller gives.
    private val INK = SolidColor(SlouchcatTokens.Color.Text.Primary)

    private fun Map<String, String>.f(key: String): Float = get(key)?.toFloat() ?: 0f

    private fun rect(a: Map<String, String>): String {
        val x = a.f("x")
        val y = a.f("y")
        val w = a.f("width")
        val h = a.f("height")
        val r = a.f("rx")
        if (r == 0f) return "M$x ${y}h${w}v${h}h${-w}z"
        return "M${x + r} ${y}H${x + w - r}A$r $r 0 0 1 ${x + w} ${y + r}V${y + h - r}" +
            "A$r $r 0 0 1 ${x + w - r} ${y + h}H${x + r}A$r $r 0 0 1 $x ${y + h - r}V${y + r}" +
            "A$r $r 0 0 1 ${x + r} ${y}z"
    }

    private fun ellipse(cx: Float, cy: Float, rx: Float, ry: Float): String =
        "M${cx - rx} ${cy}a$rx $ry 0 1 0 ${2 * rx} 0a$rx $ry 0 1 0 ${-2 * rx} 0z"
}
