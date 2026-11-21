package com.slouchcat.app.mobile.common

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.slouchcat.core.design.SlouchcatDimens
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType

/**
 * `.su-group`: rows in one framed card, divided by lines (mockup `group(rows)`). Each row is a
 * `ListRow` or a [StaticRow]; the dividers come from here.
 */
@Composable
fun RowGroup(modifier: Modifier = Modifier, rows: List<@Composable () -> Unit>) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    Column(
        modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape),
    ) {
        rows.forEachIndexed { index, row ->
            if (index > 0) HorizontalDivider(color = c.line)
            row()
        }
    }
}

/** `.li.su-static`: a row that only tells (no tap), with the icon on its small surface. */
@Composable
fun StaticRow(title: String, icon: SlouchcatIcon, modifier: Modifier = Modifier, subtitle: String? = null, iconTint: Color? = null) {
    val c = SlouchcatTheme.colors
    Row(
        modifier
            .fillMaxWidth()
            .defaultMinSize(minHeight = SlouchcatDimens.rowHeight)
            .semantics(mergeDescendants = true) {}
            .padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.size(40.dp).clip(RoundedCornerShape(12.dp)).background(c.s2), contentAlignment = Alignment.Center) {
            SlouchcatIconImage(icon, contentDescription = null, size = 22.dp, tint = iconTint ?: c.tx2)
        }
        Column(Modifier.weight(1f)) {
            Text(title, style = SlouchcatType.rowTitle, color = c.tx)
            if (subtitle != null) Text(subtitle, style = SlouchcatType.caption, color = c.tx3)
        }
    }
}

/** `.su-dl` in a sheet: name left, value right, a line above each row. */
@Composable
fun ColumnScope.DetailRows(rows: List<Pair<String, String>>, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Column(modifier.fillMaxWidth()) {
        for ((name, value) in rows) {
            HorizontalDivider(color = c.line)
            Row(
                Modifier.fillMaxWidth().semantics(mergeDescendants = true) {}.padding(vertical = 11.dp),
                horizontalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                Text(name, style = SlouchcatType.bodySmall, color = c.tx2)
                Text(
                    value,
                    style = SlouchcatType.bodySmall.copy(fontWeight = SlouchcatType.strong.fontWeight),
                    color = c.tx,
                    textAlign = TextAlign.End,
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}
