package com.slouchcat.app.mobile.person

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.item.creditCaption
import com.slouchcat.app.mobile.media.MediaCard
import com.slouchcat.app.mobile.media.MediaGrid
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.media.ServerImages
import com.slouchcat.app.mobile.media.defaultCaption
import com.slouchcat.app.mobile.media.fullWidth
import com.slouchcat.core.api.generated.models.Person
import com.slouchcat.core.design.Badge
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.strings.R
import java.time.LocalDate
import java.time.Period
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/**
 * A person (mockup `/person/:id`, reference photos handy 20 and 21): portrait, name, what they
 * are known for, «N Titel in deiner Bibliothek», life dates and biography as far as the server
 * sends them, then «Bekannt aus» with every title the profile sees and the person's roles in it.
 * From 700 dp the portrait stands beside the name.
 */
@Composable
fun PersonScreen(
    state: PersonUiState,
    images: ServerImages,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onToggleBiography: () -> Unit,
    onOpen: (String) -> (() -> Unit)?,
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        when (val loaded = state.person) {
            is Loadable.Ready -> {
                val page = loaded.value
                val cards = remember(page, images) { page.titles.map { MediaItem.from(it.item, images) to it } }
                MediaGrid(
                    cards,
                    key = { it.first.id },
                    header = {
                        fullWidth("head") { Head(page, images, state.biographyOpen, onToggleBiography) }
                        if (cards.isNotEmpty()) {
                            fullWidth("known") {
                                Text(
                                    stringResource(R.string.app_person_knownFrom),
                                    style = SlouchcatType.strong.copy(fontSize = 19.sp, lineHeight = 24.sp),
                                    color = c.tx,
                                    modifier = Modifier.padding(top = 20.dp, bottom = 4.dp).semantics { heading() },
                                )
                            }
                        }
                    },
                ) { (card, title) ->
                    val roles = title.roles.mapNotNull { creditCaption(it.role.value, it.character) }.distinct()
                    MediaCard(card, onClick = onOpen(card.id), caption = roles.takeIf { it.isNotEmpty() }?.joinToString(stringResource(R.string.format_listSeparator)) ?: defaultCaption(card), width = null)
                }
            }
            is Loadable.Failed -> Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(start = gutter(), end = gutter(), top = 88.dp)) {
                if (loaded.error == LoadError.NotFound) {
                    EmptyState(stringResource(R.string.person_notFound_title), body = stringResource(R.string.person_notFound_body), icon = SlouchcatIcon.User)
                } else {
                    LoadableContent(loaded, onRetry = onRetry) {}
                }
            }
            else -> LoadableContent(loaded, onRetry = onRetry, modifier = Modifier.padding(top = 72.dp)) {}
        }
        Box(Modifier.padding(horizontal = 8.dp, vertical = 6.dp).clip(CircleShape).background(c.scrim)) {
            SlouchcatIconButton(SlouchcatIcon.Back, contentDescription = stringResource(R.string.common_back), onClick = onBack)
        }
    }
}

/** The TMDB department (`knownForDepartment`) in the app's language; another one as it comes. */
@Composable
private fun department(value: String?): String? = when (value) {
    null, "" -> null
    "Acting" -> stringResource(R.string.person_department_acting)
    "Directing" -> stringResource(R.string.person_department_directing)
    "Writing" -> stringResource(R.string.person_department_writing)
    "Production" -> stringResource(R.string.person_department_production)
    "Sound" -> stringResource(R.string.person_department_sound)
    "Camera" -> stringResource(R.string.person_department_camera)
    "Editing" -> stringResource(R.string.person_department_editing)
    "Art" -> stringResource(R.string.person_department_art)
    "Crew" -> stringResource(R.string.person_department_crew)
    "Costume & Make-Up" -> stringResource(R.string.person_department_costume)
    "Visual Effects" -> stringResource(R.string.person_department_visualEffects)
    "Lighting" -> stringResource(R.string.person_department_lighting)
    "Creator" -> stringResource(R.string.person_department_creator)
    else -> value
}

