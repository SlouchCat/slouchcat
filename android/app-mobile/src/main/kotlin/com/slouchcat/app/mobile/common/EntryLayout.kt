package com.slouchcat.app.mobile.common

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.displayCutoutPadding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.slouchcat.core.design.EntryBackdrop
import com.slouchcat.core.design.SlouchcatDimens
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.isWideLayout

/**
 * The frame of the entry pages «Serveradresse» and «Anmelden» (`.su-entry` in the mockup): the
 * backdrop above the form on a phone, beside it from 700 dp. [pane] gets whether the layout is wide.
 */
@Composable
fun EntryLayout(modifier: Modifier = Modifier, pane: @Composable (wide: Boolean) -> Unit) {
    val wide = isWideLayout()
    Box(modifier.fillMaxSize().background(SlouchcatTheme.colors.bg)) {
        if (wide) {
            Row(Modifier.fillMaxSize()) {
                EntryBackdrop(Modifier.weight(1f).fillMaxHeight(), fadeToEnd = true)
                BoxWithConstraints(Modifier.weight(1f).fillMaxHeight()) {
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .heightIn(min = maxHeight)
                            .verticalScroll(rememberScrollState())
                            .systemBarsPadding().displayCutoutPadding()
                            .imePadding()
                            .padding(start = 24.dp, end = 48.dp, top = 48.dp, bottom = 48.dp),
                        verticalArrangement = Arrangement.Center,
                    ) {
                        Column(Modifier.widthIn(max = SlouchcatDimens.formMax).fillMaxWidth()) { pane(true) }
                    }
                }
            }
        } else {
            Column(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .imePadding(),
            ) {
                Box {
                    EntryBackdrop(Modifier.fillMaxWidth().height(270.dp))
                    // The pane overlaps the faded lower part of the backdrop (`.su-pane`, −110 px).
                    Box(Modifier.padding(start = 24.dp, end = 24.dp, top = 160.dp, bottom = 32.dp).systemBarsPadding().displayCutoutPadding()) {
                        Column(Modifier.widthIn(max = SlouchcatDimens.formMax).fillMaxWidth()) { pane(false) }
                    }
                }
            }
        }
    }
}
