package com.slouchcat.app.tv.shell

import com.slouchcat.app.tv.audio.AudioFeature
import com.slouchcat.app.tv.home.HomeFeature
import com.slouchcat.app.tv.item.ItemFeature
import com.slouchcat.app.tv.library.LibraryFeature
import com.slouchcat.app.tv.lists.ListsFeature
import com.slouchcat.app.tv.person.PersonFeature
import com.slouchcat.app.tv.playback.PlaybackSettingsFeature
import com.slouchcat.app.tv.player.PlayerFeature
import com.slouchcat.app.tv.profilesettings.ProfileSettingsFeature
import com.slouchcat.app.tv.search.SearchFeature
import com.slouchcat.app.tv.settings.SettingsFeature

/**
 * Every feature of the TV app, one line each (concept 26, section 10: one or more per stage). The
 * order here does not matter: the rail sorts by section and order, the settings by order. A stage
 * adds its feature here with one line `XFeature,` and nowhere else in the shell. Git merges this
 * file line by line (`merge=union` in .gitattributes): keep one feature per line, each with its
 * trailing comma, and its import on a line of its own.
 */
val features: List<TvFeature> = listOf(
    SettingsFeature,
    HomeFeature,
    LibraryFeature,
    ItemFeature(),
    PersonFeature,
    SearchFeature,
    ListsFeature,
    ProfileSettingsFeature,
    PlayerFeature(),
    PlaybackSettingsFeature(),
    AudioFeature(),
)
