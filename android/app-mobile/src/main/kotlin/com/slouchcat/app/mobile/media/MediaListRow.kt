package com.slouchcat.app.mobile.media

import androidx.compose.foundation.BorderStroke
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.strings.R

/** The line under a title in a list (mockup `lrow`): «2004 · Film · 2 h 5 min», «Autorin · Hörbuch». */
@Composable
fun listCaption(item: MediaItem): String {
    val parts = listOfNotNull(
        if (item.isAudio) item.author?.takeIf { it.isNotBlank() } else item.year?.toString(),
        kindName(item.kind),
        item.runtimeMinutes?.takeIf { it > 0 && item.kind != ItemKind.PODCAST }?.let { runtimeText(it) },
    )
    return parts.joinToString(stringResource(R.string.format_separator))
}

/**
 * A title as a row of a list (mockup `lrow(i)`, `.lr`): small cover (2:3, square for audio, 16:9
 * for an episode), title in up to two lines, [caption] (default [listCaption]), the check mark of a
 * finished title and the arrow. [onClick] `null` (no page yet): no control. One control for the
 * screen reader.
 */
@Composable
fun MediaListRow(
    item: MediaItem,
    onClick: (() -> Unit)?,
    modifier: Modifier = Modifier,
    caption: String = listCaption(item),
) {
    val c = SlouchcatTheme.colors
    val watched = if (item.watched) stringResource(if (item.isAudio) R.string.library_status_heard else R.string.library_status_watched) else null
    val said = listOfNotNull(item.title, caption.takeIf { it.isNotEmpty() }, watched).joinToString(stringResource(R.string.format_listSeparator))
    Row(
        modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClick = onClick) else Modifier)
            .clearAndSetSemantics { contentDescription = said }
            .heightIn(min = 72.dp)
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        val (width, ratio) = when {
            item.kind == ItemKind.EPISODE -> 96.dp to 16f / 9f
            item.isAudio -> 64.dp to 1f
            else -> 52.dp to 2f / 3f
        }
        Box(
            Modifier
                .width(width)
                .aspectRatio(ratio)
                .clip(RoundedCornerShape(8.dp))
                .border(BorderStroke(1.dp, c.line), RoundedCornerShape(8.dp)),
        ) {
            MediaImage(item.poster, item.title, Modifier.fillMaxSize())
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(item.title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
            if (caption.isNotEmpty()) Text(caption, style = SlouchcatType.caption, color = c.tx3, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        if (item.watched) SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 16.dp, tint = c.tx3)
        if (onClick != null) SlouchcatIconImage(SlouchcatIcon.Right, contentDescription = null, size = 18.dp, tint = c.tx3)
    }
}