/** «Geboren 4. Juni 1996 (30)», «Gestorben …», «Geburtsort …» (web `personFacts`). */
@Composable
private fun facts(person: Person): List<Pair<String, String>> {
    val locale = LocalConfiguration.current.locales[0]
    val format = remember(locale) { DateTimeFormatter.ofLocalizedDate(FormatStyle.LONG).withLocale(locale) }
    val today = remember { LocalDate.now() }
    val born = person.birthday
    val died = person.deathday
    return buildList {
        if (born != null) {
            val date = born.format(format)
            add(stringResource(R.string.person_facts_born) to if (died != null) date else stringResource(R.string.person_facts_bornAge, date, Period.between(born, today).years.toString()))
        }
        person.placeOfBirth?.takeIf { it.isNotBlank() }?.let { add(stringResource(R.string.person_facts_birthplace) to it) }
        if (died != null) {
            val date = died.format(format)
            add(stringResource(R.string.person_facts_died) to if (born != null) stringResource(R.string.person_facts_diedAge, date, Period.between(born, died).years.toString()) else date)
        }
    }
}

@Composable
private fun Head(page: PersonPage, images: ServerImages, biographyOpen: Boolean, onToggleBiography: () -> Unit) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    val person = page.person
    val portrait = remember(person.image, images) { images.single(person.image) }
    val glow = Brush.radialGradient(listOf(c.s2, Color.Transparent))
    val text: @Composable (Alignment.Horizontal) -> Unit = { align ->
        Column(horizontalAlignment = align, verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(
                person.name,
                style = SlouchcatType.pageTitle.copy(fontSize = if (wide) 40.sp else 28.sp, lineHeight = if (wide) 46.sp else 32.sp, fontWeight = FontWeight.ExtraBold),
                color = c.tx,
                textAlign = if (wide) TextAlign.Start else TextAlign.Center,
                modifier = Modifier.semantics { heading() },
            )
            department(person.knownForDepartment)?.let { Text(it, style = SlouchcatType.bodySmall.copy(fontSize = 15.sp), color = c.tx2) }
            Badge(pluralStringResource(R.plurals.app_person_inLibrary, page.titles.size, page.titles.size))
        }
    }
    Column(Modifier.fillMaxWidth().background(glow).padding(top = if (wide) 72.dp else 64.dp)) {
        if (wide) {
            Row(horizontalArrangement = Arrangement.spacedBy(32.dp), verticalAlignment = Alignment.Bottom) {
                Portrait(portrait, person.name, Modifier.width(200.dp))
                Box(Modifier.padding(bottom = 8.dp)) { text(Alignment.Start) }
            }
        } else {
            Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Portrait(portrait, person.name, Modifier.width(148.dp))
                text(Alignment.CenterHorizontally)
            }
        }
        val facts = facts(person)
        if (facts.isNotEmpty()) {
            Column(Modifier.padding(top = 16.dp).widthIn(max = 720.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                facts.forEach { (k, v) ->
                    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        Text(k, style = SlouchcatType.bodySmall, color = c.tx3, modifier = Modifier.width(120.dp))
                        Text(v, style = SlouchcatType.bodySmall, color = c.tx2, modifier = Modifier.weight(1f))
                    }
                }
            }
        }
        person.biography?.takeIf { it.isNotBlank() }?.let { Biography(it, person.biographyFallback == true, biographyOpen, onToggleBiography) }
    }
}

@Composable
private fun Portrait(images: com.slouchcat.app.mobile.media.ImageSet, name: String, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(18.dp)
    Box(modifier.aspectRatio(2f / 3f).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
        MediaImage(images, name, Modifier.fillMaxSize())
    }
}

/** The biography in three lines with «Mehr lesen» when it is longer (web `Biography`). */
@Composable
private fun Biography(text: String, fallback: Boolean, open: Boolean, onToggle: () -> Unit) {
    val c = SlouchcatTheme.colors
    var clamped by remember(text) { mutableStateOf(false) }
    Column(Modifier.padding(top = 14.dp).widthIn(max = 720.dp)) {
        Text(
            text,
            style = SlouchcatType.body.copy(fontSize = 15.5.sp, lineHeight = 24.sp),
            color = c.tx2,
            maxLines = if (open) Int.MAX_VALUE else 3,
            overflow = TextOverflow.Ellipsis,
            onTextLayout = { if (!open) clamped = it.hasVisualOverflow },
        )
        if (clamped || open) {
            Box(Modifier.clip(RoundedCornerShape(8.dp)).clickable(role = Role.Button, onClick = onToggle).heightIn(min = 48.dp).padding(end = 8.dp), contentAlignment = Alignment.CenterStart) {
                Text(
                    stringResource(if (open) R.string.person_biography_less else R.string.person_biography_more),
                    style = SlouchcatType.bodySmall.copy(fontSize = 14.5.sp, textDecoration = TextDecoration.Underline),
                    color = c.tx,
                )
            }
        }
        if (fallback) Text(stringResource(R.string.person_biography_fallback), style = SlouchcatType.caption, color = c.tx3)
    }
}
